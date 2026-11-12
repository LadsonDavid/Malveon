package extractor

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// cacheFormat changes when the saved shape changes (the graph's fields, this file's header).
// The caller's stamp covers everything else: which build of the extractor produced the map.
const cacheFormat = 1

// racyWindow: a file changed this recently when the map was read could change again without
// its size or time moving (file times are coarse). Such a map is not saved.
const racyWindow = 3 * time.Second

// maxCacheFiles bounds the folder: the newest maps are kept, older ones removed.
const maxCacheFiles = 16

// maxCacheBytes: a map larger than this is not saved (it would cost more to keep than to rebuild).
const maxCacheBytes = 400 << 20

type cacheHeader struct {
	Format      int
	Stamp       string
	Root        string
	Fingerprint string
}

// ExtractCached is Extract with a saved copy of the result. If nothing the extractor reads has
// changed since the copy was made (same files, same sizes and modification times, same build of
// the program) it returns the saved map without reading any file; otherwise it reads the project
// as Extract does and saves the new map. cacheDir is where the copy lives, and stamp names the
// build that is asking (a different build never trusts another's copy). The bool reports whether
// the saved copy was used. A copy that cannot be read or written is ignored, never an error.
func ExtractCached(root, cacheDir, stamp string) (*graph.Graph, bool, error) {
	if cacheDir == "" {
		g, err := Extract(root)
		return g, false, err
	}
	entries, err := walkFiles(root, true)
	if err != nil {
		return nil, false, err
	}
	abs, _ := filepath.Abs(root)
	fp, racy := fingerprint(entries, time.Now())
	path := filepath.Join(cacheDir, cacheName(abs))
	want := cacheHeader{Format: cacheFormat, Stamp: stamp, Root: abs, Fingerprint: fp}
	if g, ok := loadCache(path, want); ok {
		touch(path)
		return g, true, nil
	}
	g, err := Extract(root)
	if err != nil {
		return nil, false, err
	}
	if !racy {
		saveCache(cacheDir, path, want, g)
	}
	return g, false, nil
}

func cacheName(absRoot string) string {
	h := sha256.Sum256([]byte(strings.ToLower(filepath.ToSlash(absRoot))))
	return "codemap-" + hex.EncodeToString(h[:12]) + ".json"
}

// readsContent: the extractor reads this file's text, so a change to it can change the map.
// Any other file matters only by existing (it is listed in Files).
func readsContent(e fileEntry) bool {
	base := strings.ToLower(filepath.Base(e.path))
	switch {
	case base == "package.json", e.ext == ".sql", e.ext == ".prisma", e.ext == ".md", e.ext == ".mdx":
		return true
	}
	return tplExts[e.ext] || templateExts[e.ext] || scannedExts[e.ext]
}

// fingerprint summarises everything the extractor depends on: the list of files, and for the
// ones it reads, their size and modification time. racy reports a read file changed within
// racyWindow of now.
func fingerprint(entries []fileEntry, now time.Time) (string, bool) {
	h := sha256.New()
	racy := false
	for _, e := range entries {
		fmt.Fprintf(h, "%s\x00", e.rel)
		if !readsContent(e) {
			io.WriteString(h, "\n")
			continue
		}
		fi, err := os.Stat(e.path)
		if err != nil {
			io.WriteString(h, "?\n")
			racy = true
			continue
		}
		mt := fi.ModTime()
		if now.Sub(mt) < racyWindow {
			racy = true
		}
		fmt.Fprintf(h, "%d\x00%d\n", fi.Size(), mt.UnixNano())
	}
	return hex.EncodeToString(h.Sum(nil)), racy
}

func loadCache(path string, want cacheHeader) (*graph.Graph, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	dec := json.NewDecoder(bufio.NewReaderSize(f, 1<<20))
	var got cacheHeader
	if dec.Decode(&got) != nil || got != want {
		return nil, false
	}
	g := &graph.Graph{}
	if dec.Decode(g) != nil {
		return nil, false
	}
	return g, true
}

func saveCache(dir, path string, h cacheHeader, g *graph.Graph) {
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, "codemap-*.tmp")
	if err != nil {
		return
	}
	name := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(name)
		}
	}()
	w := bufio.NewWriterSize(tmp, 1<<20)
	enc := json.NewEncoder(w)
	if enc.Encode(h) != nil || enc.Encode(g) != nil || w.Flush() != nil {
		return
	}
	if fi, err := tmp.Stat(); err != nil || fi.Size() > maxCacheBytes {
		return
	}
	if tmp.Close() != nil {
		return
	}
	if os.Rename(name, path) != nil {
		return
	}
	ok = true
	prune(dir)
}

func touch(path string) {
	now := time.Now()
	os.Chtimes(path, now, now)
}

// prune keeps the newest maxCacheFiles maps.
func prune(dir string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type f struct {
		name string
		mt   time.Time
	}
	var fs []f
	for _, e := range ents {
		if !strings.HasPrefix(e.Name(), "codemap-") {
			continue
		}
		if info, err := e.Info(); err == nil {
			fs = append(fs, f{e.Name(), info.ModTime()})
		}
	}
	if len(fs) <= maxCacheFiles {
		return
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].mt.After(fs[j].mt) })
	for _, old := range fs[maxCacheFiles:] {
		os.Remove(filepath.Join(dir, old.name))
	}
}
