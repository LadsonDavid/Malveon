// Package wiring gives every plan line one direct answer against the
// real code: CONNECTED, BROKEN, NOT BUILT, HALF BUILT, or SKIPPED. See
// CLAUDE.md section 3.2.1 for the spec this implements — this file should
// never drift from it without both being updated together.
//
// Redesigned 2026-09-23. The earlier version answered most lines with
// "NO PROOF — nothing solid either way", and it matched a plan line to
// code by any single shared word, so a coincidence ("...the two sets
// read as one..." vs /api/notifications/read-all) could pass as proof.
// Now the things a plan line names — URLs, file names, and code names in
// backticks — are looked up directly, and each lookup ends in a plain
// fact ("/api/search isn't served or called anywhere"). Word matching is
// only a fallback for lines that name nothing, and needs a strong match.
//
// A line that names nothing and matches nothing is SKIPPED with Unchecked
// set: malveon could not look it up. It used to be its own verdict, REWRITE
// THIS LINE, and blocked the gate; on real plans most of those lines were
// sub-bullets, field lists and notes, so the report led with the tool's own
// limit instead of with problems in the code (founder, 2026-10-15: "looks
// like noise"). What it says about the code is nothing, so it is not a
// verdict, and only a plan where nothing at all could be checked blocks.
package wiring

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/features"
	"github.com/LadsonDavid/Malveon/pkg/graph"
)

type Verdict string

const (
	Working   Verdict = "CONNECTED"
	Broken    Verdict = "BROKEN"
	NotBuilt  Verdict = "NOT BUILT"
	HalfBuilt Verdict = "HALF BUILT"
	Skipped   Verdict = "SKIPPED"
)

// Why a line malveon could not look up is SKIPPED. Reports group by these.
const (
	ReasonNamesNothing  = "names no URL, file or code name, and no code clearly matches its words"
	ReasonRemovalNoFile = "a removal can only be checked for a named file — say which file should be gone"
)

type Result struct {
	Citations          []string `json:"citations,omitempty"`
	CitationValidation string   `json:"citation_validation,omitempty"`
	FeatureID          string
	FeatureName        string
	Source             string // where the plan line came from ("docs/PLAN.md:12"), see features.Feature
	// Unticked: the plan line is a "- [ ]" box, work not claimed as done
	// yet. The verdict is still the true one; only the gate treats it
	// differently (blocks on BROKEN only). See features.Feature.Unticked.
	Unticked bool `json:",omitempty"`
	Verdict  Verdict
	Reason   string
	Evidence []string // "file:line" entries backing the verdict
	// Path is the flow traced end to end through the code for a CONNECTED
	// line — Screen, Action, Server, State (see path.go). Empty otherwise.
	Path []string `json:",omitempty"`

	// Unchecked marks a SKIPPED line that malveon could not look up (it names
	// nothing checkable, or the code can't say), as opposed to one that isn't a
	// code task at all (a heading, a design note, a build step). The first kind
	// is something the writer can fix by naming a URL, file or component.
	Unchecked bool `json:",omitempty"`

	// ExistsButNoCall: HALF BUILT only because the thing the line names exists and makes
	// no server or database call while the line promised a connection. A plan line makes
	// that promise; prose such as an agent's notes does not, so a reader of prose ignores
	// this verdict.
	ExistsButNoCall bool `json:"-"`

	// Closest, for an Unchecked line only: the routes and pages in the code
	// that share a word with the line, so the writer knows what to name
	// (hints.go). Never changes the verdict.
	Closest       []string `json:",omitempty"`
	EvidenceKind  string   `json:"evidence_kind,omitempty"`
	CoverageLimit string   `json:"coverage_limit,omitempty"`

	// Pairs are the frontend-call -> server-route connections this line's
	// verdict rests on — what the contract check compares method and
	// fields for. Internal working data, not part of the JSON report.
	Pairs []graph.Pair `json:"-"`
}

// Run evaluates every feature against the graph and returns one Result
// per feature, in the same order features were given.
func Run(g *graph.Graph, fs []features.Feature) []Result {
	return RunWithChanges(g, fs, nil)
}

// RunWithChanges is Run plus the files that changed since the session
// started (session.ChangedFiles). A line that names only files and says they
// will gain something ("`PostCard.tsx` adds a report option") is judged
// against it: an unchanged file can't have gained it. nil means malveon
// doesn't know what changed (no session, not a git repo, git failed) and the
// line is judged as in Run — never as "unchanged". An empty non-nil set
// means nothing changed.
func RunWithChanges(g *graph.Graph, fs []features.Feature, changed map[string]bool) []Result {
	results := make([]Result, 0, len(fs))
	for _, f := range fs {
		results = append(results, evaluate(g, f, changed))
	}
	return results
}

// span is a piece of code a plan line pointed at: a whole file, or one
// function's lines within it (to == 0 means the whole file).
type span struct {
	file     string
	from, to int
	label    string // how the reason text names it: "`InvitesTab`", "page /circles"
	name     string // a code name defined in this span (`Domain`), when the line named one

	// A server route the line named: its URL and method, so the check can ask
	// whether a component actually calls it.
	route  bool
	url    string
	method string
}

func (s span) contains(n graph.Node) bool {
	return n.File == s.file && (s.to == 0 || (n.Line >= s.from && n.Line <= s.to))
}

// finding is what one lookup (one URL, file, or name the line mentions)
// established. exists-only findings ("the component is there, but its
// code makes no calls") don't decide a verdict alone — see combine.
type finding struct {
	verdict    Verdict // Working, Broken, NotBuilt, HalfBuilt, Skipped (can't tell); "" for exists-only
	reason     string
	evidence   []string
	pairs      []graph.Pair
	spans      []span // code this finding resolved to, for checking table/function names against
	existsOnly bool
	use        usage // exists-only findings: does anything in the app use this file
}

var removalPattern = regexp.MustCompile(`(?i)(?:^[^A-Za-z]*|[—–:]\s*)(?:remove|delete|drop|get rid of)\b`)

