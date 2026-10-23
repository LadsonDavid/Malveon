package extractor

// codeMap says which bytes of a source file are real code, and which are
// inside a comment or inside a string/regex literal. The scanners find call
// sites with patterns over the raw text; without this, `// app.post('/x')`
// and `"call fetch('/x')"` were read as a route and a call (found by audit,
// 2026-10-10). Patterns still run on the original text, so arguments are
// read exactly as written; a match only counts if it starts in code.
//
// Not a parser: a small scanner with one rule that keeps its mistakes cheap —
// a ' or " string ends at the end of its line, so an apostrophe in JSX text
// ("Don't") can blank at most the rest of that line.
type codeMap []bool

// syntax is the handful of lexical differences between the languages
// malveon reads.
type syntax struct {
	slashComments bool // // and /* */
	hashComments  bool // # to end of line
	templates     bool // `...` with ${...} (JavaScript/TypeScript)
	triple        bool // """ and ''' strings (Python)
	regexLiterals bool // /.../ (JavaScript/TypeScript)
	rustChars     bool // 'a' is a char but 'a is a lifetime
}

var (
	jsSyntax   = syntax{slashComments: true, templates: true, regexLiterals: true}
	cLikeSyn   = syntax{slashComments: true}
	rustSyntax = syntax{slashComments: true, rustChars: true}
	phpSyntax  = syntax{slashComments: true, hashComments: true}
	hashSyntax = syntax{hashComments: true}
	pySyntax   = syntax{hashComments: true, triple: true}
)

// newCodeMap scans src once.
func newCodeMap(src string, sx syntax) codeMap {
	m, _ := lex(src, sx)
	return m
}

// Lex is newCodeMap plus which of the non-code bytes are comments (the rest
// are string or regex literals). ext picks the language's rules (".py", ".go",
// otherwise JavaScript/TypeScript).
func Lex(src, ext string) (code, comment []bool) {
	m, c := lex(src, definitionSyntax(ext))
	return m, c
}

func lex(src string, sx syntax) (codeMap, []bool) {
	m := make(codeMap, len(src))
	comment := make([]bool, len(src))
	for i := range m {
		m[i] = true
	}
	blank := func(from, to int) {
		for k := from; k < to && k < len(m); k++ {
			if src[k] != '\n' {
				m[k] = false
			}
		}
	}
	blankC := func(from, to int) {
		blank(from, to)
		for k := from; k < to && k < len(m); k++ {
			if src[k] != '\n' {
				comment[k] = true
			}
		}
	}
	var prev byte // last significant (non-space) code byte, for telling / (divide) from /regex/
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case sx.slashComments && c == '/' && i+1 < len(src) && src[i+1] == '/',
			sx.hashComments && c == '#':
			end := indexByteFrom(src, '\n', i)
			blankC(i, end)
			i = end
			continue
		case sx.slashComments && c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := indexFrom(src, "*/", i+2)
			if end < 0 {
				end = len(src)
			} else {
				end += 2
			}
			blankC(i, end)
			i = end
			continue
		case sx.templates && c == '`':
			end := skipTemplate(src, i)
			blank(i+1, end-1)
			i = end
			prev = '`'
			continue
		case c == '"' || c == '\'':
			if sx.rustChars && c == '\'' && !rustCharLiteral(src, i) {
				i++ // a lifetime: 'a
				continue
			}
			if sx.triple && i+2 < len(src) && src[i+1] == c && src[i+2] == c {
				end := indexFrom(src, src[i:i+3], i+3)
				if end < 0 {
					end = len(src)
					blank(i+3, end)
					i = end
				} else {
					blank(i+3, end)
					i = end + 3
				}
				prev = c
				continue
			}
			end := skipLineString(src, i)
			blank(i+1, end-1)
			i = end
			prev = c
			continue
		}
		// A regex literal, only when it really closes on its own line: a / that
		// finds no closing / is a division or JSX markup, and blanking to the end
		// of the line would hide the code after it.
		if sx.regexLiterals && c == '/' {
			if end := regexLiteralEnd(src, i, prev); end > 0 {
				blank(i+1, end-1)
				i = end
				prev = '/'
				continue
			}
		}
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			prev = c
		}
		i++
	}
	return m, comment
}

