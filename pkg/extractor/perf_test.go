package extractor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A large single-file bundle with tens of thousands of .get( / .use( / fetch(
// calls must scan in time proportional to its size. Found by audit,
// 2026-10-10: a 7.6 MB bundle took ~45 s; two quadratic loops (a line count
// from the start of the file per match, and a fresh set of regexes over the
// whole file per app.use(name)) made it minutes. 1 MB took over 400 s then.
func TestLargeBundleIsNotQuadratic(t *testing.T) {
	root := t.TempDir()
	chunk := "var a=b.get(c),d=e.post('/x'+f);function g(h){return h.map(function(i){return i.use(j)})};const k=fetch('/api/'+m);"
	src := strings.Repeat(chunk, 1_500_000/len(chunk))
	if err := os.WriteFile(filepath.Join(root, "bundle.js"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := Extract(root); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 30*time.Second {
		t.Fatalf("Extract of a %.1f MB bundle took %v — something is quadratic again", float64(len(src))/1e6, took)
	}
}
