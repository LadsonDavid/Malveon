package extractor

import (
	"regexp"
	"strings"
)

// The identifier scanner every per-file pass shares. It returns exactly what
// regexp `[A-Za-z_$][\w$]*` finds (Go's \w is ASCII only), at a fraction of
// the cost: the regular expression was the single biggest line of a profile
// of a 6,000-file project.
func isWordStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isWordPart(c byte) bool { return isWordStart(c) || (c >= '0' && c <= '9') }

// nextWord returns the next identifier at or after from, or -1, -1.
func nextWord(src string, from int) (start, end int) {
	i := from
	for i < len(src) && !isWordStart(src[i]) {
		i++
	}
	if i >= len(src) {
		return -1, -1
	}
	j := i + 1
	for j < len(src) && isWordPart(src[j]) {
		j++
	}
	return i, j
}

// wordRegexp is the pattern nextWord replaces; kept for the test that proves they agree.
var wordRegexp = regexp.MustCompile(`[A-Za-z_$][\w$]*`)

// containsAny reports whether src holds at least one of the literals.
func containsAny(src string, lits ...string) bool {
	for _, l := range lits {
		if strings.Contains(src, l) {
			return true
		}
	}
	return false
}