func evaluate(g *graph.Graph, f features.Feature, changed map[string]bool) Result {
	base := Result{FeatureID: f.ID, FeatureName: f.Name, Source: f.Source, Unticked: f.Unticked}
	base.Citations = f.Citations
	if len(f.Citations) > 0 {
		base.CitationValidation = "declared Source-column context; external target and attached claim have not been verified"
	}

	if skip, why := features.Classify(f); skip {
		base.Verdict, base.Reason = Skipped, why
		return base
	}

	// "Remove X" is a removal task — unless the line names an HTTP method
	// with its URL ("Delete jobs via `DELETE /api/jobs`"): that's an API
	// being built, not something being taken out.
	removal := removalPattern.MatchString(f.Name)
	for _, u := range features.PlanURLs(f.Name) {
		if u.Method != "" {
			removal = false
		}
	}
	anchorText := f.Name
	for _, citation := range f.Citations {
		anchorText = strings.Replace(anchorText, citation, "", 1)
	}
	anchors := anchorsIn(anchorText)
	// A line whose only anchors are files, saying one will gain something, is
	// judged by whether the file changed this session. A URL or code name in
	// the line decides it as always; so does a removal or a line that only
	// creates the file (the verb comes before the file's name).
	promise := ""
	if changed != nil && !removal && onlyFiles(anchors) {
		var refs []string
		for _, a := range anchors {
			refs = append(refs, a.text)
		}
		promise = promisedChange(f.Name, refs)
	}
	var found []finding
	var dataNames []string
	for _, a := range anchors {
		if removal && a.kind != fileAnchor {
			continue // only a named file can be checked for "is it gone"
		}
		switch a.kind {
		case pathAnchor:
			if fd, ok := checkPath(g, a, !features.ExpectsOutsideCaller(f)); ok {
				found = append(found, fd)
			}
		case fileAnchor:
			fd := checkFile(g, a.text, removal)
			if promise != "" {
				fd = sessionChange(g, fd, a.text, promise, changed, f.InSession)
			}
			found = append(found, fd)
		case nameAnchor:
			// A tRPC procedure is named `post.create`; it is a route at its wire URL.
			if fd, ok := checkRPCName(g, a.text, f); ok {
				found = append(found, fd)
				continue
			}
			// A component or function with this name is what the line means, even
			// when a table has the same name (`Post` is a component and a Prisma model).
			if fd, ok := checkNameDefined(g, a.text); ok {
				found = append(found, fd)
			} else if dataName(g, a.text) != "" {
				dataNames = append(dataNames, a.text) // checked below, against what the rest of the line points at
			} else if fd, ok := checkName(g, a.text); ok {
				found = append(found, fd)
			}
		}
	}
	var spans []span
	for _, fd := range found {
		spans = append(spans, fd.spans...)
	}
	for _, dn := range dataNames {
		found = append(found, checkDataName(g, dataName(g, dn), spans))
	}
	found = checkClaimedUses(g, found, f.Name)

	if removal && len(found) == 0 {
		base.Verdict, base.Unchecked = Skipped, true
		base.Reason = ReasonRemovalNoFile
		return base
	}
	if len(found) == 0 {
		found = wordFallback(g, f)
	}
	if len(found) == 0 {
		// Nothing in the code mentions any of the line's meaningful words:
		// that's a fact, NOT BUILT (founder's call, 2026-09-29; "Refund button"
		// in a project with no refund code used to come back "rewrite this line").

		// Only explicit source anchors can establish a missing named artifact.
		// The absence of ordinary prose keywords is not absence of behavior.
		if words := specificWords(g, f); len(anchors) > 0 && len(words) > 0 && !anyMentioned(g, words) && !negativeLine(f) {
			base.Verdict = NotBuilt
			base.EvidenceKind = "source_fact"
			base.Reason = fmt.Sprintf("no extracted source matches the explicitly named artifact (%s); unsupported syntax is outside this lookup", orList(words))
			return base
		}
		base.Verdict, base.Unchecked = Skipped, true
		base.Reason = ReasonNamesNothing
		base.Closest = closestInCode(g, f)
		return base
	}
	res := combine(base, found, features.ClaimsConnection(f))
	if res.Verdict == Working {
		if promise != "" {
			// The calls a changed file makes aren't what the line promised; showing
			// them as its flow (or comparing them in the contract check) would
			// suggest a path was checked that wasn't.
			res.Pairs = nil
		} else {
			res = traceFlow(g, f, res, found)
		}
	}
	return res
}

// onlyFiles: the line names at least one file and nothing else checkable.
func onlyFiles(anchors []anchor) bool {
	for _, a := range anchors {
		if a.kind != fileAnchor {
			return false
		}
	}
	return len(anchors) > 0
}

var sourceFactWords = regexp.MustCompile(`(?i)^\s*(?:in\s+)?(?:exists(?:\s+and\s+is\s+imported)?|is\s+defined)\s*\.?\s*$`)

func sourceFactOnly(text string, found []finding) bool {
	rest := backtickPattern.ReplaceAllString(text, "")
	if !sourceFactWords.MatchString(rest) {
		return false
	}
	if strings.Contains(strings.ToLower(rest), "imported") {
		for _, f := range found {
			if !f.use.traced || !f.use.used {
				return false
			}
		}
	}
	return true
}

