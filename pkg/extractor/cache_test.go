package extractor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// age moves every file's modification time into the past, so none counts as just changed.
var longAgo = time.Now().Add(-time.Hour) // one fixed moment, so aging twice changes nothing

func age(t *testing.T, root string) {
	t.Helper()
	old := longAgo
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			os.Chtimes(p, old, old)
		}
		return nil
	})
}

// sameGraph: the saved copy and a fresh read agree on every exported field, including which
// collections are nil and which are merely empty (code tests for both).
func sameGraph(t *testing.T, label string, a, b *graph.Graph) {
	t.Helper()
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatalf("%s: the maps differ (%d vs %d bytes of JSON)", label, len(ja), len(jb))
	}
	va, vb := reflect.ValueOf(a).Elem(), reflect.ValueOf(b).Elem()
	for i := 0; i < va.NumField(); i++ {
		f := va.Type().Field(i)
		if !f.IsExported() {
			continue
		}
		switch f.Type.Kind() {
		case reflect.Map, reflect.Slice:
			if va.Field(i).IsNil() != vb.Field(i).IsNil() {
				t.Fatalf("%s: %s is nil in one and empty in the other", label, f.Name)
			}
		}
	}
}

func TestCachedMapEqualsAFreshOneAndIsReused(t *testing.T) {
	for _, root := range []string{"../../testdata/fixture", "../../testdata/fixture-go", "../../testdata/fixture-python", "../../testdata/fixture-java", bigTree(t)} {
		dir := t.TempDir()
		if !filepath.IsAbs(root) {
			root = copyTree(t, root) // fixtures live in the repository: copy so their times can be set
		}
		age(t, root)
		cold, err := Extract(root)
		if err != nil {
			t.Fatal(err)
		}
		g1, hit1, err := ExtractCached(root, dir, "build-1")
		if err != nil || hit1 {
			t.Fatalf("%s: first call hit=%v err=%v", root, hit1, err)
		}
		g2, hit2, err := ExtractCached(root, dir, "build-1")
		if err != nil || !hit2 {
			t.Fatalf("%s: second call should use the saved map: hit=%v err=%v", root, hit2, err)
		}
		sameGraph(t, root+" first", cold, g1)
		sameGraph(t, root+" saved", cold, g2)
	}
}

func copyTree(t *testing.T, src string) string {
	dst := t.TempDir()
	filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			os.MkdirAll(filepath.Join(dst, rel), 0o755)
			return nil
		}
		b, _ := os.ReadFile(p)
		os.WriteFile(filepath.Join(dst, rel), b, 0o644)
		return nil
	})
	return dst
}

func TestCachedMapIsRebuiltWhenAnythingItReadsChanges(t *testing.T) {
	root := bigTree(t)
	age(t, root)
	dir := t.TempDir()
	if _, hit, _ := ExtractCached(root, dir, "b"); hit {
		t.Fatal("nothing saved yet")
	}
	check := func(label string, wantHit bool) {
		t.Helper()
		g, hit, err := ExtractCached(root, dir, "b")
		if err != nil || hit != wantHit {
			t.Fatalf("%s: hit=%v want %v err=%v", label, hit, wantHit, err)
		}
		cold, _ := Extract(root)
		sameGraph(t, label, cold, g)
		age(t, root)
		// ExtractCached saved a new copy on a miss; age made the files old again, so the next call can reuse it
	}
	check("nothing changed", true)

	// a file the extractor reads, edited (new size, new time)
	os.WriteFile(filepath.Join(root, "server/routes/r3.js"), []byte("const express = require('express')\nconst router = express.Router()\nrouter.delete('/api/gone', (req, res) => res.json({}))\nmodule.exports = router\n"), 0o644)
	age(t, root)
	check("a read file was edited", false)
	check("and again unchanged", true)

	// a new file
	os.WriteFile(filepath.Join(root, "web/new.js"), []byte("export const a = 1\n"), 0o644)
	age(t, root)
	check("a file was added", false)

	// a deleted file
	os.Remove(filepath.Join(root, "web/new.js"))
	check("a file was removed", false)

	// same size, different content, only the time moved
	p := filepath.Join(root, "web/h5.js")
	b, _ := os.ReadFile(p)
	b[len(b)-3] = 'Z'
	os.WriteFile(p, b, 0o644)
	later := time.Now().Add(-30 * time.Minute)
	os.Chtimes(p, later, later)
	check("same size, newer time", false)

	// a file the extractor only lists: its time is not read, but its name is
	os.WriteFile(filepath.Join(root, "logo.png"), []byte("x"), 0o644)
	age(t, root)
	check("an image was added", false)
	later = time.Now().Add(-10 * time.Minute)
	os.Chtimes(filepath.Join(root, "logo.png"), later, later)
	check("an image was touched", true)

	// a rename keeps a file's size and time, but not its name
	os.Rename(filepath.Join(root, "web/h7.js"), filepath.Join(root, "web/h7-renamed.js"))
	check("a read file was renamed", false)
	os.Rename(filepath.Join(root, "logo.png"), filepath.Join(root, "logo2.png"))
	check("an image was renamed", false)
}

