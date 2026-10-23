package extractor

import (
	"regexp"
	"sort"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// tRPC. Nothing in tRPC code is a URL: a procedure is a key in a router object
// (`create: publicProcedure.input(...).mutation(...)`), a call is
// `api.post.create.useMutation()`. On the wire, though, every procedure has one
// URL — the adapter's base path plus its dotted name (`/api/trpc/post.create`) —
// and queries are GETs, mutations POSTs. So each procedure becomes a route at that
// URL and each call a call to it, and the checks that follow routes and calls
// (wiring, contract, the path trace) work on tRPC unchanged.
//
// Only what the code proves is used: a call counts only when its path names a
// procedure that exists; a router malveon can't see into (imported from a package,
// built by a spread) leaves an unreadable marker instead of a guess.

type trpcProc struct {
	end  int // last line of the entry (a procedure's own code), 0 when unknown
	key  string
	kind string // query, mutation, subscription; "" = the entry is a reference to look up
	ref  string // the identifier the entry points at (a router, or a procedure defined elsewhere)
	sub  *trpcRouter
	line int
}

type trpcRouter struct {
	file       string
	name       string // the variable it is assigned to, "" for an inline one
	line       int
	entries    []trpcProc
	merges     []trpcProc // mergeRouters(a, b): their procedures sit at this router's level
	incomplete bool       // a spread, a computed key: some procedures can't be named
}

type trpcCand struct {
	file       string
	line       int
	root, path string
	verb       string
	direct     bool // root.a.b(...) with no hook or verb: a server-side caller
}

type trpcProject struct {
	routers     map[string][]*trpcRouter
	procConst   map[string][]string // const name -> kinds seen (ambiguous when more than one distinct)
	clients     map[string]bool     // identifiers bound to a tRPC client or caller
	adapter     []string            // base paths where a server adapter is mounted
	clientBases []string            // base paths the client links point at
	cands       []trpcCand
	order       []*trpcRouter
}

func newTRPCProject() *trpcProject {
	return &trpcProject{
		routers:   map[string][]*trpcRouter{},
		procConst: map[string][]string{},
		clients:   map[string]bool{"api": true, "trpc": true, "client": true, "caller": true, "utils": true, "trpcClient": true},
	}
}

var (
	trpcRouterAssign = regexp.MustCompile(`(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*(?::\s*[^=]+?)?=\s*(?:await\s+)?((?:\w+\.)*(?:router|createTRPCRouter|createRouter|mergeRouters))\s*\(`)
	trpcProcAssign   = regexp.MustCompile(`(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*(?::\s*[^=]+?)?=\s*`)
	trpcClientAssign = regexp.MustCompile(`(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*(?::\s*[^=]+?)?=\s*(?:await\s+)?(?:[\w$]+\s*\.\s*)*(?:createTRPCReact|createTRPCClient|createTRPCProxyClient|createTRPCNext|createTRPCOptionsProxy|useTRPC|createCaller|createCallerFactory|useUtils)\b`)
	trpcRouterCall   = regexp.MustCompile(`^(?:\w+\.)*(?:router|createTRPCRouter|createRouter)\s*\(`)
	trpcMergeCall    = regexp.MustCompile(`^(?:\w+\.)*mergeRouters\s*\(`)
	trpcHookCall     = regexp.MustCompile(`\b([A-Za-z_$][\w$]*)((?:\s*\??\.\s*[A-Za-z_$][\w$]*)+)\s*\??\.\s*(useQuery|useSuspenseQuery|useInfiniteQuery|useSuspenseInfiniteQuery|useMutation|useSubscription|queryOptions|infiniteQueryOptions|mutationOptions|subscriptionOptions|query|mutate|subscribe|prefetch|prefetchInfinite|fetch|fetchInfinite|ensureData)\s*\(`)
	trpcDirectCall   = regexp.MustCompile(`\b([A-Za-z_$][\w$]*)((?:\s*\??\.\s*[A-Za-z_$][\w$]*)+)\s*\(`)
	trpcAdapterNames = regexp.MustCompile(`\b(fetchRequestHandler|createNextApiHandler|createNextRouteHandler|createExpressMiddleware|createHTTPServer|createHTTPHandler|createFastifyHandler|trpcServer|awsLambdaRequestHandler|createKoaMiddleware|createOpenApiNextHandler)\s*\(`)
	trpcEndpointKey  = regexp.MustCompile(`\bendpoint\s*:\s*['"]([^'"]*)['"]`)
	trpcExpressUse   = regexp.MustCompile(`\.\s*use\s*\(\s*['"]([^'"]*)['"]\s*,\s*[\w$.]*createExpressMiddleware`)
	trpcFastifyReg   = regexp.MustCompile(`register\s*\(\s*fastifyTRPCPlugin\s*,\s*\{[^}]*\bprefix\s*:\s*['"]([^'"]+)['"]`)
	trpcLinkCall     = regexp.MustCompile(`\b(?:httpBatchLink|httpLink|httpBatchStreamLink|unstable_httpBatchStreamLink|httpSubscriptionLink)\s*\(`)
	trpcLinkURL      = regexp.MustCompile(`\burl\s*:\s*(?:"([^"]*)"|'([^']*)'|\x60([^\x60]*)\x60)`)
)

// hookVerbs belong to tRPC alone; the other verbs (query, fetch, ...) need a
// root that is known to be a tRPC client.
var hookVerbs = map[string]bool{
	"useQuery": true, "useSuspenseQuery": true, "useInfiniteQuery": true, "useSuspenseInfiniteQuery": true,
	"useMutation": true, "useSubscription": true, "queryOptions": true, "infiniteQueryOptions": true,
	"mutationOptions": true, "subscriptionOptions": true,
}

var verbMethod = map[string]string{
	"useMutation": "POST", "mutate": "POST", "mutationOptions": "POST",
}

// readFile collects what one JavaScript/TypeScript file contributes. rel is its
// root-relative path.
func (tp *trpcProject) readFile(rel, src string) {
	src = maskComments(src, jsSyntax)
	cm := newCodeMap(src, jsSyntax)

	for _, m := range trpcClientAssign.FindAllStringSubmatchIndex(src, -1) {
		if cm.codeAt(src, m[0]) {
			tp.clients[src[m[2]:m[3]]] = true
		}
	}
	for _, m := range trpcRouterAssign.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		name, callee := src[m[2]:m[3]], src[m[4]:m[5]]
		r := &trpcRouter{file: rel, name: name, line: lineOf(src, m[0])}
		tp.fillRouter(r, src, m[1]-1, strings.HasSuffix(callee, "mergeRouters"))
		tp.routers[name] = append(tp.routers[name], r)
		tp.order = append(tp.order, r)
	}
	for _, m := range trpcProcAssign.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		if kind := chainKind(readJSStatement(src, m[1])); kind != "" {
			name := src[m[2]:m[3]]
			if !contains(tp.procConst[name], kind) {
				tp.procConst[name] = append(tp.procConst[name], kind)
			}
		}
	}
	tp.readAdapters(rel, src, cm)

	for _, m := range trpcHookCall.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		tp.cands = append(tp.cands, trpcCand{file: rel, line: lineOf(src, m[0]), root: src[m[2]:m[3]],
			path: dotted(src[m[4]:m[5]]), verb: src[m[6]:m[7]]})
	}
	if strings.Contains(src, "createCaller") || strings.Contains(src, "trpc") || strings.Contains(src, "api.") {
		for _, m := range trpcDirectCall.FindAllStringSubmatchIndex(src, -1) {
			if !cm.codeAt(src, m[0]) {
				continue
			}
			tp.cands = append(tp.cands, trpcCand{file: rel, line: lineOf(src, m[0]), root: src[m[2]:m[3]],
				path: dotted(src[m[4]:m[5]]), direct: true})
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// dotted turns ".post . create" into "post.create".
func dotted(chain string) string {
	return strings.ReplaceAll(strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", "?", "").Replace(chain), "..", ".")[1:]
}

// fillRouter reads the arguments of router(...) / mergeRouters(...): src is the
// whole file and open the offset of the opening parenthesis. Every line number
// comes from an offset in src, so a procedure's span is exact however deeply its
// router is nested.
func (tp *trpcProject) fillRouter(r *trpcRouter, src string, open int, merge bool) {
	if open < 0 || open >= len(src) || src[open] != '(' {
		r.incomplete = true
		return
	}
	args := readBalanced(src, open)
	if len(args) < 2 {
		r.incomplete = true
		return
	}
	inner := args[1 : len(args)-1]
	for _, part := range spans(inner) {
		e, abs := trimmedAt(part.text, open+1+part.off)
		if e == "" {
			continue
		}
		if merge {
			tp.merges(r, src, e, abs)
			continue
		}
		if !strings.HasPrefix(e, "{") {
			r.incomplete = true
			continue
		}
		// router({ a: ..., b: ... }): the entries of the object
		obj := readBalanced(e, 0)
		if len(obj) < 2 {
			r.incomplete = true
			continue
		}
		objInner := obj[1 : len(obj)-1]
		for _, ent := range spans(objInner) {
			ee, eabs := trimmedAt(ent.text, abs+1+ent.off)
			tp.entry(r, src, ee, eabs)
		}
	}
}

// trimmedAt trims s and returns where its first byte sits, given where s starts.
func trimmedAt(s string, at int) (string, int) {
	t := strings.TrimLeft(s, " \t\r\n")
	return strings.TrimRight(t, " \t\r\n"), at + (len(s) - len(t))
}

func (tp *trpcProject) merges(r *trpcRouter, src, e string, abs int) {
	line := lineOf(src, abs)
	switch {
	case jsRef.MatchString(e):
		r.merges = append(r.merges, trpcProc{ref: lastName(e), line: line})
	case trpcRouterCall.MatchString(e):
		sub := &trpcRouter{file: r.file, line: line}
		tp.fillRouter(sub, src, abs+strings.IndexByte(e, '('), false)
		r.merges = append(r.merges, trpcProc{sub: sub, line: line})
	default:
		r.incomplete = true
	}
}

var jsRef = regexp.MustCompile(`^[\w$.]+$`)

// entry classifies one `key: value` of a router object; abs is where it starts in src.
func (tp *trpcProject) entry(r *trpcRouter, src, e string, abs int) {
	if e == "" {
		return
	}
	if strings.HasPrefix(e, "...") {
		r.incomplete = true
		return
	}
	line, end := lineOf(src, abs), lineOf(src, abs+len(e)-1)
	key, value, valueAt, ok := splitKeyValue(e)
	if !ok {
		key, value, valueAt = e, e, 0 // shorthand: { listAll }
	}
	key = strings.Trim(key, `'"`)
	if key == "" || strings.HasPrefix(key, "[") || !plainIdent.MatchString(strings.ReplaceAll(key, "-", "_")) {
		r.incomplete = true
		return
	}
	switch {
	case trpcRouterCall.MatchString(value):
		sub := &trpcRouter{file: r.file, line: line}
		tp.fillRouter(sub, src, abs+valueAt+strings.IndexByte(value, '('), false)
		r.entries = append(r.entries, trpcProc{key: key, sub: sub, line: line})
	case trpcMergeCall.MatchString(value):
		sub := &trpcRouter{file: r.file, line: line}
		tp.fillRouter(sub, src, abs+valueAt+strings.IndexByte(value, '('), true)
		r.entries = append(r.entries, trpcProc{key: key, sub: sub, line: line})
	default:
		if kind := chainKind(value); kind != "" {
			r.entries = append(r.entries, trpcProc{key: key, kind: kind, line: line, end: end})
		} else if jsRef.MatchString(value) {
			r.entries = append(r.entries, trpcProc{key: key, ref: lastName(value), line: line})
		} else {
			r.incomplete = true
		}
	}
}

// splitKeyValue is keyValue plus where the value starts within e.
func splitKeyValue(e string) (key, value string, valueAt int, ok bool) {
	k, v, ok := keyValue(e)
	if !ok {
		return "", "", 0, false
	}
	colon := len(k)
	for colon < len(e) && e[colon] != ':' {
		colon++
	}
	at := colon + 1
	for at < len(e) && (e[at] == ' ' || e[at] == '\t' || e[at] == '\r' || e[at] == '\n') {
		at++
	}
	return k, v, at, true
}

type span struct {
	text string
	off  int
}

// spans splits s at its top-level commas, keeping where each part starts.
func spans(s string) []span {
	var out []span
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == ',' && depth == 0:
			out = append(out, span{s[start:i], start})
			start = i + 1
		}
	}
	return append(out, span{s[start:], start})
}

// keyValue splits `key: value` at the first colon outside brackets and strings.
func keyValue(e string) (key, value string, ok bool) {
	depth := 0
	var quote byte
	for i := 0; i < len(e); i++ {
		c := e[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == ':' && depth == 0:
			return strings.TrimSpace(e[:i]), strings.TrimSpace(e[i+1:]), true
		}
	}
	return "", "", false
}

// chainKind finds the final `.query(` / `.mutation(` / `.subscription(` of a
// procedure chain, outside the arguments of the calls before it
// (`.input(z.object({...}))` can contain anything).
func chainKind(v string) string {
	kind := ""
	depth := 0
	var quote byte
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == '.' && depth == 0:
			rest := strings.TrimLeft(v[i+1:], " \t\n\r")
			for _, k := range []string{"query", "mutation", "subscription"} {
				if strings.HasPrefix(rest, k) && strings.HasPrefix(strings.TrimLeft(rest[len(k):], " \t"), "(") {
					kind = k
				}
			}
		}
	}
	return kind
}

