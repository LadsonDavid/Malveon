package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/beta-test/internal/graph"
)

// callSite is a raw hit before we've decided whether its argument is a
// resolvable literal.
type callSite struct {
	kind          graph.Kind
	method        string // "" if not applicable
	line          int
	argRaw        string // raw text of the first argument, unparsed
	enclosingFunc string // name of the innermost named function containing this call, "" if top-level/unknown
	bodyFields    []string // request fields sent (NetworkCall) or read via req.body (RouteHandler); nil if unresolved
}

// funcRange is one named function's line span, used to work out which
// function (if any) a given call site sits inside — a structural fact
// the overlap check's reachability signal is built on (internal/checks/overlap).
type funcRange struct {
	name      string
	startLine int
	endLine   int
}

// enclosingFuncFor returns the name of the smallest funcRange containing
// line, or "" if none does (i.e. the line is top-level module code).
func enclosingFuncFor(ranges []funcRange, line int) string {
	best := ""
	bestSpan := -1
	for _, r := range ranges {
		if line < r.startLine || line > r.endLine {
			continue
		}
		span := r.endLine - r.startLine
		if bestSpan == -1 || span < bestSpan {
			bestSpan = span
			best = r.name
		}
	}
	return best
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

	// Named JS/TS functions whose body might contain a call site:
	// function name(...) { ... }, and const/let/var name = (...) => { ... }
	// or = function(...) { ... }. Anonymous functions have no name to
	// reference elsewhere, so they're deliberately not tracked here.
	jsFuncDeclPattern   = regexp.MustCompile(`(?:^|\W)(?:async\s+)?function\s+(\w+)\s*\([^)]*\)\s*\{`)
	jsFuncAssignPattern = regexp.MustCompile(`\b(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?(?:function\s*\([^)]*\)|\([^)]*\)\s*=>|[\w$]+\s*=>)\s*\{`)
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
		readFields, _ := jsReadFieldsFromHandler(readArgsList(src, openParen))
		sites = append(sites, callSite{
			kind:       graph.RouteHandler,
			method:     strings.ToUpper(src[m[2]:m[3]]),
			line:       lineOf(src, m[0]),
			argRaw:     arg,
			bodyFields: readFields,
		})
	}
	for _, m := range jsFetchPattern.FindAllStringIndex(src, -1) {
		openParen := m[1] - 1
		arg := readFirstArg(src, openParen)
		argsList := readArgsList(src, openParen)
		sentFields, _ := jsSentFieldsFromCallArgs(argsList)
		sites = append(sites, callSite{
			kind:       graph.NetworkCall,
			method:     fetchMethod(argsList),
			line:       lineOf(src, m[0]),
			argRaw:     arg,
			bodyFields: sentFields,
		})
	}
	for _, m := range jsAxiosPattern.FindAllStringSubmatchIndex(src, -1) {
		openParen := m[1] - 1
		arg := readFirstArg(src, openParen)
		sentFields, _ := jsSentFieldsFromCallArgs(readArgsList(src, openParen))
		sites = append(sites, callSite{
			kind:       graph.NetworkCall,
			method:     strings.ToUpper(src[m[2]:m[3]]),
			line:       lineOf(src, m[0]),
			argRaw:     arg,
			bodyFields: sentFields,
		})
	}

	ranges := jsFuncRanges(src)
	for i := range sites {
		sites[i].enclosingFunc = enclosingFuncFor(ranges, sites[i].line)
	}
	return sites
}

// jsFuncRanges finds every named function declaration/assignment in src
// and returns its name and line span, by locating the opening brace after
// the signature and walking forward to its balanced match.
func jsFuncRanges(src string) []funcRange {
	var ranges []funcRange
	for _, pat := range []*regexp.Regexp{jsFuncDeclPattern, jsFuncAssignPattern} {
		for _, m := range pat.FindAllStringSubmatchIndex(src, -1) {
			name := src[m[2]:m[3]]
			openBrace := m[1] - 1
			closeBrace := matchBrace(src, openBrace)
			ranges = append(ranges, funcRange{
				name:      name,
				startLine: lineOf(src, m[0]),
				endLine:   lineOf(src, closeBrace),
			})
		}
	}
	return ranges
}

// matchBrace walks forward from an opening '{' and returns the index of
// its balanced closing '}', respecting quotes so a brace inside a string
// or template literal doesn't throw the count off.
func matchBrace(src string, openBraceIdx int) int {
	depth := 0
	var quote byte
	i := openBraceIdx
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
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
		i++
	}
	return len(src) - 1
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

	ranges := pyFuncRanges(src)
	for i := range sites {
		sites[i].enclosingFunc = enclosingFuncFor(ranges, sites[i].line)
	}
	return sites
}

var pyDefPattern = regexp.MustCompile(`(?m)^([ \t]*)def\s+(\w+)\s*\(`)

// pyFuncRanges finds every "def name(" and, since Python scopes by
// indentation rather than braces, walks forward line by line until a
// non-blank line dedents back to (or past) the def's own indentation —
// that's where the function body ends.
func pyFuncRanges(src string) []funcRange {
	lines := strings.Split(src, "\n")
	var ranges []funcRange
	for _, m := range pyDefPattern.FindAllStringSubmatchIndex(src, -1) {
		indent := src[m[2]:m[3]]
		name := src[m[4]:m[5]]
		startLine := lineOf(src, m[0])
		endLine := len(lines)
		for i := startLine; i < len(lines); i++ { // lines[i] is 1-indexed line i+1, i.e. the line after startLine
			text := lines[i]
			if strings.TrimSpace(text) == "" {
				continue
			}
			if len(leadingWhitespace(text)) <= len(indent) {
				endLine = i // 0-indexed i == 1-indexed line i, the last body line
				break
			}
		}
		ranges = append(ranges, funcRange{name: name, startLine: startLine, endLine: endLine})
	}
	return ranges
}

func leadingWhitespace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i]
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
