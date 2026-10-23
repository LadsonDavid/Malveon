package extractor

import (
	"sort"
	"strings"
	"sync"
	"unsafe"
)

// lineIndex answers "which line is this byte on" in O(log lines) instead of
// counting newlines from the start of the file for every match (quadratic on a
// large file). The scanners ask about one file at a time, so one remembered
// index is enough; it is rebuilt when a different string is asked about.
type lineIndex struct {
	mu     sync.Mutex
	ptr    *byte
	n      int
	starts []int // byte offset where each line starts
}

var lines lineIndex

func (li *lineIndex) lineOf(src string, off int) int {
	if off > len(src) {
		off = len(src)
	}
	li.mu.Lock()
	defer li.mu.Unlock()
	if p := unsafe.StringData(src); li.ptr != p || li.n != len(src) {
		li.ptr, li.n = p, len(src)
		li.starts = li.starts[:0]
		li.starts = append(li.starts, 0)
		for i := 0; ; {
			j := strings.IndexByte(src[i:], '\n')
			if j < 0 {
				break
			}
			i += j + 1
			li.starts = append(li.starts, i)
		}
	}
	// the number of line starts at or before off is the 1-based line number
	return sort.Search(len(li.starts), func(i int) bool { return li.starts[i] > off })
}