// readJSStatement is the expression starting at from: to the next semicolon, or
// the end of the line unless a bracket is open or the next line carries on with `.`.
func readJSStatement(src string, from int) string {
	depth := 0
	var quote byte
	for i := from; i < len(src); i++ {
		c := src[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == ';' && depth <= 0:
			return src[from:i]
		case c == '\n' && depth <= 0:
			if rest := strings.TrimLeft(src[i:], " \t\r\n"); !strings.HasPrefix(rest, ".") && !strings.HasPrefix(rest, "?.") {
				return src[from:i]
			}
		}
	}
	return src[from:]
}

// readAdapters finds where the server adapter is mounted and where the client
// links point: the base path every procedure's URL starts with.
func (tp *trpcProject) readAdapters(rel, src string, cm codeMap) {
	for _, m := range trpcAdapterNames.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		name := src[m[2]:m[3]]
		args := readArgsList(src, m[1]-1)
		if e := trpcEndpointKey.FindStringSubmatch(args); e != nil {
			tp.adapter = append(tp.adapter, strings.TrimRight(e[1], "/"))
			continue
		}
		switch name {
		case "fetchRequestHandler", "createNextApiHandler", "createNextRouteHandler":
			// no endpoint written: Next.js serves it where the file sits (…/trpc/[trpc])
			if p, ok := nextRoutePath(rel); ok {
				tp.adapter = append(tp.adapter, trimCatchAll(p))
			} else if p, _, ok := nextPagesPath(rel); ok {
				tp.adapter = append(tp.adapter, trimCatchAll(p))
			}
		case "createHTTPServer", "createHTTPHandler":
			tp.adapter = append(tp.adapter, "")
		}
	}
	for _, m := range trpcExpressUse.FindAllStringSubmatchIndex(src, -1) {
		if cm.codeAt(src, m[0]) {
			tp.adapter = append(tp.adapter, strings.TrimRight(src[m[2]:m[3]], "/"))
		}
	}
	for _, m := range trpcFastifyReg.FindAllStringSubmatchIndex(src, -1) {
		if cm.codeAt(src, m[0]) {
			tp.adapter = append(tp.adapter, strings.TrimRight(src[m[2]:m[3]], "/"))
		}
	}
	for _, m := range trpcLinkCall.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		if u := trpcLinkURL.FindStringSubmatch(readArgsList(src, m[1]-1)); u != nil {
			if p, ok := linkPath(u[1] + u[2] + u[3]); ok {
				tp.clientBases = append(tp.clientBases, p)
			}
		}
	}
}

