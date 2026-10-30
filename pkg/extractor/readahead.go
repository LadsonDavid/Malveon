package extractor

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/LadsonDavid/Malveon/pkg/skipdirs"
)

// readAheadWindow is how many files may be read ahead of the one being analysed. Opening a file
// is the slow part on Windows (every open is scanned by antivirus: 35k files took 48s of
// 58s), so reads overlap while the analysis stays in order. The window bounds memory.
const readAheadWindow = 64

// newReadAhead returns a replacement for os.ReadFile for the files collectJSInfo reads. Files
// must be requested in the order the walk visits them; any other path falls back to a plain read.
func newReadAhead(root string) func(string) ([]byte, error) {
	type result struct {
		b   []byte
		err error
	}
	var paths []string
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && skipdirs.Names[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		// Exactly the files collectJSInfo reads: one never taken would hold a window slot forever.
		ext := strings.ToLower(filepath.Ext(path))
		switch {
		case strings.EqualFold(filepath.Base(path), "package.json"), ext == ".prisma", templateExts[ext]:
		case ext == ".js", ext == ".jsx", ext == ".ts", ext == ".tsx", ext == ".mjs", ext == ".cjs":
		default:
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	chans := make(map[string]chan result, len(paths))
	for _, p := range paths {
		chans[p] = make(chan result, 1)
	}
	sem := make(chan struct{}, readAheadWindow)
	var once sync.Once
	start := func() {
		go func() {
			for _, p := range paths {
				sem <- struct{}{}
				go func(p string) {
					b, err := os.ReadFile(p)
					chans[p] <- result{b, err}
				}(p)
			}
		}()
	}
	return func(path string) ([]byte, error) {
		ch, ok := chans[path]
		if !ok {
			return os.ReadFile(path)
		}
		once.Do(start)
		r := <-ch
		<-sem
		return r.b, r.err
	}
}
