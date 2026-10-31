package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// callSite is a raw hit before we've decided whether its argument is a
// resolvable literal.
type callSite struct {
	kind           graph.Kind
	method         string // "" if not applicable
	line           int
	argRaw         string   // raw text of the first argument, unparsed
	enclosingFunc  string   // name of the innermost named function containing this call, "" if top-level/unknown
	bodyFields     []string // request fields sent (NetworkCall) or read via req.body (RouteHandler); nil if unresolved
	tail           bool     // argRaw is only the end of the address (see graph.Node.Tail)
	requiredFields []string // RouteHandler: see graph.Node.RequiredFields
	receiver       string   // Python route: the router variable it hangs on
	unmounted      bool     // a route of a router its own file never mounts
	stub           bool     // RouteHandler whose handler only answers with an error
	unreferenced   bool     // NetworkCall inside a function nothing in the project calls
	op             string   // DataCall only: the method chained right after .from(...), or the auth method
	fileRouted     bool     // the URL was derived from where the file sits (see graph.Node.FileRouted)
	rpc            bool     // a tRPC procedure or a call to one (see graph.Node.RPC)
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
// enclosingResolver is enclosingFuncFor remembering its answers by line: a
// minified bundle puts thousands of call sites on a few lines, and asking the
// whole function list for each was most of the scan time.
func enclosingResolver(ranges []funcRange) func(line int) string {
	memo := map[int]string{}
	return func(line int) string {
		if v, ok := memo[line]; ok {
			return v
		}
		v := enclosingFuncFor(ranges, line)
		memo[line] = v
		return v
	}
}

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
	jsRouteChain   = regexp.MustCompile(`\b\w+\s*\.route\s*\(`)
	jsChainedVerb  = regexp.MustCompile(`\)\s*\.\s*(get|post|put|delete|patch)\s*\(`)
	jsRoutePattern = regexp.MustCompile(`(?i)\b(\w+)\.(get|post|put|delete|patch)\s*\(`)

	// frontend network calls
	jsFetchPattern = regexp.MustCompile(`\bfetch\s*\(`)
	jsAxiosPattern = regexp.MustCompile(`(?i)\baxios\.(get|post|put|delete|patch)\s*\(`)

	// direct database calls through the Supabase client:
	// supabase.from('profiles'), supabase.storage.from('avatars'),
	// supabase.rpc('generate_referral_code'). The word before ".from" is
	// captured so Array.from('x')/Buffer.from('x') are never read as tables.
	supabaseFromPattern = regexp.MustCompile(`(\w*)\s*\)?\s*\.from\s*\(\s*['"]([A-Za-z_][\w-]*)['"]\s*\)`)
	supabaseRPCPattern  = regexp.MustCompile(`\.rpc\s*\(\s*['"]([A-Za-z_]\w*)['"]`)
	// supabase.auth.resetPasswordForEmail(...), supabase.functions.invoke('send-email')
	supabaseAuthPattern   = regexp.MustCompile(`\.auth\.(\w+)\s*\(`)
	supabaseInvokePattern = regexp.MustCompile(`\.functions\.invoke\s*\(\s*['"]([\w-]+)['"]`)
	// the method chained directly after .from('t'): .select( .insert( .upload( ...
	chainedOpPattern = regexp.MustCompile(`^\s*\.\s*(\w+)\s*\(`)

	// Python: @app.route("/x"), @app.get("/x"), @router.post("/x")
	pyRoutePattern = regexp.MustCompile(`(?i)@\w+\.(route|get|post|put|delete|patch)\s*\(`)
	// Python: requests.get("/x"), requests.post("/x")
	pyRequestPattern = regexp.MustCompile(`(?i)\brequests\.(get|post|put|delete|patch)\s*\(`)

	// Named JS/TS functions whose body might contain a call site:
	// function name(...) { ... }, and const/let/var name = (...) => { ... }
	// or = function(...) { ... }. Anonymous functions have no name to
	// reference elsewhere, so they're deliberately not tracked here.
	// An optional TypeScript return type (`): Promise<void> {`) is allowed
	// before the body — without it, a typed function's body was never found.
	jsFuncDeclPattern   = regexp.MustCompile(`(?:^|\W)(?:async\s+)?function\s+(\w+)\s*\([^)]*\)\s*(?::\s*[^{;]+)?\{`)
	jsFuncAssignPattern = regexp.MustCompile(`\b(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?(?:function\s*\([^)]*\)\s*(?::\s*[^{;]+)?|\([^)]*\)\s*(?::\s*[^=;{]+)?=>|[\w$]+\s*=>)\s*\{`)
)