// trimCatchAll turns /api/trpc/:trpc into /api/trpc.
func trimCatchAll(p string) string {
	segs := strings.Split(strings.Trim(p, "/"), "/")
	if n := len(segs); n > 0 && strings.HasPrefix(segs[n-1], ":") {
		segs = segs[:n-1]
	}
	return "/" + strings.Join(segs, "/")
}

var linkTail = regexp.MustCompile(`(/[\w\-./]*)$`)

// linkPath is the path part of a client link's url: "http://localhost:3000/trpc"
// and `${getBaseUrl()}/api/trpc` both end in the base path.
func linkPath(u string) (string, bool) {
	u = strings.TrimSpace(u)
	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			return strings.TrimRight(rest[j:], "/"), true
		}
		return "", true
	}
	if m := linkTail.FindStringSubmatch(u); m != nil && !strings.Contains(m[1], "${") {
		return strings.TrimRight(m[1], "/"), true
	}
	return "", false
}

// base is the path every procedure's URL starts with, and whether the code says
// so (an adapter or a client link). Without either, /api/trpc is assumed for
// both ends, so they still pair, and the routes are marked Unmounted.
func (tp *trpcProject) base() (string, bool) {
	if a := distinct(tp.adapter); len(a) >= 1 {
		return a[0], len(a) == 1
	}
	if c := distinct(tp.clientBases); len(c) >= 1 {
		return c[0], len(c) == 1
	}
	return "/api/trpc", false
}

