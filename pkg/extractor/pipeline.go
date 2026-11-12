package extractor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/skipdirs"
)

// fileEntry is one file the walk visited, in walk order.
type fileEntry struct {
	path string // as walked (absolute or root-relative to the working directory)
	rel  string // relative to the root, in the OS form
	ext  string // lower-case extension
}

// walkFiles lists every file under root that is not in a skipped folder, in the order
// filepath.WalkDir visits them. With stopOnError a walk error ends the walk (what Extract
// always did); without it an unreadable folder is skipped (what collectJSInfo always did).
func walkFiles(root string, stopOnError bool) ([]fileEntry, error) {
	var out []fileEntry
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if stopOnError {
				return err
			}
			return nil
		}
		if d.IsDir() {
			if path != root && skipdirs.Names[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		out = append(out, fileEntry{path: path, rel: rel, ext: strings.ToLower(filepath.Ext(path))})
		return nil
	})
	return out, err
}

// parallelOrdered runs work(i) for every i in [0, n) on several goroutines and gives each
// result to emit in index order, on the calling goroutine. Only a bounded number of results
// wait at once, so the memory held for files not yet merged stays small. Because emit sees
// the results in the same order a plain loop would, everything built from them is identical
// to the sequential run's.
func parallelOrdered[T any](n int, work func(i int) T, emit func(i int, r T)) {
	workers := runtime.GOMAXPROCS(0)
	if workers < 2 || n < 2 {
		for i := 0; i < n; i++ {
			emit(i, work(i))
		}
		return
	}
	order := make(chan chan T, workers*3)
	go func() {
		for i := 0; i < n; i++ {
			ch := make(chan T, 1)
			go func(i int) { ch <- work(i) }(i)
			order <- ch
		}
		close(order)
	}()
	i := 0
	for ch := range order {
		emit(i, <-ch)
		i++
	}
}