// combine turns a line's findings into one verdict: a proven problem
// wins over a missing piece, which wins over a half-built one; otherwise
// the line is CONNECTED. When every finding is exists-only, the line is
// HALF BUILT if it claimed a server/database connection (the thing is
// there, but nothing in it talks to anything), or if nothing in the app
// uses it (a file existing is not a feature); CONNECTED otherwise.
func combine(base Result, found []finding, claimsConnection bool) Result {
	rank := map[Verdict]int{Broken: 5, NotBuilt: 4, HalfBuilt: 3, Skipped: 2, Working: 1}
	best := Verdict("")
	for _, fd := range found {
		if rank[fd.verdict] > rank[best] {
			best = fd.verdict
		}
		base.Evidence = append(base.Evidence, fd.evidence...)
		base.Pairs = append(base.Pairs, fd.pairs...)
	}
	allExistsOnly := best == ""
	unused, useNote := "", ""
	if allExistsOnly {
		best = Skipped // Existence/imports alone do not establish a promised behavior.
		for _, fd := range found {
			switch {
			case fd.use.traced && !fd.use.used && unused == "":
				unused = fd.use.how
			case useNote == "":
				useNote = fd.use.how
			}
		}
		if claimsConnection || unused != "" || strings.Contains(useNote, "only tests or fixtures") {
			best = Skipped
		}
	}
	// Exact structural claims can be supported without claiming behavior.
	if allExistsOnly && sourceFactOnly(base.FeatureName, found) {
		best = Working
	}
	// Problems first in the reason text, so the line reads as its verdict.
	var reasons []string
	for _, v := range []Verdict{Broken, NotBuilt, HalfBuilt, Skipped, Working, ""} {
		for _, fd := range found {
			if fd.verdict == v {
				reasons = append(reasons, fd.reason)
			}
		}
	}
	base.Verdict = best
	base.EvidenceKind = "source_fact"
	base.Unchecked = best == Skipped // malveon couldn't tell: not a verdict about the code
	base.Reason = strings.Join(reasons, "; ")
	if allExistsOnly && best != Working {
		base.Reason += "; only source existence/imports were checked; promised behavior remains unverified"
	}
	switch {
	case allExistsOnly && claimsConnection:
		base.ExistsButNoCall = true
		base.Reason += " — source found, but the requested behavior is not established; direct HTTP/database calls are not required for delegated or local work"
	case unused != "":
		base.Reason += " — production integration is not established: " + unused
	case allExistsOnly && useNote != "":
		base.Reason += "; " + useNote
	}
	base.Evidence = dedupe(base.Evidence)
	if base.Unchecked {
		base.CoverageLimit = base.Reason
	}
	return base
}

// ---------------------------------------------------------------------
// What a plan line names
// ---------------------------------------------------------------------

type anchorKind int

const (
	pathAnchor anchorKind = iota // a URL: /api/jobs, /admin/digest
	fileAnchor                   // a file: PostJobModal.tsx, src/app/landing.css
	nameAnchor                   // a code name in backticks: InvitesTab, generate_referral_code
)

type anchor struct {
	kind   anchorKind
	text   string
	method string // HTTP method written right before a URL ("POST /api/jobs"), else ""
}

var (
	backtickPattern  = regexp.MustCompile("`([^`]+)`")
	fileRefPattern   = regexp.MustCompile(`^[\w\-./\[\]()@]*[\w-]\.(?:tsx|ts|jsx|js|mjs|cjs|py|go|rs|java|rb|php|cs|css|scss|less|sql|md|json|html|vue|svelte|prisma|yml|yaml)$`)
	slashFilePattern = regexp.MustCompile(`(?:^|\s)((?:[\w.()\[\]-]+/)+[\w.-]+\.(?:tsx|ts|jsx|js|mjs|cjs|py|go|rs|java|rb|php|cs|css|scss|less|sql|md|json))\b`)
	codeNamePattern  = regexp.MustCompile(`^[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*(?:\(\))?$`)
	// labelSpanPattern finds an ALL-CAPS word in backticks that opens a line
	// and is followed straight by a colon ("`INPUT`: Request payload"): a
	// label, not a code name. `WITHDRAWN` anywhere else still counts.
	labelSpanPattern = regexp.MustCompile("^[\\s*+-]*(?:\\*\\*)?`[A-Z]+`(?:\\*\\*)?:")
	// winPathPattern is a Windows path (src\components\X.tsx), written as people
	// on Windows type it; anchorsIn reads it as the same path with slashes.
	winPathPattern = regexp.MustCompile(`(?:[\w.()\[\]@-]+\\)+[\w.-]+\.(?:tsx|ts|jsx|js|mjs|cjs|py|go|rs|java|rb|php|cs|css|scss|less|sql|md|json|html|vue|svelte|prisma|yml|yaml)\b`)
)

// anchorsIn pulls the checkable things out of a plan line: every URL,
// every file reference (in backticks, or written as a path with "/"),
// and every code name in backticks. A UI breadcrumb in backticks
// (`Settings > CircleTab`) names its last part.
func anchorsIn(name string) []anchor {
	name = citationParenthesis.ReplaceAllString(name, " ")
	name = winPathPattern.ReplaceAllStringFunc(name, func(p string) string { return strings.ReplaceAll(p, "\\", "/") })
	var out []anchor
	seen := map[string]bool{}
	add := func(a anchor) {
		key := fmt.Sprint(a.kind, a.text, a.method)
		if a.text != "" && !seen[key] {
			seen[key] = true
			out = append(out, a)
		}
	}

	for _, u := range features.PlanURLs(name) {
		add(anchor{kind: pathAnchor, text: u.Path, method: u.Method})
	}
	scan := name
	if loc := labelSpanPattern.FindStringIndex(name); loc != nil {
		scan = strings.ReplaceAll(name[:loc[1]], "`", "") + name[loc[1]:] // so the label isn't read as a code span
	}
	for _, m := range backtickPattern.FindAllStringSubmatch(scan, -1) {
		inner := strings.TrimSpace(m[1])
		if i := strings.LastIndex(inner, ">"); i >= 0 {
			inner = strings.TrimSpace(inner[i+1:])
		}
		switch {
		case fileRefPattern.MatchString(inner):
			add(anchor{kind: fileAnchor, text: inner})
		case codeNamePattern.MatchString(inner):
			add(anchor{kind: nameAnchor, text: strings.TrimSuffix(inner, "()")})
		}
	}
	for _, m := range slashFilePattern.FindAllStringSubmatch(name, -1) {
		add(anchor{kind: fileAnchor, text: m[1]})
	}
	return out
}

// Anchored reports whether text names at least one thing this check
// can look up directly: a URL, a file, or a code name in backticks. Used
// to reject a prompt translation that names nothing checkable (3.2.22).
func Anchored(text string) bool { return len(anchorsIn(text)) > 0 }