// scanJSLike extracts route/network call sites from JavaScript or
// TypeScript source using pattern matching plus a hand-rolled argument
// scanner — not a full parser, deliberately: it never claims more than
// "found a call shaped like a route/network call, here's its first
// literal argument if it has one."
func scanJSLike(src string) []callSite { return scanJSLikeWith(src, nil) }

// scanJSLikeWith is scanJSLike given what the whole project does with
// requests (jsInfo): axios.create() instances and fetch wrappers. nil info
// reads one file alone.
func scanJSLikeWith(src string, info *jsInfo) []callSite {
	cm := newCodeMap(src, jsSyntax)
	consts := stringConsts(src, cm)
	alts := stringAlts(src, cm)
	// `import axios from "../api/axios"`: an instance with its own baseURL, not the library
	axiosIsLocal := strings.Contains(src, "axios") && strings.HasPrefix(importTable(src)["axios"], ".")
	var sites []callSite

	for _, m := range jsRoutePattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		if recv := src[m[2]:m[3]]; httpClientReceivers[strings.ToLower(recv)] || info.isClient(recv) {
			continue // axios.post(...) is a call out, not a route registration
		}
		openParen := m[1] - 1
		arg := readFirstArg(src, openParen)
		readFields, _ := jsReadFieldsFromHandler(readArgsList(src, openParen))
		handlerArgs := splitTopLevelArgs(readArgsList(src, openParen))
		stub := len(handlerArgs) > 1 && isStubHandler(handlerArgs[len(handlerArgs)-1])
		sites = append(sites, callSite{
			stub:           stub,
			requiredFields: requiredFieldsIn(lastArg(handlerArgs), readFields),
			kind:           graph.RouteHandler,
			method:         strings.ToUpper(src[m[4]:m[5]]),
			line:           lineOf(src, m[0]),
			argRaw:         arg,
			bodyFields:     readFields,
		})
	}
	// router.route('/p').get(h).post(h): the chained form of the same registration.
	chains := jsRouteChain.FindAllStringIndex(src, -1)
	for ci, m := range chains {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		open := m[1] - 1
		arg := readFirstArg(src, open)
		end := min(len(src), m[1]+4000)
		if ci+1 < len(chains) && chains[ci+1][0] < end {
			end = chains[ci+1][0] // the next route() starts a new chain
		}
		if i := strings.IndexByte(src[m[1]:end], ';'); i >= 0 {
			end = m[1] + i
		}
		for _, c := range jsChainedVerb.FindAllStringSubmatchIndex(src[m[1]:end], -1) {
			if !cm.codeAt(src, m[1]+c[0]) {
				continue
			}
			sites = append(sites, callSite{
				kind:   graph.RouteHandler,
				method: strings.ToUpper(src[m[1]+c[2] : m[1]+c[3]]),
				line:   lineOf(src, m[1]+c[0]),
				argRaw: arg,
			})
		}
	}
	for _, m := range jsFetchPattern.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		openParen := m[1] - 1
		arg := withConsts(readFirstArg(src, openParen), consts)
		argsList := readArgsList(src, openParen)
		sentFields, _ := jsSentFieldsFromCallArgs(argsList, false)
		for _, a := range urlAlternatives(arg, alts) {
			sites = append(sites, callSite{
				kind:       graph.NetworkCall,
				method:     fetchMethod(argsList),
				line:       lineOf(src, m[0]),
				argRaw:     tailArg(a),
				tail:       isTail(a),
				bodyFields: sentFields,
			})
		}
	}
	for _, m := range jsAxiosPattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		openParen := m[1] - 1
		arg := withConsts(readFirstArg(src, openParen), consts)
		sentFields, _ := jsSentFieldsFromCallArgs(readArgsList(src, openParen), true)
		for _, a := range urlAlternatives(arg, alts) {
			sites = append(sites, callSite{
				kind:       graph.NetworkCall,
				method:     strings.ToUpper(src[m[2]:m[3]]),
				line:       lineOf(src, m[0]),
				argRaw:     tailArg(a),
				tail:       isTail(a) || axiosIsLocal,
				bodyFields: sentFields,
			})
		}
	}

	for _, m := range supabaseFromPattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		receiver := src[m[2]:m[3]]
		if notSupabaseReceivers[receiver] {
			continue
		}
		method := "TABLE"
		if receiver == "storage" {
			method = "STORAGE"
		}
		op := ""
		if c := chainedOpPattern.FindStringSubmatch(src[m[1]:]); c != nil {
			op = c[1]
		}
		sites = append(sites, callSite{kind: graph.DataCall, method: method, line: lineOf(src, m[0]), argRaw: src[m[4]:m[5]], op: op})
	}
	for _, m := range supabaseRPCPattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		sites = append(sites, callSite{kind: graph.DataCall, method: "RPC", line: lineOf(src, m[0]), argRaw: src[m[2]:m[3]]})
	}
	for _, m := range supabaseAuthPattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		sites = append(sites, callSite{kind: graph.DataCall, method: "AUTH", line: lineOf(src, m[0]), argRaw: src[m[2]:m[3]], op: src[m[2]:m[3]]})
	}
	for _, m := range supabaseInvokePattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		sites = append(sites, callSite{kind: graph.DataCall, method: "FUNCTION", line: lineOf(src, m[0]), argRaw: src[m[2]:m[3]]})
	}

	sites = append(sites, scanORM(src, cm, info)...)
	sites = append(sites, scanJSShapes(src, cm, consts, info)...)

	ranges := jsFuncRanges(src)
	encl := enclosingResolver(ranges)
	for i := range sites {
		sites[i].enclosingFunc = encl(sites[i].line)
		// A call inside a function nothing in the project mentions (and that
		// isn't exported) is never made.
		if f := sites[i].enclosingFunc; sites[i].kind == graph.NetworkCall && info.unreferenced(f) && !exportedDecl(src, f) {
			sites[i].unreferenced = true
		}
	}
	return sites
}