// flat is one procedure with its dotted path.
type flat struct {
	path, kind, file string
	line, end        int
}

// apply adds the procedures as routes and the matching client calls as calls.
func (tp *trpcProject) apply(g *graph.Graph) {
	if len(tp.order) == 0 && len(tp.cands) == 0 {
		return
	}
	base, known := tp.base()

	// A router that no other router uses is a root.
	used := map[*trpcRouter]bool{}
	var mark func(r *trpcRouter, depth int)
	mark = func(r *trpcRouter, depth int) {
		if depth > 8 {
			return
		}
		visit := func(p trpcProc) {
			if p.sub != nil {
				used[p.sub] = true
				mark(p.sub, depth+1)
			}
			if p.ref != "" {
				if t, ok := tp.unique(p.ref); ok {
					used[t] = true
					mark(t, depth+1)
				}
			}
		}
		for _, e := range r.entries {
			visit(e)
		}
		for _, e := range r.merges {
			visit(e)
		}
	}
	for _, r := range tp.order {
		mark(r, 0)
	}

	var procs []flat
	incomplete := false
	var walk func(r *trpcRouter, prefix string, depth int)
	walk = func(r *trpcRouter, prefix string, depth int) {
		if depth > 8 {
			incomplete = true
			return
		}
		if r.incomplete {
			incomplete = true
		}
		for _, e := range r.entries {
			p := prefix + e.key
			switch {
			case e.sub != nil:
				walk(e.sub, p+".", depth+1)
			case e.kind != "":
				procs = append(procs, flat{p, e.kind, r.file, e.line, e.end})
			case e.ref != "":
				if t, ok := tp.unique(e.ref); ok {
					walk(t, p+".", depth+1)
				} else if kinds := tp.procConst[e.ref]; len(kinds) == 1 {
					procs = append(procs, flat{p, kinds[0], r.file, e.line, 0})
				} else if len(kinds) > 1 {
					procs = append(procs, flat{p, "", r.file, e.line, 0}) // a procedure of unknown kind
				} else {
					incomplete = true // a router or procedure defined where malveon can't read it
				}
			}
		}
		for _, mg := range r.merges {
			if mg.sub != nil {
				walk(mg.sub, prefix, depth+1)
			} else if t, ok := tp.unique(mg.ref); ok {
				walk(t, prefix, depth+1)
			} else {
				incomplete = true
			}
		}
	}
	for _, r := range tp.order {
		if !used[r] && r.name != "" {
			walk(r, "", 0)
		}
	}

	byPath := map[string]flat{}
	for _, p := range procs {
		if _, dup := byPath[p.path]; !dup {
			byPath[p.path] = p
		}
	}
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		pr := byPath[p]
		n := graph.Node{
			Kind: graph.RouteHandler, File: pr.file, Line: pr.line, Method: kindMethod(pr.kind),
			Path: base + "/" + p, Confidence: graph.Extracted, RPC: true, Unmounted: !known,
		}
		n.Words = wordsFor(pr.file, n.Path)
		if pr.end > 0 {
			// The procedure's own code is its handler: what it changes is what its lines
			// change, not what the other procedures in the router file do.
			n.EnclosingFunc = p
			if g.Defs == nil {
				g.Defs = map[string][]graph.Loc{}
			}
			g.Defs[p] = append(g.Defs[p], graph.Loc{File: pr.file, Line: pr.line, End: pr.end})
		}
		g.Nodes = append(g.Nodes, n)
	}
	if incomplete && len(tp.order) > 0 {
		// Some procedures live where malveon can't read: say so, so that "no such
		// procedure" is never claimed.
		g.Nodes = append(g.Nodes, graph.Node{Kind: graph.RouteHandler, File: tp.order[0].file, Line: tp.order[0].line, Confidence: graph.Inferred, RPC: true})
	}

	for _, c := range tp.cands {
		pr, ok := byPath[c.path]
		if !ok || (!hookVerbs[c.verb] && !tp.clients[c.root]) {
			continue
		}
		if c.direct && !tp.clients[c.root] {
			continue
		}
		method := pr.kind
		switch {
		case c.direct:
			method = kindMethod(pr.kind)
		case verbMethod[c.verb] == "POST":
			method = "POST"
		default:
			method = "GET"
		}
		n := graph.Node{
			Kind: graph.NetworkCall, File: c.file, Line: c.line, Method: method,
			Path: base + "/" + c.path, Confidence: graph.Extracted, RPC: true,
		}
		n.Words = wordsFor(c.file, n.Path)
		g.Nodes = append(g.Nodes, n)
	}
}

// unique is the one router a name refers to; a name defined more than once is ambiguous.
func (tp *trpcProject) unique(name string) (*trpcRouter, bool) {
	rs := tp.routers[name]
	if len(rs) != 1 {
		return nil, false
	}
	return rs[0], true
}

func kindMethod(kind string) string {
	switch kind {
	case "query", "subscription":
		return "GET"
	case "mutation":
		return "POST"
	}
	return "" // a procedure whose kind couldn't be read answers either
}