// AnchorSet is what a line names, by kind.
type AnchorSet struct {
	URLs  []features.PlanURL // routes and pages, with the HTTP method when the line gives one
	Files []string           // files, as written
	Names []string           // code names (functions, components, tables), as written
}

// AnchorsOf lists what text names: its URLs, files and code names.
func AnchorsOf(text string) AnchorSet {
	var out AnchorSet
	for _, a := range anchorsIn(text) {
		switch a.kind {
		case pathAnchor:
			out.URLs = append(out.URLs, features.PlanURL{Path: a.text, Method: a.method})
		case fileAnchor:
			out.Files = append(out.Files, a.text)
		default:
			out.Names = append(out.Names, a.text)
		}
	}
	return out
}

// Decisive reports whether text names something this check can call missing: a URL,
// a file, or a code-shaped name (camelCase, PascalCase, snake_case, dotted). A plain
// lowercase word in backticks (`lodash`, `profiles`) is an anchor too, but its being
// absent from the code proves nothing, so a claim resting on it alone is never
// contradicted (the agent's own notes use this: "added the `lodash` dependency").
func Decisive(text string) bool {
	for _, a := range anchorsIn(text) {
		if a.kind != nameAnchor {
			return true
		}
		parts := strings.Split(a.text, ".")
		if looksLikeCodeName(parts[len(parts)-1]) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------
// Checking each thing against the code
// ---------------------------------------------------------------------

// checkPath answers "is this URL built and connected": served by a route
// and called by the frontend, or a page that exists. A route nothing calls
// is HALF BUILT unless the line says something outside the app calls it
// (needsCaller false) — see features.ExpectsOutsideCaller.
func checkPath(g *graph.Graph, a anchor, needsCaller bool) (finding, bool) {
	p := a.text
	routes, under := routesFor(g, p)
	// A page answers GET by itself: routes that only take other methods there (a
	// Remix action next to its page) don't make GET missing.
	if len(routes) > 0 && a.method == "GET" && len(g.PagesAt(p)) > 0 && !acceptsGET(routes) {
		routes = nil
	}

	if len(routes) > 0 {
		if a.method != "" {
			var withMethod []graph.Node
			for _, r := range routes {
				if r.Method == "" || r.Method == a.method {
					withMethod = append(withMethod, r)
				}
			}
			if len(withMethod) == 0 {
				// A screen that sends this method to a URL the server doesn't accept
				// it on is broken; with no such screen, the method is just not built
				// yet (a plan adding POST next to a GET that exists).
				for _, c := range g.CallsTo(p) {
					if c.Method == "" || c.Method == a.method {
						return finding{verdict: Broken, evidence: append(locs(routes), loc(c)),
							reason: fmt.Sprintf("the frontend sends %s %s (%s), but the server only accepts %s there (%s)",
								a.method, p, loc(c), methodsOf(routes), loc(routes[0]))}, true
					}
				}
				return finding{verdict: NotBuilt, evidence: locs(routes),
					reason: fmt.Sprintf("the server has %s, but no %s handler for it yet (it accepts %s)", p, a.method, methodsOf(routes))}, true
			}
			routes = withMethod
		}
		var pairs, wrongMethod []graph.Pair
		for _, r := range routes {
			for _, c := range g.CallsToRoute(r) {
				if !under && !callNames(c.Path, p) {
					continue // a call to /posts/${id} isn't a call to /posts/pending
				}
				if graph.MethodsCompatible(c, r) {
					pairs = append(pairs, graph.Pair{Call: c, Route: r})
				} else {
					wrongMethod = append(wrongMethod, graph.Pair{Call: c, Route: r})
				}
			}
		}
		if len(pairs) > 0 {
			return connected(pairs), true
		}
		// With a method named in the line, calls using another method on the
		// same URL say nothing about this one: the route is just uncalled.
		if len(wrongMethod) > 0 && a.method == "" {
			return brokenMethod(wrongMethod[0]), true
		}
		// Only a route registered as exactly this URL counts on its own; one
		// that merely catches it (/posts/[id] for /posts/pending) isn't proof
		// the thing was built, so it still needs a real caller.
		// "Exactly" means the same pattern: placeholders where the plan has
		// them (/api/users/:id and /api/users/<int:id>), fixed text where
		// it has fixed text.
		exact := false
		for _, r := range routes {
			if !under && samePattern(p, r.Path) {
				exact = true
			}
		}
		exists := fmt.Sprintf("the server route for %s exists (%s)", p, loc(routes[0]))
		// Say what malveon could not read, rather than claiming nothing calls it.
		if un := g.UnreadableCalls(); len(un) > 0 {
			exists += fmt.Sprintf(" (note: %d call(s) in this project use a URL malveon can't read, e.g. %s — one of them may call it)", len(un), loc(un[0]))
		}
		// A call to it that sits in a function nothing calls isn't a caller.
		for _, r := range routes {
			if ign := g.IgnoredCallsTo(r.Path); len(ign) > 0 {
				exists += fmt.Sprintf(" and %s calls it from `%s`, a function nothing in the project calls", loc(ign[0]), ign[0].EnclosingFunc)
				break
			}
		}
		// A project with no screens at all (a backend API): nothing in it
		// could call the route, so the route existing is the whole job
		// (founder's call, 2026-09-29; every endpoint used to be HALF BUILT
		// unless its line said "public API").
		// When the line names a method, a route must be known to accept it:
		// an unknown method (a route registered without saying) can't prove
		// the DELETE the line asked for was built.
		knownMethod := a.method == ""
		for _, r := range routes {
			knownMethod = knownMethod || r.Method == a.method
		}
		if needsCaller && exact && knownMethod && !g.HasFrontend() {
			return finding{verdict: Working, evidence: locs(routes), spans: routeSpans(routes),
				reason: exists + "; this project has no screens (no frontend code calls any server), so the route is the whole job"}, true
		}
		return uncalledRoute(exists+testCallsNote(g, routes), locs(routes), routeSpans(routes), needsCaller || !exact), true
	}

	if pages := g.PagesAt(p); len(pages) > 0 {
		s := span{file: pages[0].File, label: "page " + p}
		return checkSpan(g, s, fmt.Sprintf("page %s exists (%s)", p, pages[0].File)), true
	}
	if calls := g.CallsTo(p); len(calls) > 0 {
		return finding{verdict: Broken, evidence: locs(calls),
			reason: fmt.Sprintf("the frontend calls %s (%s), but no server route handles it", p, loc(calls[0]))}, true
	}
	// Only call it missing when this project has the kind of code that
	// would serve it — otherwise malveon simply can't read that kind of app.
	apiLike := strings.HasPrefix(p, "/api/") || p == "/api"
	if hasUnreadableRPC(g) {
		return finding{}, false // a tRPC router malveon can't read may serve it
	}
	// A server-rendered app (Django, Flask, Express) serves its pages through routes:
	// when it has routes at non-API URLs, a page URL none of them serves is missing.
	if (apiLike && hasKind(g, graph.RouteHandler)) || (!apiLike && (hasKind(g, graph.Page) || (hasPageRoutes(g) && !staticFileMayServe(g, p)))) {
		return finding{verdict: NotBuilt, reason: fmt.Sprintf("nothing in the code serves or calls %s", p)}, true
	}
	return finding{}, false
}

func acceptsGET(routes []graph.Node) bool {
	for _, r := range routes {
		if r.Method == "" || r.Method == "GET" {
			return true
		}
	}
	return false
}

// routesFor finds the routes a plan URL means: the ones serving exactly
// that path (a static route beats a dynamic one, as in Next.js), or —
// when none does — the group of routes nested under it ("/api/upload"
// naming /api/upload/sign, ...).
func routesFor(g *graph.Graph, p string) (routes []graph.Node, under bool) {
	routes = g.RoutesServing(p)
	var static []graph.Node
	for _, r := range routes {
		if r.Path == p {
			static = append(static, r)
		}
	}
	if len(static) > 0 {
		return static, false
	}
	if len(routes) > 0 {
		return routes, false
	}
	return g.RoutesUnder(p), true
}

// callNames reports whether a call's path names the plan URL: every
// segment equal, or the plan's segment is itself a placeholder. A call's
// own placeholder (/posts/:param) doesn't name a fixed segment (/posts/pending).
func callNames(callPath, planPath string) bool {
	a := strings.Split(strings.Trim(callPath, "/"), "/")
	b := strings.Split(strings.Trim(planPath, "/"), "/")
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if graph.IsWildcardSegment(b[i]) || a[i] == b[i] {
			continue
		}
		return false
	}
	return true
}

// checkFile answers "does this file exist" — and for a removal line,
// "is it gone".
func checkFile(g *graph.Graph, ref string, removal bool) finding {
	files := g.FindFiles(ref)
	switch {
	case removal && len(files) == 0:
		return finding{verdict: Working, reason: fmt.Sprintf("`%s` is gone, as planned", ref)}
	case removal:
		return finding{verdict: NotBuilt, evidence: files, reason: fmt.Sprintf("`%s` is still there — the plan says to remove it", files[0])}
	case len(files) == 0:
		return finding{verdict: NotBuilt, reason: fmt.Sprintf("`%s` doesn't exist in the project", ref)}
	}
	return checkSpan(g, span{file: files[0], label: "`" + filepath.Base(files[0]) + "`"}, fmt.Sprintf("`%s` exists", files[0]))
}

// checkNameDefined is checkName's first half: the name is a component's own
// file or a function/class/type defined somewhere. It says nothing for a name
// the code only uses.
func checkNameDefined(g *graph.Graph, name string) (finding, bool) {
	parts := strings.Split(name, ".")
	last := parts[len(parts)-1]
	label := "`" + last + "`"
	for _, ext := range []string{".tsx", ".ts", ".jsx", ".js", ".vue", ".svelte", ".astro", ".py", ".go"} {
		if files := g.FindFiles(last + ext); len(files) > 0 {
			return checkSpan(g, span{file: files[0], label: label}, fmt.Sprintf("%s exists (%s)", label, files[0])), true
		}
	}
	if def, ok := bestDef(g.Defs[last], last); ok {
		s := span{file: def.File, from: def.Line, to: def.End, label: label, name: last}
		return checkSpan(g, s, fmt.Sprintf("%s is defined in %s:%d", label, def.File, def.Line)), true
	}
	return finding{}, false
}

// checkName answers "is this code name really in the code": a file with
// that name (a component's own file), or a function/component defined
// somewhere. A name the code only uses (a library function) proves
// nothing on its own and is left out; a real code-shaped name that
// appears nowhere at all is NOT BUILT.
func checkName(g *graph.Graph, name string) (finding, bool) {
	if fd, ok := checkNameDefined(g, name); ok {
		return fd, true
	}
	parts := strings.Split(name, ".")
	last := parts[len(parts)-1]
	label := "`" + last + "`"
	if g.NameAnywhere(last) {
		return finding{}, false
	}
	// A plain lowercase word (`profiles`) could be a column or a CSS value
	// malveon doesn't read — too loose to call missing. A code-shaped name
	// (camelCase, PascalCase, snake_case) that's nowhere in the code is.
	if looksLikeCodeName(last) {
		return finding{verdict: NotBuilt, reason: fmt.Sprintf("%s isn't anywhere in the code", label)}, true
	}
	return finding{}, false
}

// bestDef picks the definition a plan's name most likely means: one in a
// file named after it, else a real function body, else the first.
func bestDef(defs []graph.Loc, name string) (graph.Loc, bool) {
	if len(defs) == 0 {
		return graph.Loc{}, false
	}
	for _, d := range defs {
		if strings.TrimSuffix(filepath.Base(d.File), filepath.Ext(d.File)) == name {
			return d, true
		}
	}
	for _, d := range defs {
		if d.End > 0 {
			return d, true
		}
	}
	return defs[0], true
}

func looksLikeCodeName(s string) bool {
	return strings.ContainsAny(s, "_$") || strings.ToLower(s) != s
}

// dataName returns which part of a code name is a database table,
// function, bucket, or Supabase auth call the code actually uses
// (`profiles.notification_settings` -> "profiles"), or "" if none is.
func dataName(g *graph.Graph, name string) string {
	parts := strings.Split(name, ".")
	candidates := []string{name, parts[0], parts[len(parts)-1]}
	if len(parts) >= 3 {
		candidates = append(candidates, parts[len(parts)-2]) // prisma.post.create -> post
	}
	for _, candidate := range candidates {
		if len(g.DataCallsTo(candidate)) > 0 {
			return candidate
		}
	}
	return ""
}

// checkDataName answers "does the code this line is about actually use
// this table/function". When the line also names a component, page, or
// URL, only that code counts — and the server routes it calls — never a
// use somewhere unrelated. A line naming only the table checks the whole
// codebase.
func checkDataName(g *graph.Graph, name string, spans []span) finding {
	all := g.DataCallsTo(name)
	what := dataLabel(all[0], name)
	if near := typoTable(g, all[0]); near != "" {
		return finding{verdict: Broken, evidence: locs(all),
			reason: fmt.Sprintf("the code uses table `%s` (%s), which your migrations don't define; `%s` is — a typo?", name, loc(all[0]), near)}
	}
	if len(spans) == 0 {
		return finding{verdict: Working, evidence: locs(all), reason: fmt.Sprintf("the code uses %s (%s)", what, loc(all[0]))}
	}

	searched := withCalledRoutes(g, spans)
	for _, dc := range all {
		for _, s := range searched {
			if s.contains(dc) {
				return finding{verdict: Working, evidence: []string{loc(dc)},
					reason: fmt.Sprintf("%s uses %s (%s)", spans[0].label, what, loc(dc))}
			}
		}
	}
	// The use may sit in a project file the code imports: a route that calls a
	// helper (lib/notify.ts) which does the insert.
	for _, s := range searched {
		if !jsFile(s.file) {
			continue
		}
		files, _ := importClosure(g, s.file, 3)
		for _, f := range files[1:] {
			for _, dc := range all {
				if dc.File == f {
					return finding{verdict: Working, evidence: []string{loc(dc)},
						reason: fmt.Sprintf("%s reaches %s through %s (%s)", spans[0].label, what, f, loc(dc))}
				}
			}
		}
	}
	// A call through a same-named helper: the component calls
	// resetPasswordForEmail(), whose own code reaches Supabase.
	for _, s := range spans {
		if g.NameIn(s.file, name) {
			return finding{verdict: Working, evidence: []string{loc(all[0])},
				reason: fmt.Sprintf("%s calls `%s`, which reaches %s (%s)", s.label, name, what, loc(all[0]))}
		}
	}
	return finding{verdict: HalfBuilt, evidence: []string{spans[0].file},
		reason: fmt.Sprintf("%s never uses %s — checked its code and the server routes it calls", spans[0].label, what)}
}

// withCalledRoutes is spans plus the server routes their code calls: a
// component that fetches a route reaches whatever that route uses.
func withCalledRoutes(g *graph.Graph, spans []span) []span {
	out := append([]span(nil), spans...)
	for _, s := range spans {
		for _, n := range g.NodesInFile(s.file) {
			if n.Kind == graph.NetworkCall && n.Confidence == graph.Extracted && s.contains(n) {
				for _, r := range g.RoutesServing(n.Path) {
					out = append(out, span{file: r.File})
				}
			}
		}
	}
	return out
}

func dataLabel(n graph.Node, name string) string {
	switch n.Method {
	case "RPC":
		return "database function `" + name + "`"
	case "STORAGE":
		return "storage bucket `" + name + "`"
	case "AUTH":
		return "Supabase auth (`" + name + "`)"
	case "FUNCTION":
		return "edge function `" + name + "`"
	}
	return "table `" + name + "`"
}

// checkSpan looks at what a file (or one function in it) that exists
// actually does: every server call in it must reach a real route; any
// call or database use makes it CONNECTED; no calls at all is exists-only.
func checkSpan(g *graph.Graph, s span, existsReason string) finding {
	var pairs []graph.Pair
	var talks []string
	for _, n := range g.NodesInFile(s.file) {
		if n.Confidence != graph.Extracted || !s.contains(n) {
			continue
		}
		switch n.Kind {
		case graph.NetworkCall:
			routes := g.RoutesServing(n.Path)
			if len(routes) == 0 {
				return finding{verdict: Broken, evidence: []string{loc(n)}, spans: []span{s},
					reason: fmt.Sprintf("%s — but it calls %s %s (%s), and no server route handles that", existsReason, methodLabel(n), n.Path, loc(n))}
			}
			var ok *graph.Node
			for i := range routes {
				if graph.MethodsCompatible(n, routes[i]) {
					ok = &routes[i]
					break
				}
			}
			if ok == nil {
				fd := brokenMethod(graph.Pair{Call: n, Route: routes[0]})
				fd.reason = existsReason + " — but " + fd.reason
				fd.spans = []span{s}
				return fd
			}
			pairs = append(pairs, graph.Pair{Call: n, Route: *ok})
			talks = append(talks, fmt.Sprintf("%s %s", methodLabel(n), n.Path))
		case graph.DataCall:
			if near := typoTable(g, n); near != "" {
				return finding{verdict: Broken, evidence: []string{loc(n)}, spans: []span{s},
					reason: fmt.Sprintf("%s — but it uses table `%s` (%s), which your migrations don't define; `%s` is — a typo?", existsReason, n.Path, loc(n), near)}
			}
			talks = append(talks, dataLabel(n, n.Path))
		}
	}
	if len(talks) == 0 {
		return finding{existsOnly: true, evidence: []string{s.file}, spans: []span{s}, reason: existsReason, use: usageOf(g, s.file, s.name)}
	}
	return finding{verdict: Working, evidence: []string{s.file}, pairs: pairs, spans: []span{s},
		reason: fmt.Sprintf("%s and talks to %s", existsReason, strings.Join(dedupe(talks), ", "))}
}

// typoTable: a table the code uses that the project's schema doesn't define,
// but that is one or two letters from one it does. "" otherwise — an unknown
// name alone proves nothing, the schema may live outside the files read.
func typoTable(g *graph.Graph, n graph.Node) string {
	if n.Method != "TABLE" {
		return ""
	}
	return g.NearTable(n.Path)
}

func routeSpans(routes []graph.Node) []span {
	var out []span
	for _, r := range routes {
		out = append(out, span{file: r.File, label: "route " + r.Path, route: true, url: r.Path, method: r.Method})
	}
	return out
}

// ---------------------------------------------------------------------
// Lines that name nothing: strong word match only
// ---------------------------------------------------------------------

// wordFallback matches a line that names no URL/file/code name against
// code by its words — but only strongly: every meaningful word of the
// node's URL must appear in the line, and the line and node must share
// at least half of the line's meaningful words. "refund button" still
// finds fetch("/refund"); "...the two sets read as one..." no longer
// finds /api/notifications/read-all.
func wordFallback(g *graph.Graph, f features.Feature) []finding {
	lineWords := g.ContentWords(f.Words())
	if len(lineWords) == 0 {
		return nil
	}
	var found []finding
	for _, c := range strongMatches(g, graph.NetworkCall, lineWords) {
		found = append(found, checkCall(g, c))
	}
	if len(found) > 0 {
		return found
	}
	for _, r := range strongMatches(g, graph.RouteHandler, lineWords) {
		var pairs []graph.Pair
		for _, c := range g.CallsToRoute(r) {
			if graph.MethodsCompatible(c, r) {
				pairs = append(pairs, graph.Pair{Call: c, Route: r})
			}
		}
		if len(pairs) > 0 {
			found = append(found, connected(pairs))
		} else {
			// A word match alone is never enough for CONNECTED: it needs a real call.
			found = append(found, uncalledRoute(fmt.Sprintf("the server route %s %s exists (%s)", methodLabel(r), r.Path, loc(r))+testCallsNote(g, []graph.Node{r}),
				[]string{loc(r)}, nil, true))
		}
	}
	return found
}

// testCallsNote says when a test calls one of these routes ("" when none
// does). A test never makes a route CONNECTED — the app isn't calling it — but
// naming it shows the route isn't being ignored, only untested-by-the-app.
func testCallsNote(g *graph.Graph, routes []graph.Node) string {
	for _, r := range routes {
		if calls := g.TestCallsTo(r.Path); len(calls) > 0 {
			return fmt.Sprintf(" — only tests call it (%s)", loc(calls[0]))
		}
	}
	return ""
}

// uncalledRoute is a route that exists with nothing in the app calling
// it: HALF BUILT if the line asked for a caller, CONNECTED otherwise, with
// the missing caller still said plainly in the reason.
func uncalledRoute(exists string, evidence []string, spans []span, needsCaller bool) finding {
	if needsCaller {
		return finding{verdict: HalfBuilt, evidence: evidence, spans: spans,
			reason: exists + ", but no frontend call to it was found (if something outside the app calls it — a health check, webhook, cron job or public API — say so in the line)"}
	}
	return finding{verdict: Working, evidence: evidence, spans: spans,
		reason: exists + "; nothing in the app calls it, and this line says it's called from outside the app"}
}

func strongMatches(g *graph.Graph, kind graph.Kind, lineWords []string) []graph.Node {
	inLine := map[string]bool{}
	for _, w := range lineWords {
		inLine[stem(w)] = true
	}
	var out []graph.Node
	for _, n := range g.Nodes {
		if n.Kind != kind || n.Confidence != graph.Extracted || n.InTest {
			continue // a test's call isn't what a plan line is about
		}
		pathWords := g.ContentWords(pathWordsOf(n.Path))
		if len(pathWords) == 0 {
			continue
		}
		covered := true
		for _, w := range pathWords {
			if !inLine[stem(w)] {
				covered = false
				break
			}
		}
		if !covered {
			continue
		}
		shared := map[string]bool{}
		for _, w := range g.ContentWords(n.Words) {
			if inLine[stem(w)] {
				shared[stem(w)] = true
			}
		}
		if 2*len(shared) >= len(inLine) {
			out = append(out, n)
		}
	}
	return out
}

// vagueWords say what kind of thing or change a line is about, never which
// feature: actions (add, make), UI element types (button, page, form) and
// quality words (faster, better). A line made only of these has nothing to
// look for in the code. Edit here.
var vagueWords = map[string]bool{
	"add": true, "adds": true, "added": true, "make": true, "makes": true, "create": true, "build": true,
	"implement": true, "support": true, "allow": true, "allows": true, "let": true, "lets": true, "can": true,
	"should": true, "must": true, "show": true, "shows": true, "display": true, "update": true, "fix": true,
	"improve": true, "handle": true, "use": true, "using": true, "work": true, "works": true, "working": true,
	"button": true, "buttons": true, "page": true, "pages": true, "screen": true, "form": true, "forms": true,
	"feature": true, "features": true, "component": true, "section": true, "view": true, "modal": true,
	"link": true, "list": true, "user": true, "users": true, "new": true, "app": true,
	"feel": true, "faster": true, "fast": true, "better": true, "nicer": true, "nicely": true, "cleaner": true,
	"easier": true, "simple": true, "simpler": true, "good": true, "nice": true, "clean": true, "properly": true,
	"correctly": true, "correct": true, "right": true, "exported": true, "export": true, "it": true,
}

// specificWords are the line's words that name *what* it's about: its
// content words (graph.ContentWords: no stopwords, nothing too common in
// this codebase), minus vagueWords, at least three letters.
func specificWords(g *graph.Graph, f features.Feature) []string {
	var out []string
	for _, w := range g.ContentWords(f.Words()) {
		if len(w) >= 3 && !vagueWords[w] {
			out = append(out, w)
		}
	}
	return out
}

// samePattern: route registers exactly the plan's URL pattern, not a
// wider one that merely catches it (/posts/[id] catching /posts/pending).
func samePattern(planPath, routePath string) bool {
	a := strings.Split(strings.Trim(planPath, "/"), "/")
	b := strings.Split(strings.Trim(routePath, "/"), "/")
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		wa, wb := graph.IsWildcardSegment(a[i]), graph.IsWildcardSegment(b[i])
		if wa != wb || (!wa && a[i] != b[i]) {
			return false
		}
	}
	return true
}

