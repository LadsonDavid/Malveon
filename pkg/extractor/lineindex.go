package extractor

import (
	"sort"
	"strings"
	"sync"
	"unsafe"
)

// fileFacts holds what several passes over the same file ask for again and
// again: where its lines start, where its functions are, and where its names
// are defined. Each is computed once, the first time anyone asks, whichever
// worker that is. The cache is keyed by the string's memory (not its text), the
// same way the single remembered line index was, and is bounded by bytes, so a
// 7 MB bundle costs one entry and never an index rebuilt per question.
type fileFacts struct {
	src string

	lineOnce sync.Once
	starts   []int // byte offset where each line starts

	rangesOnce sync.Once
	ranges     []funcRange

	defsOnce sync.Once
	defs     [][]int // defPatterns["js"] matches, with submatch positions
}

type factsKey struct {
	ptr *byte
	n   int
}

var factsCache struct {
	sync.Mutex
	byKey map[factsKey]*fileFacts
	order []factsKey
	bytes int
}

const factsBudget = 96 << 20

// factsFor returns the shared facts of src.
func factsFor(src string) *fileFacts {
	k := factsKey{unsafe.StringData(src), len(src)}
	factsCache.Lock()
	defer factsCache.Unlock()
	if f, ok := factsCache.byKey[k]; ok {
		return f
	}
	if factsCache.byKey == nil {
		factsCache.byKey = map[factsKey]*fileFacts{}
	}
	f := &fileFacts{src: src}
	factsCache.byKey[k] = f
	factsCache.order = append(factsCache.order, k)
	factsCache.bytes += len(src)
	for factsCache.bytes > factsBudget && len(factsCache.order) > 1 {
		old := factsCache.order[0]
		factsCache.order = factsCache.order[1:]
		factsCache.bytes -= old.n
		delete(factsCache.byKey, old)
	}
	return f
}

func (f *fileFacts) lineOf(off int) int {
	f.lineOnce.Do(func() {
		f.starts = append(f.starts, 0)
		for i := 0; ; {
			j := strings.IndexByte(f.src[i:], '\n')
			if j < 0 {
				break
			}
			i += j + 1
			f.starts = append(f.starts, i)
		}
	})
	if off > len(f.src) {
		off = len(f.src)
	}
	// the number of line starts at or before off is the 1-based line number
	return sort.Search(len(f.starts), func(i int) bool { return f.starts[i] > off })
}

// funcRanges is a copy: callers may reorder or change what they get.
func (f *fileFacts) funcRanges() []funcRange {
	f.rangesOnce.Do(func() { f.ranges = computeJSFuncRanges(f.src) })
	return append([]funcRange(nil), f.ranges...)
}

// defMatches are the matches of the JavaScript definition pattern; read-only.
func (f *fileFacts) defMatches() [][]int {
	f.defsOnce.Do(func() { f.defs = defPatterns["js"].FindAllStringSubmatchIndex(f.src, -1) })
	return f.defs
}

type lineIndex struct{}

var lines lineIndex

func (lineIndex) lineOf(src string, off int) int { return factsFor(src).lineOf(off) }
