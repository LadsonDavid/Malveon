// Package graph holds the in-memory representation every check reads from.
// Nothing outside this package should know how nodes were extracted — it
// only sees Nodes and asks Find/PathMatch questions.
package graph

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Confidence records how sure the extractor is about a node's path.
// EXTRACTED means the path was read directly from a string literal in
// source. INFERRED means the extractor found the call/route but the path
// wasn't a plain literal (built from a variable, template, config, etc.) —
// never treat an Inferred node as proof of a match.
type Confidence int

const (
	Extracted Confidence = iota
	Inferred
)

func (c Confidence) String() string {
	if c == Extracted {
		return "EXTRACTED"
	}
	return "INFERRED"
}

// Kind is which side of a wire a node sits on.
type Kind int

const (
	RouteHandler Kind = iota // backend: something that registers a path
	NetworkCall              // frontend: something that calls a path
	Page                     // frontend: a page file served at Path (Next.js App Router page.tsx)
	DataCall                 // app code talking to the database directly (Supabase .from/.rpc); Path is the table or function name
)

// Loc is one place in the code: a file (root-relative) and a line.
// End is the last line of a function definition when known, else 0.
type Loc struct {
	File string
	Line int
	End  int
}

// Node is one extracted call or route registration.
type Node struct {
	Kind          Kind
	File          string
	Line          int
	Method        string     // HTTP verb if known, "" if not applicable/unknown
	Path          string     // the literal path string, "" if not resolvable statically
	Confidence    Confidence // Extracted only when Path is a real literal
	Words         []string   // tokens used for loose feature-name matching
	EnclosingFunc string     // name of the innermost named function this node's line falls inside; "" if top-level or unknown
	BodyFields    []string   // request fields sent (NetworkCall) or read via req.body (RouteHandler); nil if unresolved, non-nil (possibly empty) if resolved
	Receiver      string     // Python route: the router variable it hangs on (bp, router), for following where that router is mounted
	Stub          bool       // RouteHandler whose handler is short and only answers with an error (500/501, "not implemented")
	Unreferenced  bool       // NetworkCall inside a function nothing in the project calls: not a call anyone makes
	InTest        bool       // NetworkCall in a test file (foo.test.ts, __tests__/, *_test.go, test_*.py, cypress/, e2e/): the tests call it, the app doesn't
	Unmounted     bool       // RouteHandler in a router file nothing was seen mounting: its real URL is unknown, so it isn't compared with other files' routes
	Op            string     // DataCall only: the method chained right after .from(...) ("select", "insert", "upload"), or the auth method; "" if not chained directly
	FileRouted    bool       // the URL comes from where the file sits (Next.js, Nuxt, SvelteKit, Remix, Astro), and the framework picks a static segment over a dynamic one whatever the order
	RPC           bool       // a tRPC procedure or a call to one: Path is the wire path (/api/trpc/post.create), not a URL anyone wrote
}

// Import is one thing a file imports from another file in the project
// (never a package). Name is the imported name as defined in File, or ""
// for the whole file (a default, namespace, or side-effect import). A
// project import (./x, @/x) that matched no scanned file has File "" and
// the module text in Missing — code malveon can't read.
type Import struct {
	Name    string
	File    string
	Missing string
	Spec    string // the module text as written: './lib/format'
	Line    int    // the line of the import statement
}

// ExportSet is what a JavaScript or TypeScript file exports. Open means the names
// cannot be known (a re-export of everything, CommonJS, a destructured export): an
// open file has every name.
type ExportSet struct {
	Names map[string]bool
	Open  bool
}

// BrokenImport is an import that cannot work: a named import the target file no
// longer exports, or a relative import of a file that does not exist.
type BrokenImport struct {
	File    string // the importing file
	Line    int
	Name    string // the name imported ("" when the whole file is missing)
	Spec    string // the module text as written
	Target  string // the file imported from ("" when it is missing)
	Missing bool   // the imported file does not exist
}