// httpClientReceivers are HTTP client libraries: x.get('/path') on one of
// these makes a request, it never registers a route.
var httpClientReceivers = map[string]bool{
	"axios": true, "got": true, "ky": true, "superagent": true, "http": true, "https": true, "httpclient": true,
}

// notSupabaseReceivers are built-ins with their own .from(...) that take
// a string — never a database table.
var notSupabaseReceivers = map[string]bool{
	"Array": true, "Buffer": true, "Uint8Array": true, "Int8Array": true, "Set": true,
	"Map": true, "Object": true, "String": true, "Blob": true, "Promise": true,
}

// jsFuncRanges finds every named function declaration/assignment in src
// and returns its name and line span, by locating the opening brace after
// the signature and walking forward to its balanced match.
func jsFuncRanges(src string) []funcRange {
	cm := newCodeMap(src, jsSyntax)
	var ranges []funcRange
	for _, pat := range []*regexp.Regexp{jsFuncDeclPattern, jsFuncAssignPattern} {
		for _, m := range pat.FindAllStringSubmatchIndex(src, -1) {
			if !cm.codeAt(src, m[0]) {
				continue
			}
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
	cm := newCodeMap(src, pySyntax)
	var sites []callSite
	prefixes, openRouters := pyRouterPrefixes(src, cm)

	for _, m := range pyRoutePattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		openParen := m[1] - 1
		receiver := pyReceiver(src[m[0]:m[1]])
		stub := isPyStub(pyHandlerBody(src, m[1]-1+len(readBalanced(src, m[1]-1))))
		arg := withPrefix(readFirstArg(src, openParen), prefixes[receiver])
		method := strings.ToUpper(src[m[2]:m[3]])
		methods := []string{method}
		if method == "ROUTE" {
			// @app.route(path, methods=["GET", "POST"]): one route per
			// listed method, when every entry is a plain string. No list, or
			// anything not literal, stays unknown rather than guessed
			// (2026-09-29: an unknown method let a GET-only route pass for a
			// plan line asking for DELETE).
			methods = pyRouteMethods(callArgs(src, openParen))
			if len(methods) == 0 {
				methods = []string{""}
			}
		}
		for _, meth := range methods {
			sites = append(sites, callSite{
				kind:      graph.RouteHandler,
				method:    meth,
				line:      lineOf(src, m[0]),
				argRaw:    arg,
				receiver:  receiver,
				unmounted: openRouters[receiver],
				stub:      stub,
			})
		}
	}
	for _, m := range pyRequestPattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
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
	encl := enclosingResolver(ranges)
	for i := range sites {
		sites[i].enclosingFunc = encl(sites[i].line)
	}
	return sites
}

var (
	pyMethodsKwarg = regexp.MustCompile(`\bmethods\s*=\s*[\[(]([^\])]*)[\])]`)
	pyMethodName   = regexp.MustCompile(`^\s*["'](GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)["']\s*$`)
)

// pyRouteMethods reads a Flask methods=[...] list from a route call's
// arguments. Every entry must be a plain string naming an HTTP method,
// else nil (unknown).
func pyRouteMethods(args string) []string {
	m := pyMethodsKwarg.FindStringSubmatch(args)
	if m == nil {
		return nil
	}
	var out []string
	for _, item := range strings.Split(m[1], ",") {
		if strings.TrimSpace(item) == "" {
			continue
		}
		mm := pyMethodName.FindStringSubmatch(strings.ToUpper(item))
		if mm == nil {
			return nil
		}
		out = append(out, mm[1])
	}
	return out
}

// callArgs returns the text between the parenthesis at open and its match,
// skipping over quoted strings; "" if it isn't closed.
func callArgs(src string, open int) string {
	depth := 0
	var quote byte
	for i := open; i < len(src); i++ {
		c := src[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return src[open+1 : i]
			}
		}
	}
	return ""
}

