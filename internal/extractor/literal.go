package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/beta-test/internal/graph"
)

// callSite is a raw hit before we've decided whether its argument is a
// resolvable literal.
type callSite struct {
	kind   graph.Kind
	method string // "" if not applicable
	line   int
	argRaw string // raw text of the first argument, unparsed
}

var (
	// backend route registrations: app.get("/x", ...), router.post('/x', ...)
	jsRoutePattern = regexp.MustCompile(`(?i)\b\w+\.(get|post|put|delete|patch)\s*\(`)

	// frontend network calls
	jsFetchPattern = regexp.MustCompile(`\bfetch\s*\(`)
	jsAxiosPattern = regexp.MustCompile(`(?i)\baxios\.(get|post|put|delete|patch)\s*\(`)

	// Python: @app.route("/x"), @app.get("/x"), @router.post("/x")
	pyRoutePattern = regexp.MustCompile(`(?i)@\w+\.(route|get|post|put|delete|patch)\s*\(`)
	// Python: requests.get("/x"), requests.post("/x")
	pyRequestPattern = regexp.MustCompile(`(?i)\brequests\.(get|post|put|delete|patch)\s*\(`)
)

// scanJSLike extracts route/network call sites from JavaScript or
// TypeScript source using pattern matching plus a hand-rolled argument
// scanner — not a full parser, deliberately: it never claims more than
// "found a call shaped like a route/network call, here's its first
// literal argument if it has one."
func scanJSLike(src string) []callSite {
	var sites []callSite

	for _, m := range jsRoutePattern.FindAllStringSubmatchIndex(src, -1) {
		openParen := m[1] - 1
		arg := readFirstArg(src, openParen)
		sites = append(sites, callSite{
			kind:   graph.RouteHandler,
			method: strings.ToUpper(src[m[2]:m[3]]),
			line:   lineOf(src, m[0]),
			argRaw: arg,
		})
	}
	for _, m := range jsFetchPattern.FindAllStringIndex(src, -1) {
		openParen := m[1] - 1
		arg := readFirstArg(src, openParen)
		sites = append(sites, callSite{
			kind:   graph.NetworkCall,
			method: fetchMethod(readArgsList(src, openParen)),
			line:   lineOf(src, m[0]),
			argRaw: arg,
		})
	}
	for _, m := range jsAxiosPattern.FindAllStringSubmatchIndex(src, -1) {
		openParen := m[1] - 1
		arg := readFirstArg(src, openParen)
		sites = append(sites, callSite{
			kind:   graph.NetworkCall,
			method: strings.ToUpper(src[m[2]:m[3]]),
			line:   lineOf(src, m[0]),
			argRaw: arg,
		})
	}
	return sites
}

// scanPython extracts Flask/FastAPI-style route decorators and
// requests-library calls from Python source.
func scanPython(src string) []callSite {
	var sites []callSite

	for _, m := range pyRoutePattern.FindAllStringSubmatchIndex(src, -1) {
		openParen := m[1] - 1
		arg := readFirstArg(src, openParen)
		method := strings.ToUpper(src[m[2]:m[3]])
		if method == "ROUTE" {
			method = "" // methods=[...] kwarg not parsed in v1; leave unknown rather than guess
		}
		sites = append(sites, callSite{
			kind:   graph.RouteHandler,
			method: method,
			line:   lineOf(src, m[0]),
			argRaw: arg,
		})
	}
	for _, m := range pyRequestPattern.FindAllStringSubmatchIndex(src, -1) {
		openParen := m[1] - 1
		arg := readFirstArg(src, openParen)
		sites = append(sites, callSite{
			kind:   graph.NetworkCall,
			method: strings.ToUpper(src[m[2]:m[3]]),
			line:   lineOf(src, m[0]),
			argRaw: arg,
		})
	}
	return sites
}

// readFirstArg walks forward from an opening paren and returns the raw
// text of the first top-level argument (stops at the first comma or
// closing paren at depth 0, respecting quotes so a comma inside a string
// doesn't end the argument early).
func readFirstArg(src string, openParenIdx int) string {
	i := openParenIdx + 1
	start := i
	depth := 0
	var quote byte
	for i < len(src) {
		c := src[i]
		if quote != 0 {
			if c == '\\' {
				i += 2
				continue
			}
			if c == quote {
				quote = 0
			}
			i++
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				return strings.TrimSpace(src[start:i])
			}
			depth--
		case ',':
			if depth == 0 {
				return strings.TrimSpace(src[start:i])
			}
		}
		i++
	}
	return strings.TrimSpace(src[start:])
}

// readArgsList returns the raw text of an entire balanced argument list,
// parens included stripped, starting right after openParenIdx.
func readArgsList(src string, openParenIdx int) string {
	i := openParenIdx + 1
	start := i
	depth := 0
	var quote byte
	for i < len(src) {
		c := src[i]
		if quote != 0 {
			if c == '\\' {
				i += 2
				continue
			}
			if c == quote {
				quote = 0
			}
			i++
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				return src[start:i]
			}
			depth--
		}
		i++
	}
	return src[start:]
}

var fetchMethodFieldPattern = regexp.MustCompile(`(?i)method\s*:\s*['"](\w+)['"]`)

// fetchMethod reads the method the Fetch API will actually use: whatever
// the options object's method field says, or GET by spec default when
// there's no options object or no method field. This is the one place a
// default is applied instead of leaving the method unknown — it's a
// real, documented default from the Fetch API, not a guess.
func fetchMethod(argsList string) string {
	if m := fetchMethodFieldPattern.FindStringSubmatch(argsList); m != nil {
		return strings.ToUpper(m[1])
	}
	return "GET"
}

func lineOf(src string, byteOffset int) int {
	return strings.Count(src[:byteOffset], "\n") + 1
}

// literalPath classifies a raw argument: returns (path, true) if it's a
// plain string literal with no interpolation, or ("", false) if it's a
// variable, expression, or an interpolated template/f-string.
func literalPath(raw string) (string, bool) {
	if len(raw) < 2 {
		return "", false
	}
	fstring := false
	body := raw
	if (raw[0] == 'f' || raw[0] == 'F') && len(raw) >= 3 && (raw[1] == '"' || raw[1] == '\'') {
		fstring = true
		body = raw[1:]
	}
	quote := body[0]
	if quote != '\'' && quote != '"' && quote != '`' {
		return "", false
	}
	if body[len(body)-1] != quote {
		return "", false
	}
	inner := body[1 : len(body)-1]
	if quote == '`' && strings.Contains(inner, "${") {
		return "", false
	}
	if fstring && strings.Contains(inner, "{") {
		return "", false
	}
	if inner == "" || inner[0] != '/' {
		return "", false // not a path-looking literal
	}
	return inner, true
}