// negativeLine: the line says something should *not* exist ("Zero
// placeholder pages remain", "No more mock data", "never shows raw
// errors"). Code not mentioning its words may mean it's already true, so
// such a line never becomes NOT BUILT for that reason; it stays unchecked.
func negativeLine(f features.Feature) bool {
	for _, w := range f.Words() {
		switch w {
		case "zero", "no", "none", "never", "without", "not", "nothing", "remain", "remains", "anymore", "longer", "stop", "stops":
			return true
		}
	}
	return false
}

func anyMentioned(g *graph.Graph, words []string) bool {
	for _, w := range words {
		if g.MentionsWord(w) {
			return true
		}
	}
	return false
}

// orList joins words as "a", "a or b", "a, b or c".
func orList(words []string) string {
	if len(words) <= 1 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " or " + words[len(words)-1]
}

func pathWordsOf(path string) []string {
	var words []string
	for _, seg := range strings.Split(strings.Trim(path, "/"), "/") {
		if seg == "" || graph.IsWildcardSegment(seg) {
			continue
		}
		words = append(words, graph.SplitWords(seg)...)
	}
	return words
}

// checkCall answers "does this frontend call reach a real route".
func checkCall(g *graph.Graph, c graph.Node) finding {
	routes := g.RoutesServing(c.Path)
	if len(routes) == 0 {
		return finding{verdict: Broken, evidence: []string{loc(c)},
			reason: fmt.Sprintf("the frontend calls %s %s (%s), but no server route handles it", methodLabel(c), c.Path, loc(c))}
	}
	var pairs []graph.Pair
	for _, r := range routes {
		if graph.MethodsCompatible(c, r) {
			pairs = append(pairs, graph.Pair{Call: c, Route: r})
		}
	}
	if len(pairs) == 0 {
		return brokenMethod(graph.Pair{Call: c, Route: routes[0]})
	}
	return connected(pairs)
}