// BrokenImports lists, in file and line order, the imports that cannot work. It claims
// nothing it cannot read: a file whose exports are unknowable, an import of a
// non-JavaScript file, a package or an alias path are never reported.
func (g *Graph) BrokenImports() []BrokenImport {
	var files []string
	for f := range g.Imports {
		files = append(files, f)
	}
	sort.Strings(files)
	var out []BrokenImport
	seen := map[string]bool{}
	add := func(b BrokenImport) {
		key := fmt.Sprintf("%s|%s|%s|%v", b.File, b.Name, b.Spec, b.Missing)
		if !seen[key] {
			seen[key] = true
			out = append(out, b)
		}
	}
	for _, file := range files {
		for _, im := range g.Imports[file] {
			switch {
			case im.Missing != "":
				if strings.HasPrefix(im.Missing, ".") && jsLikeSpec(im.Missing) {
					add(BrokenImport{File: file, Line: im.Line, Spec: im.Missing, Missing: true})
				}
			case im.Name != "" && im.File != "":
				if ex, ok := g.Exports[im.File]; ok && !ex.Open && !ex.Names[im.Name] {
					add(BrokenImport{File: file, Line: im.Line, Name: im.Name, Spec: im.Spec, Target: im.File})
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// jsLikeSpec: a module path with no extension or a JavaScript one. './data.json' and
// './x.module.css' are assets, not code malveon reads.
func jsLikeSpec(spec string) bool {
	switch strings.ToLower(path.Ext(spec)) {
	case "", ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
		return true
	}
	return false
}

// HandlerKind says what is attached to a trigger.
type HandlerKind string

const (
	HandlerNone      HandlerKind = "none"      // nothing: clicking it does nothing
	HandlerInline    HandlerKind = "inline"    // a function written in place: onClick={() => save()}
	HandlerNamed     HandlerKind = "named"     // a name or a member: onClick={save}, onClick={api.save}
	HandlerForm      HandlerKind = "form"      // a button that submits the form it sits in; the form's own trigger holds the handler
	HandlerNavigates HandlerKind = "navigates" // a link or a form with a URL: it goes somewhere and calls no function
	HandlerUnknown   HandlerKind = "unknown"   // spread props, a framework attribute (hx-post, @click), an expression malveon can't read
)

// Handler is what a trigger runs. Mentions are the identifiers its expression
// uses, whatever its kind, so "does this reach that function" can be answered
// from names alone.
type Handler struct {
	Kind     HandlerKind
	Name     string   // Named: the name, or the last part of a member (api.save -> save)
	Mentions []string // identifiers the expression uses, sorted, without keywords
	From, To int      // Inline: first and last line of the function written in place
}

// Trigger is something on a screen a person acts on: a button, a form, a link,
// or any element given an event handler (onClick, onSubmit, ...). One element
// with two handlers makes two triggers.
type Trigger struct {
	File    string
	Line    int
	Tag     string // as written: button, form, a, Link, Modal
	Label   string // the visible text, when it is plain text
	Event   string // onClick, onSubmit, action; "" when none is attached
	Handler Handler
}

// Graph is the flat set of nodes a run produced, plus the plain facts a
// plan line can be checked against directly: which files exist, and
// which names (functions, classes, components) are defined where.
type Graph struct {
	Nodes   []Node
	Files   []string                   // every scanned file, root-relative (same form as Node.File)
	Defs    map[string][]Loc           // top-level names defined in code -> where
	Names   map[string]map[string]bool // file -> every identifier-shaped word in it (code and strings)
	Tables  map[string]string          // database tables/views the project declares (lowercase name -> the .sql/.prisma file); nil when it declares none
	Imports map[string][]Import        // file -> what it imports from other project files (JS/TS only)
	Exports map[string]ExportSet       // file -> what it exports (JS/TS only); a file not listed is unknown
	// TableAlias maps a second name for a table to the one its calls use, both
	// lowercase: a Prisma @@map("user_profiles") to the model UserProfile, a Drizzle
	// variable `users` to the table "user". A plan line may say either.
	TableAlias map[string]string

	// Triggers are the buttons, forms, links and handlers on screens (JSX only for now).
	Triggers []Trigger
	// OpaqueWiring: JSX files with an element wired in a way malveon cannot read (spread
	// props, a framework attribute). A button with no handler of its own in such a file
	// may still be handled.
	OpaqueWiring map[string]bool
	// Uses counts, for every function or component the project defines, the places
	// its name is used: anything but its own declaration, an import of it, or an
	// export list. 0 means nothing in the project (templates included) ever uses it.
	// Names in comments and strings count, so this errs toward "used".
	Uses map[string]int

	commonWords map[string]bool      // lazily computed, see commonWordSet
	mentioned   map[string]bool      // lazily computed, see MentionsWord
	importers   map[string][]string  // lazily computed, see ImportersOf
	triggersIn  map[string][]Trigger // lazily computed, see TriggersNear
}

// TriggersNear returns the triggers that could be what runs something in file:
// the ones in file itself first, then those in the files that import it (a
// parent handing a handler down), then in the files that import those. A
// trigger anywhere else belongs to another screen, even when it names a
// function the same way.
func (g *Graph) TriggersNear(file string) []Trigger {
	if g.triggersIn == nil {
		g.triggersIn = map[string][]Trigger{}
		for _, t := range g.Triggers {
			g.triggersIn[t.File] = append(g.triggersIn[t.File], t)
		}
	}
	seen := map[string]bool{file: true}
	level := []string{file}
	out := append([]Trigger(nil), g.triggersIn[file]...)
	for depth := 0; depth < 2 && len(level) > 0; depth++ {
		var next []string
		for _, f := range level {
			for _, imp := range g.ImportersOf(f) {
				if !seen[imp] {
					seen[imp] = true
					next = append(next, imp)
					out = append(out, g.triggersIn[imp]...)
				}
			}
		}
		level = next
	}
	return out
}

// ImportersOf returns every file that imports file.
func (g *Graph) ImportersOf(file string) []string {
	if g.importers == nil {
		g.importers = map[string][]string{}
		for from, imps := range g.Imports {
			seen := map[string]bool{}
			for _, im := range imps {
				if im.File != "" && !seen[im.File] {
					seen[im.File] = true
					g.importers[im.File] = append(g.importers[im.File], from)
				}
			}
		}
		for _, fs := range g.importers {
			sort.Strings(fs)
		}
	}
	return g.importers[file]
}

// commonWordMinCount and commonWordFraction jointly decide when a word
// has stopped meaning anything in *this* codebase. Chosen from real data
// (2026-09-16, a 292-node real project): "api" sat at 140 nodes (47.9%)
// and "admin" at 19 (6.5%), both causing real false-positive matches
// (a "Circles" feature matching an unrelated "announcements" route
// because both happened to be under /admin/*); a genuinely specific
// feature word like "circles" itself sat at 10 nodes (3.4%). The
// min-count floor exists specifically so this never fires on a small
// graph (a test fixture, a young project) where a legitimate shared
// word between a real call/route pair can easily be 100% of a tiny
// node count — the fraction alone can't tell "too common" apart from
// "this pair is exactly what it looks like" without enough nodes to
// make the statistic mean something.
const (
	commonWordMinCount = 15
	commonWordFraction = 0.05
)

// stopWords are ordinary English function words that carry no domain
// meaning at all — unlike commonWordFraction/commonWordMinCount (which
// catch a word specific to *this* codebase becoming too common to mean
// anything), these can coincidentally collide with a URL path fragment
// regardless of how rare that collision is. Confirmed 2026-09-16: a
// feature description starting "All 6 Settings tabs..." matched an
// unrelated /api/notifications/read-all route — "all" appeared on only
// one node in the whole graph, far too rare to trip the frequency
// filter, but it's still not a real signal. A small, fixed, standard
// stopword list — not per-codebase, never needs tuning — closes that
// gap the frequency filter structurally can't.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true,
	"and": true, "or": true, "of": true, "to": true, "in": true, "on": true,
	"for": true, "with": true, "from": true, "by": true, "at": true,
	"is": true, "are": true, "be": true, "as": true, "it": true,
	"this": true, "that": true, "all": true, "any": true,
	"your": true, "you": true, "we": true, "our": true,
}

// IsStopWord reports whether w (lowercase) is one of stopWords.
func IsStopWord(w string) bool { return stopWords[w] }

// commonWordSet returns the words that carry no identifying signal in
// this graph — either because they're generic English (stopWords) or
// because they're specific to this codebase but appear in too many
// nodes to mean anything (commonWordFraction/commonWordMinCount).
// Computed once and cached — every check that resolves features against
// this graph calls FindNodesMatching repeatedly, and the set doesn't
// change mid-run.
func (g *Graph) commonWordSet() map[string]bool {
	if g.commonWords != nil {
		return g.commonWords
	}
	counts := map[string]int{}
	for _, n := range g.Nodes {
		for _, w := range dedupe(normalizeWords(n.Words)) {
			counts[w]++
		}
	}
	floor := float64(len(g.Nodes)) * commonWordFraction
	common := map[string]bool{}
	for w := range stopWords {
		common[w] = true
	}
	for w, c := range counts {
		if c > commonWordMinCount && float64(c) > floor {
			common[w] = true
		}
	}
	g.commonWords = common
	return common
}

func dedupe(words []string) []string {
	seen := make(map[string]bool, len(words))
	out := make([]string, 0, len(words))
	for _, w := range words {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

// ContentWords normalizes words and drops the ones that carry no
// identifying signal in this graph (see commonWordSet) — what's left is
// what a plan line and a code node can meaningfully share.
func (g *Graph) ContentWords(words []string) []string {
	return excludeCommon(dedupe(normalizeWords(words)), g.commonWordSet())
}

func excludeCommon(words []string, common map[string]bool) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		if !common[w] {
			out = append(out, w)
		}
	}
	return out
}

func normalizeWords(words []string) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" {
			continue
		}
		out = append(out, w)
	}
	return out
}

// Pair is one network call and one route handler whose literal paths
// agree — a connection the wiring check proved exists.
type Pair struct {
	Call  Node
	Route Node
}

// RoutesServing returns every literal route whose path serves path.
func (g *Graph) RoutesServing(path string) []Node {
	routes := g.literalNodes(RouteHandler, func(n Node) bool {
		return PathsMatch(path, n.Path) || (n.Unmounted && endsWith(path, n.Path))
	})
	// A tRPC procedure is served by its own route; the adapter's catch-all
	// (/api/trpc/:trpc) only carries it. Left in, the catch-all accepts any method
	// and hides a mutation called as a query.
	var procs []Node
	for _, n := range routes {
		if n.RPC {
			procs = append(procs, n)
		}
	}
	if len(procs) > 0 {
		routes = procs
	}
	// The most specific route first, as every file-based router picks it.
	sort.SliceStable(routes, func(i, j int) bool { return wildcardCount(routes[i].Path) < wildcardCount(routes[j].Path) })
	return routes
}

func wildcardCount(path string) int {
	n := 0
	for _, seg := range splitPath(path) {
		if IsWildcardSegment(seg) {
			n++
		}
	}
	return n
}

// CallsToRoute returns every literal call to route r. A route of a router
// whose mount wasn't followed (Unmounted) is also reached by a call that ends
// with its path: /items is probably /api/items under a prefix malveon didn't see.
func (g *Graph) CallsToRoute(r Node) []Node {
	return g.literalNodes(NetworkCall, func(n Node) bool {
		return PathsMatch(n.Path, r.Path) || (r.Unmounted && endsWith(n.Path, r.Path))
	})
}

// endsWith: path's last segments are suffix (wildcards match anything).
func endsWith(path, suffix string) bool {
	a, b := splitPath(path), splitPath(suffix)
	if len(b) == 0 || len(b) > len(a) {
		return false
	}
	return PathsMatch("/"+strings.Join(a[len(a)-len(b):], "/"), suffix)
}

// CallsTo returns every literal network call whose path is path.
func (g *Graph) CallsTo(path string) []Node {
	return g.literalNodes(NetworkCall, func(n Node) bool { return PathsMatch(n.Path, path) })
}

// RoutesUnder returns every literal route nested below prefix — a plan
// line naming "/api/upload" usually means the group of routes under it
// (/api/upload/sign, ...), not one route at exactly that path. A literal
// prefix segment must match literally: a route's own wildcard
// (/api/notifications/:id/read) is not "under" /api/notifications/read-all.
func (g *Graph) RoutesUnder(prefix string) []Node {
	pre := splitPath(prefix)
	return g.literalNodes(RouteHandler, func(n Node) bool {
		segs := splitPath(n.Path)
		if len(segs) <= len(pre) {
			return false
		}
		for i, p := range pre {
			if !IsWildcardSegment(p) && segs[i] != p {
				return false
			}
		}
		return true
	})
}

// PagesAt returns the page file(s) served at path.
func (g *Graph) PagesAt(path string) []Node {
	return g.literalNodes(Page, func(n Node) bool { return PathsMatch(path, n.Path) })
}

// DataCallsTo returns every direct database call naming table/function. Names
// compare ignoring case and through TableAlias: Prisma models are PascalCase in
// the schema and a plan line may write the table name instead.
func (g *Graph) DataCallsTo(name string) []Node {
	want := g.canonTable(name)
	return g.literalNodes(DataCall, func(n Node) bool { return n.Path == name || g.canonTable(n.Path) == want })
}

func (g *Graph) canonTable(name string) string {
	l := strings.ToLower(name)
	if a, ok := g.TableAlias[l]; ok {
		return a
	}
	return l
}

// NameIn reports whether name appears in file's code at all.
func (g *Graph) NameIn(file, name string) bool { return g.Names[file][name] }

// NameAnywhere reports whether name appears in any scanned file.
func (g *Graph) NameAnywhere(name string) bool {
	for _, names := range g.Names {
		if names[name] {
			return true
		}
	}
	return false
}

// MentionsWord reports whether any code mentions w (lowercase): a word of
// any identifier in any file (split at camelCase and snake_case, so
// "RefundButton" mentions refund and button) or of any file path, singular
// or plural. It answers "is there anything like this at all", so a plan
// line nothing resembles can be called NOT BUILT instead of "rewrite".
func (g *Graph) MentionsWord(w string) bool {
	if g.mentioned == nil {
		g.mentioned = map[string]bool{}
		for _, names := range g.Names {
			for id := range names {
				for _, s := range SplitWords(id) {
					g.mentioned[s] = true
				}
			}
		}
		for _, f := range g.Files {
			for _, s := range SplitWords(f) {
				g.mentioned[s] = true
			}
		}
	}
	return g.mentioned[w] || g.mentioned[w+"s"] || g.mentioned[strings.TrimSuffix(w, "s")] || g.mentioned[strings.TrimSuffix(w, "es")]
}

// HasFrontend reports whether the project has any screen-side code malveon
// sees: a call to a server path (resolved or not) or a page. A backend-only
// API has none, so an uncalled route there isn't missing its caller. A call in
// a test file isn't screen-side code: a backend whose tests fetch its routes
// still has no screens.
func (g *Graph) HasFrontend() bool {
	for _, n := range g.Nodes {
		if (n.Kind == NetworkCall && !n.InTest) || n.Kind == Page {
			return true
		}
	}
	return false
}

// NodesInFile returns every node extracted from file.
func (g *Graph) NodesInFile(file string) []Node {
	var out []Node
	for _, n := range g.Nodes {
		if n.File == file {
			out = append(out, n)
		}
	}
	return out
}

// FindFiles returns the scanned files a plan's file reference points at:
// a reference containing "/" matches by path suffix
// ("src/components/Footer.tsx"), a bare name matches by file name
// ("Footer.tsx").
func (g *Graph) FindFiles(ref string) []string {
	ref = strings.TrimPrefix(strings.ReplaceAll(ref, "\\", "/"), "./")
	var out []string
	for _, f := range g.Files {
		slash := strings.ReplaceAll(f, "\\", "/")
		if slash == ref || strings.HasSuffix(slash, "/"+ref) {
			out = append(out, f)
		}
	}
	return out
}

// UnreadableCalls returns the calls whose URL malveon could not read (built
// from a variable or an expression): any of them may be the caller of a
// route no readable call reaches.
func (g *Graph) UnreadableCalls() []Node {
	var out []Node
	for _, n := range g.Nodes {
		if n.Kind == NetworkCall && n.Confidence == Inferred && !n.Unreferenced && !n.InTest {
			out = append(out, n)
		}
	}
	return out
}

// IgnoredCallsTo returns the calls to path that are not counted because they
// sit in a function nothing in the project calls (Node.Unreferenced).
func (g *Graph) IgnoredCallsTo(path string) []Node {
	var out []Node
	for _, n := range g.Nodes {
		if n.Kind == NetworkCall && n.Unreferenced && !n.InTest && n.Confidence == Extracted && PathsMatch(n.Path, path) {
			out = append(out, n)
		}
	}
	return out
}

// TestCallsTo returns the calls to path made from test files (Node.InTest):
// never counted as the app calling a route, but worth naming when a route has
// no other caller.
func (g *Graph) TestCallsTo(path string) []Node {
	var out []Node
	for _, n := range g.Nodes {
		if n.Kind == NetworkCall && n.InTest && n.Confidence == Extracted && PathsMatch(n.Path, path) {
			out = append(out, n)
		}
	}
	return out
}

func (g *Graph) literalNodes(kind Kind, keep func(Node) bool) []Node {
	var out []Node
	for _, n := range g.Nodes {
		if kind == NetworkCall && (n.Unreferenced || n.InTest) {
			continue // a call in dead code, or in a test, is not a call the app makes
		}
		if n.Kind == kind && n.Confidence == Extracted && keep(n) {
			out = append(out, n)
		}
	}
	return out
}

// MethodsCompatible reports whether a call's HTTP method is one the route
// accepts. An unknown method on either side can't contradict anything.
func MethodsCompatible(call, route Node) bool {
	return call.Method == "" || route.Method == "" || call.Method == route.Method
}

// PathsMatch compares a network-call path against a route-handler path,
// treating a segment like :id or {id} as a wildcard so "/refund/:id" and
// "/refund/123" count as the same route.
func PathsMatch(callPath, routePath string) bool {
	a := splitPath(callPath)
	b := splitPath(routePath)
	// A catch-all segment (:...slug, from [...slug]) takes one or more
	// segments, two or more for none-or-more (:...slug?).
	if k, optional := catchAll(b); k >= 0 {
		return matchesBefore(a, b[:k], optional)
	}
	if k, optional := catchAll(a); k >= 0 {
		return matchesBefore(b, a[:k], optional)
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if IsWildcardSegment(b[i]) || IsWildcardSegment(a[i]) {
			continue
		}
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// catchAll finds a catch-all segment (":...slug" or ":...slug?"): its index,
// and whether it also matches zero segments. -1 when there is none.
func catchAll(segs []string) (int, bool) {
	for i, s := range segs {
		if strings.HasPrefix(s, ":...") {
			return i, strings.HasSuffix(s, "?")
		}
	}
	return -1, false
}

// matchesBefore: path starts with the segments in prefix (wildcards allowed)
// and has something left over for the catch-all — unless it may be empty.
func matchesBefore(path, prefix []string, optional bool) bool {
	need := len(prefix) + 1
	if optional {
		need = len(prefix)
	}
	if len(path) < need {
		return false
	}
	for i, p := range prefix {
		if IsWildcardSegment(p) || IsWildcardSegment(path[i]) {
			continue
		}
		if p != path[i] {
			return false
		}
	}
	return true
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return []string{}
	}
	return strings.Split(p, "/")
}

func IsWildcardSegment(seg string) bool {
	if seg == "" {
		return false
	}
	if strings.HasPrefix(seg, ":") {
		return true
	}
	if strings.HasSuffix(seg, "}") && strings.Contains(seg, "{") {
		return true // {id}, and @{username}: text before a template hole
	}
	// Flask/Django: <id>, <int:id>. Without this a Flask route like
	// /api/users/<int:id> was a literal path no real call could match.
	return strings.HasPrefix(seg, "<") && strings.HasSuffix(seg, ">")
}

// IsFullyStatic reports whether path has no wildcard segments at all —
// used by the overlap check to tell a genuinely static route ("/users/
// mine") apart from a parameterized one ("/users/:id") that happens to
// match it, since several routers (see internal/checks/overlap's
// reachability/ordering notes) resolve the two very differently.
func IsFullyStatic(path string) bool {
	for _, seg := range splitPath(path) {
		if IsWildcardSegment(seg) {
			return false
		}
	}
	return true
}

// SplitWords is the one tokenizer for both sides of matching: plan text
// and code nodes (file names, path segments). It breaks camelCase
// ("PostJobModal" -> post job modal) and splits on every non-alphanumeric
// run, so "(`/api/jobs`)" yields "api", "jobs". Before 2026-09-23 plan
// text was split on whitespace only, so any word touching a backtick,
// bracket, "**", colon or "/" could never match anything.
func SplitWords(s string) []string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && isUpper(r) && !isUpper(runes[i-1]) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	fields := strings.FieldsFunc(b.String(), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, strings.ToLower(f))
	}
	return out
}

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }

// NearTable returns the declared table a name is probably a typo of (edit
// distance 1 or 2, never the same name), or "" when there is no schema or
// nothing is close. Only a near miss is reported: a project's schema may
// live partly outside the files malveon reads, so an unknown name alone
// proves nothing.
func (g *Graph) NearTable(name string) string {
	if len(g.Tables) == 0 || len(name) < 4 {
		return ""
	}
	name = strings.ToLower(name)
	if _, known := g.Tables[name]; known {
		return ""
	}
	best, bestD := "", 3
	for t := range g.Tables {
		if d := EditDistance(name, t); d < bestD || (d == bestD && t < best) {
			best, bestD = t, d
		}
	}
	return best
}

// EditDistance is the Levenshtein distance between a and b.
func EditDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