func TestCachedMapIsNotTrustedAcrossBuildsOrWhenDamaged(t *testing.T) {
	root := bigTree(t)
	age(t, root)
	dir := t.TempDir()
	ExtractCached(root, dir, "build-1")
	if _, hit, _ := ExtractCached(root, dir, "build-2"); hit {
		t.Fatal("another build must not trust this copy")
	}
	ExtractCached(root, dir, "build-1")
	files, _ := filepath.Glob(filepath.Join(dir, "codemap-*.json"))
	if len(files) != 1 {
		t.Fatalf("expected one saved map, got %v", files)
	}
	for name, damage := range map[string]func(string){
		"truncated": func(p string) { b, _ := os.ReadFile(p); os.WriteFile(p, b[:len(b)/2], 0o644) },
		"garbage":   func(p string) { os.WriteFile(p, []byte("not json"), 0o644) },
		"empty":     func(p string) { os.WriteFile(p, nil, 0o644) },
	} {
		ExtractCached(root, dir, "build-1")
		damage(files[0])
		g, hit, err := ExtractCached(root, dir, "build-1")
		if err != nil || hit || g == nil || len(g.Files) == 0 {
			t.Fatalf("%s: hit=%v err=%v", name, hit, err)
		}
		if _, hit, _ := ExtractCached(root, dir, "build-1"); !hit {
			t.Fatalf("%s: the damaged copy should have been replaced", name)
		}
	}
}

func TestAJustChangedFileIsNeverSavedAndAnEmptyFolderMeansNoCache(t *testing.T) {
	root := bigTree(t) // written just now: every file is "racy"
	dir := t.TempDir()
	if _, hit, _ := ExtractCached(root, dir, "b"); hit {
		t.Fatal("first call")
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "codemap-*.json")); len(files) != 0 {
		t.Fatalf("a map read while files were still changing must not be saved: %v", files)
	}
	g, hit, err := ExtractCached(root, "", "b")
	if err != nil || hit || len(g.Files) == 0 {
		t.Fatalf("no folder: hit=%v err=%v", hit, err)
	}
}

func TestOnlyTheNewestMapsAreKept(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		p := filepath.Join(dir, fmt.Sprintf("codemap-old%02d.json", i))
		os.WriteFile(p, []byte("{}"), 0o644)
		mt := time.Now().Add(time.Duration(-100+i) * time.Minute)
		os.Chtimes(p, mt, mt)
	}
	root := bigTree(t)
	age(t, root)
	ExtractCached(root, dir, "b")
	files, _ := filepath.Glob(filepath.Join(dir, "codemap-*.json"))
	if len(files) != maxCacheFiles {
		t.Fatalf("kept %d maps, want %d", len(files), maxCacheFiles)
	}
	if _, err := os.Stat(filepath.Join(dir, cacheName(mustAbs(root)))); err != nil {
		t.Fatal("the map just saved must be among the newest")
	}
}

func mustAbs(p string) string { a, _ := filepath.Abs(p); return a }

// If the graph grows a field the saved copy cannot hold (an unexported one, an interface, a
// function), the cache would silently lose it. This fails first, so the format gets decided.
func TestGraphHoldsNothingTheSavedCopyCannotKeep(t *testing.T) {
	lazy := map[string]bool{"commonWords": true, "mentioned": true, "importers": true, "triggersIn": true}
	seen := map[reflect.Type]bool{}
	var walk func(rt reflect.Type, path string)
	walk = func(rt reflect.Type, path string) {
		for rt.Kind() == reflect.Ptr || rt.Kind() == reflect.Slice || rt.Kind() == reflect.Array || rt.Kind() == reflect.Map {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct || seen[rt] {
			return
		}
		seen[rt] = true
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			switch {
			case !f.IsExported() && rt == reflect.TypeOf(graph.Graph{}) && lazy[f.Name]:
				// recomputed on first use
			case !f.IsExported():
				t.Errorf("%s.%s is unexported: the saved copy would lose it", path, f.Name)
			case f.Type.Kind() == reflect.Interface || f.Type.Kind() == reflect.Func || f.Type.Kind() == reflect.Chan:
				t.Errorf("%s.%s cannot be saved", path, f.Name)
			default:
				walk(f.Type, path+"."+f.Name)
			}
		}
	}
	walk(reflect.TypeOf(graph.Graph{}), "Graph")
}