// ---------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------

func connected(pairs []graph.Pair) finding {
	p := pairs[0]
	var spans []span
	allStubs := true
	for _, pr := range pairs {
		spans = append(spans, span{file: pr.Route.File, label: "route " + pr.Route.Path, route: true, url: pr.Route.Path, method: pr.Route.Method})
		allStubs = allStubs && pr.Route.Stub
	}
	note := ""
	for _, pr := range pairs {
		if pr.Route.Unmounted {
			// The route is in a router malveon never saw mounted: the URL
			// prefix it is served under is unverified, not confirmed.
			note = "; note: this route is in a router malveon didn't see mounted, so the URL prefix it is served under is unverified"
			break
		}
	}
	if allStubs {
		// The route is there and called, but its handler only answers with an
		// error: registered, not built.
		return finding{verdict: HalfBuilt, pairs: pairs, spans: spans, evidence: []string{loc(p.Call), loc(p.Route)},
			reason: fmt.Sprintf("%s %s reaches its server route (%s), but that handler only answers with an error (500/501 or \"not implemented\") — it isn't built yet",
				methodLabel(p.Call), p.Route.Path, loc(p.Route))}
	}
	return finding{verdict: Working, pairs: pairs, spans: spans, evidence: []string{loc(p.Call), loc(p.Route)},
		reason: fmt.Sprintf("%s %s reaches its server route", methodLabel(p.Call), p.Route.Path) + note}
}