// codeAt reports whether the first non-space byte at or after off is code:
// the test a pattern match has to pass to count.
func (m codeMap) codeAt(src string, off int) bool {
	for off < len(src) && (src[off] == ' ' || src[off] == '\t' || src[off] == '\n' || src[off] == '\r') {
		off++
	}
	return off < len(m) && m[off]
}

func indexByteFrom(s string, b byte, from int) int {
	for i := from; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return len(s)
}

func indexFrom(s, sub string, from int) int {
	for i := from; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// skipLineString returns the index just past the string starting at i, which
// ends at its closing quote or, unterminated, at the end of the line.
func skipLineString(src string, i int) int {
	q := src[i]
	for k := i + 1; k < len(src); k++ {
		switch src[k] {
		case '\\':
			k++
		case q:
			return k + 1
		case '\n':
			return k
		}
	}
	return len(src)
}

// skipTemplate returns the index just past the template literal starting at
// i, treating ${...} as nested code (which can hold more templates).
func skipTemplate(src string, i int) int {
	for k := i + 1; k < len(src); k++ {
		switch src[k] {
		case '\\':
			k++
		case '`':
			return k + 1
		case '$':
			if k+1 < len(src) && src[k+1] == '{' {
				depth := 0
				for k = k + 1; k < len(src); k++ {
					switch src[k] {
					case '{':
						depth++
					case '}':
						depth--
					case '`':
						k = skipTemplate(src, k) - 1
					case '"', '\'':
						k = skipLineString(src, k) - 1
					}
					if depth == 0 {
						break
					}
				}
			}
		}
	}
	return len(src)
}

// regexAllowed: a / starts a regex literal (not a division) when the last
// significant byte can't end an expression.
func regexAllowed(src string, i int, prev byte) bool {
	if i+1 < len(src) && (src[i+1] == '/' || src[i+1] == '*' || src[i+1] == '>') {
		return false // a comment, or the /> that ends a JSX tag
	}
	if i > 0 && src[i-1] == '<' {
		return false // </tag>: a JSX closing tag, not a regex after a less-than
	}
	switch prev {
	case 0, '(', ',', '=', ':', '[', '!', '&', '|', '?', '{', '}', ';', '+', '-', '*', '%', '<', '>', '~', '^':
		return true
	}
	return false
}

// regexLiteralEnd is the index just past the regex literal starting at i, or
// -1 when the / isn't one (a division, JSX, or no closing / on the line).
func regexLiteralEnd(src string, i int, prev byte) int {
	if !regexAllowed(src, i, prev) {
		return -1
	}
	inClass := false
	for k := i + 1; k < len(src); k++ {
		switch src[k] {
		case '\\':
			k++
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '/':
			if !inClass {
				return k + 1
			}
		case '\n':
			return -1
		}
	}
	return -1
}

// rustCharLiteral: 'x' or '\n' (a char) rather than a lifetime like 'a.
func rustCharLiteral(src string, i int) bool {
	if i+2 < len(src) && src[i+1] != '\\' && src[i+2] == '\'' {
		return true
	}
	return i+3 < len(src) && src[i+1] == '\\' && indexByteFrom(src[i+1:min(i+8, len(src))], '\'', 0) < 7
}

// maskComments returns src with every comment byte replaced by a space (line
// breaks kept), so offsets and line numbers stay the same. The readers that
// split code at commas and brackets are aware of quotes but not of comments: the
// apostrophe in `// we'll use the next cursor` would otherwise open a string
// that swallows the rest of the file.
func maskComments(src string, sx syntax) string {
	_, comment := lex(src, sx)
	b := []byte(src)
	changed := false
	for i, c := range comment {
		if c && b[i] != '\n' && b[i] != '\r' {
			b[i] = ' '
			changed = true
		}
	}
	if !changed {
		return src
	}
	return string(b)
}
