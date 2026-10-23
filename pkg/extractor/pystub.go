package extractor

import (
	"regexp"
	"strings"
)

// The Python counterpart of stub.go: a Flask/FastAPI handler that only
// raises NotImplementedError, answers 501/500, aborts, or is `pass` / `...` is
// registered but not built (found by audit, 2026-10-10: stub detection was
// JavaScript only).

var (
	pyDefLine   = regexp.MustCompile(`^([ \t]*)(?:async[ \t]+)?def[ \t]+\w+[^\n]*:[ \t]*(?:#[^\n]*)?$`)
	pyStubStmts = []*regexp.Regexp{
		regexp.MustCompile(`^(?:pass|\.\.\.)$`),
		regexp.MustCompile(`^raise\s+(?:[\w.]+\.)?NotImplemented\w*`),
		regexp.MustCompile(`(?s)^raise\s+(?:[\w.]+\.)?HTTPException\s*\(.*status_code\s*=\s*50[01]\b`),
		regexp.MustCompile(`^abort\s*\(\s*50[01]\b`),
		regexp.MustCompile(`(?is)^return\b.*not[ _-]?implemented`),
	}
	pyReturnErr = regexp.MustCompile(`(?s)^return\b.*\b50[01]\b`)
	pyReturnOK  = regexp.MustCompile(`\b20\d\b`)
)

// pyHandlerBody is the body of the function defined right after the decorator
// that ends at `from`, or "" if no def follows (other decorators, blank lines
// and comments may sit in between).
func pyHandlerBody(src string, from int) string {
	ls := strings.Split(src[from:], "\n")
	i := 0
	// the rest of the decorator's own line, then decorators / blanks / comments
	i++
	for i < len(ls) {
		t := strings.TrimSpace(ls[i])
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "@") {
			i++
			continue
		}
		break
	}
	if i >= len(ls) {
		return ""
	}
	m := pyDefLine.FindStringSubmatch(strings.TrimRight(ls[i], "\r"))
	if m == nil {
		return ""
	}
	indent := len(m[1])
	var body []string
	for j := i + 1; j < len(ls); j++ {
		line := strings.TrimRight(ls[j], "\r")
		if strings.TrimSpace(line) == "" {
			body = append(body, "")
			continue
		}
		if len(line)-len(strings.TrimLeft(line, " \t")) <= indent {
			break
		}
		body = append(body, line)
	}
	return strings.Join(body, "\n")
}

// isPyStub: every statement in the handler body is an error or filler (pass,
// ...), and at least one is an explicit error. A body of only `pass` is not
// called a stub: it says nothing, and the same empty handler is common in
// examples and abstract methods (JavaScript's empty handler isn't flagged either).
func isPyStub(body string) bool {
	stmts := pyStatements(body)
	explicit := false
	for _, st := range stmts {
		if st == "pass" || st == "..." {
			continue
		}
		if !pyStubStatement(st) {
			return false
		}
		explicit = true
	}
	return explicit
}

func pyStubStatement(st string) bool {
	for _, re := range pyStubStmts {
		if re.MatchString(st) {
			return true
		}
	}
	return pyReturnErr.MatchString(st) && !pyReturnOK.MatchString(st)
}

// pyStatements splits a body into statements: comment lines and a leading
// docstring dropped, bracketed continuation lines joined.
func pyStatements(body string) []string {
	body = stripLeadingDocstring(body)
	var out []string
	cur, depth := "", 0
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if cur == "" && (t == "" || strings.HasPrefix(t, "#")) {
			continue
		}
		cur += " " + t
		depth += strings.Count(t, "(") + strings.Count(t, "[") + strings.Count(t, "{") -
			strings.Count(t, ")") - strings.Count(t, "]") - strings.Count(t, "}")
		if depth <= 0 {
			out = append(out, strings.TrimSpace(cur))
			cur, depth = "", 0
		}
	}
	if strings.TrimSpace(cur) != "" {
		out = append(out, strings.TrimSpace(cur))
	}
	return out
}

func stripLeadingDocstring(body string) string {
	t := strings.TrimLeft(body, " \t\r\n")
	for _, q := range []string{`"""`, `'''`} {
		if strings.HasPrefix(t, q) {
			if end := strings.Index(t[3:], q); end >= 0 {
				return t[3+end+3:]
			}
			return ""
		}
	}
	for _, q := range []string{`"`, `'`} { // a one-line docstring
		if strings.HasPrefix(t, q) {
			if end := strings.Index(t[1:], q); end >= 0 {
				if nl := strings.IndexByte(t, '\n'); nl < 0 || end+1 < nl {
					return t[end+2:]
				}
			}
		}
	}
	return body
}
