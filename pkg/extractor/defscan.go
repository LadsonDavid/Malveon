package extractor

// scanJSDefs finds exactly what defPatterns["js"] finds, without the regular expression:
//
//	(?m)(?:^|[\s;])(?:export\s+)?(?:default\s+)?(?:async\s+)?(?:function\*?|class|const|let|var|interface|type|enum)\s+([A-Za-z_$][\w$]*)
//
// Each element is [start, end, nameStart, nameEnd], the same positions FindAllStringSubmatchIndex
// gave. The pattern has no real ambiguity (every optional part is a distinct keyword, every
// whitespace run is greedy and must be followed by the next token), so one left-to-right pass
// does it; the regular expression stays in the tests as the judge. It was about 17% of the time
// spent reading a large project.
func scanJSDefs(src string) [][]int {
	var out [][]int
	n := len(src)
	for p := 0; p < n; {
		var m []int
		// "^" in multi-line mode: the start of the text or just after a newline. Tried first.
		if p == 0 || src[p-1] == '\n' {
			m = defTail(src, p, p)
		}
		if m == nil && (isDefSpace(src[p]) || src[p] == ';') { // the pattern's leading [\s;]
			m = defTail(src, p, p+1)
		}
		if m == nil {
			p++
			continue
		}
		out = append(out, m)
		p = m[1] // the next search starts where this match ended
	}
	return out
}

func isDefSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r' }

// skipSpace returns the index after a run of at least one whitespace byte at i, or -1.
func skipSpace(src string, i int) int {
	j := i
	for j < len(src) && isDefSpace(src[j]) {
		j++
	}
	if j == i {
		return -1
	}
	return j
}

func hasWord(src string, i int, w string) bool {
	return len(src)-i >= len(w) && src[i:i+len(w)] == w
}

// defTail matches everything after the leading character, starting at q.
func defTail(src string, start, q int) []int {
	for _, opt := range [...]string{"export", "default", "async"} {
		if hasWord(src, q, opt) {
			if j := skipSpace(src, q+len(opt)); j >= 0 {
				q = j
			}
		}
	}
	var after int
	switch {
	case hasWord(src, q, "function"):
		after = q + len("function")
		if after < len(src) && src[after] == '*' {
			after++
		}
	case hasWord(src, q, "class"):
		after = q + len("class")
	case hasWord(src, q, "const"):
		after = q + len("const")
	case hasWord(src, q, "let"):
		after = q + len("let")
	case hasWord(src, q, "var"):
		after = q + len("var")
	case hasWord(src, q, "interface"):
		after = q + len("interface")
	case hasWord(src, q, "type"):
		after = q + len("type")
	case hasWord(src, q, "enum"):
		after = q + len("enum")
	default:
		return nil
	}
	j := skipSpace(src, after)
	if j < 0 || j >= len(src) || !isWordStart(src[j]) {
		return nil
	}
	e := j + 1
	for e < len(src) && isWordPart(src[e]) {
		e++
	}
	return []int{start, e, j, e}
}