var pyDefPattern = regexp.MustCompile(`(?m)^([ \t]*)def\s+(\w+)\s*\(`)

// pyFuncRanges finds every "def name(" and, since Python scopes by
// indentation rather than braces, walks forward line by line until a
// non-blank line dedents back to (or past) the def's own indentation —
// that's where the function body ends.
func pyFuncRanges(src string) []funcRange {
	cm := newCodeMap(src, pySyntax)
	lines := strings.Split(src, "\n")
	var ranges []funcRange
	for _, m := range pyDefPattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
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
	return lines.lineOf(src, byteOffset)
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
	if fstring && strings.Contains(inner, "{") {
		return "", false
	}
	if i := queryStart(inner); i >= 0 && !optionalCatchAllEnd(inner, i) {
		inner = inner[:i] // the query string / fragment isn't part of the route
	}
	if inner == "" || inner[0] != '/' {
		return "", false // not a path-looking literal
	}
	if quote == '`' && strings.Contains(inner, "${") {
		return templatePath(inner)
	}
	return inner, true
}

// optionalCatchAllEnd: the ? at i ends a segment written :...name? — an optional
// catch-all (Next.js [[...slug]], Nest's `*`), not the start of a query string.
func optionalCatchAllEnd(path string, i int) bool {
	if path[i] != '?' || (i+1 < len(path) && path[i+1] != '/') {
		return false
	}
	start := strings.LastIndexByte(path[:i], '/') + 1
	return strings.HasPrefix(path[start:i], ":...")
}

// templatePath resolves a JS template-literal path like
// `/api/circles/${id}/members` to /api/circles/:param/members — but only
// when every ${...} fills a whole path segment on its own. A value mixed
// into a segment (`/files/report-${id}`) or a dynamic prefix
// (`${BASE}/jobs`) can't be matched to a route honestly, so it stays
// unresolved. Added 2026-09-23: before this, every templated URL was
// unresolvable, so a real call to /profile/${userId} never counted.
func templatePath(inner string) (string, bool) {
	segs := strings.Split(inner, "/")
	for i, seg := range segs {
		if !strings.Contains(seg, "${") {
			continue
		}
		if strings.HasPrefix(seg, "${") && strings.HasSuffix(seg, "}") && strings.Count(seg, "${") == 1 && strings.Count(seg, "}") == 1 {
			segs[i] = ":param"
			continue
		}
		return "", false
	}
	return strings.Join(segs, "/"), true
}

// queryStart is the index of the "?" or "#" that begins a query string or fragment, ignoring one
// inside a ${...} placeholder (`${team?.slug}` is optional chaining, not a query). -1 if none.
func queryStart(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '$' && i+1 < len(s) && s[i+1] == '{':
			depth++
			i++
		case s[i] == '}' && depth > 0:
			depth--
		case depth == 0 && (s[i] == '?' || s[i] == '#'):
			return i
		}
	}
	return -1
}