func brokenMethod(p graph.Pair) finding {
	return finding{verdict: Broken, pairs: []graph.Pair{p}, evidence: []string{loc(p.Call), loc(p.Route)},
		reason: fmt.Sprintf("the frontend sends %s %s (%s), but the server only accepts %s there (%s)",
			p.Call.Method, p.Call.Path, loc(p.Call), p.Route.Method, loc(p.Route))}
}

func hasKind(g *graph.Graph, k graph.Kind) bool {
	for _, n := range g.Nodes {
		if n.Kind == k {
			return true
		}
	}
	return false
}

// staticFileMayServe: a plain file in the project could be what answers this URL
// (/about served by about.html or about/index.html), which no route would show.
func staticFileMayServe(g *graph.Graph, url string) bool {
	segs := strings.Split(strings.Trim(url, "/"), "/")
	last := segs[len(segs)-1]
	if last == "" {
		return false
	}
	for _, f := range g.Files {
		slash := strings.ToLower(strings.ReplaceAll(f, "\\", "/"))
		base := slash[strings.LastIndexByte(slash, '/')+1:]
		dir := ""
		if i := strings.LastIndexByte(slash, '/'); i > 0 {
			dir = slash[strings.LastIndexByte(slash[:i], '/')+1 : i]
		}
		l := strings.ToLower(last)
		if base == l+".html" || base == l+".htm" || (dir == l && (base == "index.html" || base == "index.htm")) {
			return true
		}
	}
	return false
}

// hasPageRoutes reports whether the project has server routes at URLs that are
// not under /api: the mark of an app that serves its pages through routes.
func hasPageRoutes(g *graph.Graph) bool {
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && n.Confidence == graph.Extracted && !n.RPC &&
			n.Path != "" && n.Path != "/" && !strings.HasPrefix(n.Path, "/api/") && n.Path != "/api" {
			return true
		}
	}
	return false
}

func methodsOf(nodes []graph.Node) string {
	var ms []string
	for _, n := range nodes {
		if n.Method != "" {
			ms = append(ms, n.Method)
		}
	}
	return strings.Join(dedupe(ms), "/")
}

func methodLabel(n graph.Node) string {
	if n.Method != "" {
		return n.Method
	}
	return "call"
}

func loc(n graph.Node) string { return fmt.Sprintf("%s:%d", n.File, n.Line) }

func locs(nodes []graph.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, loc(n))
	}
	return out
}

func dedupe(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
