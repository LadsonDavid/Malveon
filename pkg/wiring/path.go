package wiring

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/features"
	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// One end-to-end path per flow (2026-09-25, a tester: "one end-to-end
// path for every user-visible flow"). For a CONNECTED line, malveon traces
// the flow through the code — never by running it:
//
//	Screen  the page that shows the code making the call (imports followed backwards)
//	Action  the call itself
//	Server  the route that answers it
//	State   what that route, and the project code it imports, changes
//
// A missing Screen is shown but never blocks (some flows start outside a
// page). A missing State blocks — HALF BUILT — only when the line expects
// a change (a PUT/PATCH/DELETE call, or POST with a word like "save") and
// the route provably changes nothing: every database call it makes is a
// read, and it reaches nothing malveon can't read (an RPC, another URL, a
// service SDK, a database client other than Supabase, a missing import).

// state is what a route was found to change.
type state struct {
	hop      string // the State hop as the report shows it
	writes   bool   // a data change was seen
	noChange bool   // proven: the route changes no data
	evidence string
	// typo: the route (or code it imports) uses a table the schema doesn't
	// define but that is a letter or two from one it does — it would fail.
	typo string
}

var changeWordPattern = regexp.MustCompile(`(?i)\b(?:save|saves|saving|submit|submits|submitting|store|stores|persist|persists|insert|inserts|upsert|upserts|delete|deletes|deleting|remove|removes|removing|toggle|toggles|approve|approves|reject|rejects|invite|invites|join|joins|follow|follows|unfollow|unfollows|vote|votes|upload|uploads|register|registers|rsvp|rsvps|bookmark|bookmarks)\b`)

// gainVerbPattern: what a plan line says a file will do to itself ("`PostCard.tsx`
// adds a report option"). Edit here. Only a verb written after the file's
// name counts: before it, the file is the verb's object ("Add `ReportsTab.tsx`"
// creates the file, it doesn't promise the file gains something).
var gainVerbPattern = regexp.MustCompile(`(?i)\b(?:add|adds|render|renders|display|displays|update|updates|export|exports|show|shows|remove|removes|register|registers|wire|wires|replace|replaces|change|changes|connect|connects|handle|handles)\b`)

// promisedChange returns what a line says its file will do, quoted from the
// line ("adds the report button to the post detail…"), or "" when the line
// makes no such promise. files are the file references the line names.
func promisedChange(name string, files []string) string {
	first := -1
	for _, f := range files {
		if i := strings.Index(name, f); i >= 0 && (first < 0 || i < first) {
			first = i + len(f)
		}
	}
	if first < 0 {
		return ""
	}
	rest := name[first:]
	// A verb-looking part of another backticked name ("`handle.ts`") isn't the line's verb.
	masked := backtickPattern.ReplaceAllStringFunc(rest, func(s string) string { return strings.Repeat(" ", len(s)) })
	loc := gainVerbPattern.FindStringIndex(masked)
	if loc == nil {
		return ""
	}
	words := strings.Fields(rest[loc[0]:])
	if len(words) > 8 {
		return strings.Join(words[:8], " ") + "…"
	}
	return strings.Join(words, " ")
}

// sessionChange judges a file-only line against what changed since the
// session started. It replaces the finding checkFile made (a file existing,
// or the unrelated routes it calls, says nothing about the line's promise):
//
//   - file unchanged → NOT BUILT, a plain fact
//   - file changed   → the old finding, plus a note that malveon sees the file
//     changed, not that the promised addition is there
//
// A finding that is already a proven problem (BROKEN) stays as it is.
func sessionChange(g *graph.Graph, fd finding, ref, promise string, changed map[string]bool, inSession bool) finding {
	files := g.FindFiles(ref)
	if len(files) == 0 || fd.verdict == Broken || fd.verdict == NotBuilt {
		return fd
	}
	file := files[0]
	if !changed[filepath.ToSlash(file)] {
		// A prompt was typed during the session, so an untouched file is not done.
		// A plan document may describe work finished before it: malveon can't tell,
		// and says so rather than claiming NOT BUILT (sweep 2026-10-12).
		if !inSession {
			return finding{verdict: Skipped, evidence: []string{file}, spans: fd.spans,
				reason: fmt.Sprintf("`%s` exists, but it hasn't changed since this session started, and this line says it \"%s\" — "+
					"malveon can't tell whether it already did that before the session or still doesn't; "+
					"name the new component or function in backticks and malveon will look for it", file, promise)}
		}
		return finding{verdict: NotBuilt, evidence: []string{file}, spans: fd.spans,
			reason: fmt.Sprintf("`%s` exists, but it hasn't changed since this session started, and this line says it \"%s\" — "+
				"if it already did that before the session, name the new component or function in backticks so malveon can look for it", file, promise)}
	}
	fd.reason += fmt.Sprintf("; it changed since this session started — malveon sees the file changed, not that it \"%s\"; "+
		"name the new component or function in backticks to check that", promise)
	return fd
}

// changeExpected reports whether this call is meant to change data: its
// method says so (PUT/PATCH/DELETE), or it's a POST and the line uses a
// word that does.
func changeExpected(f features.Feature, p graph.Pair) bool {
	m := p.Call.Method
	if m == "" {
		m = p.Route.Method
	}
	switch m {
	case "PUT", "PATCH", "DELETE":
		return true
	case "POST":
		return changeWordPattern.MatchString(f.Name)
	}
	return false
}

// traceFlow adds the end-to-end path to a CONNECTED line, and turns it
// HALF BUILT when the line expects a change its route provably never makes.
func traceFlow(g *graph.Graph, f features.Feature, res Result, found []finding) Result {
	if len(res.Pairs) == 0 {
		res.Path = directPath(g, found)
		return res
	}

	states := map[string]state{}
	stateFor := func(r graph.Node) state {
		key := loc(r)
		if _, ok := states[key]; !ok {
			states[key] = stateOf(g, r)
		}
		return states[key]
	}

	best := res.Pairs[0]
	for _, p := range res.Pairs {
		if stateFor(p.Route).writes {
			best = p
			break
		}
	}
	pathOf := func(p graph.Pair) []string {
		return []string{
			screenOf(g, p.Call.File),
			fmt.Sprintf("%s %s (%s)", methodLabel(p.Call), p.Call.Path, loc(p.Call)),
			fmt.Sprintf("route %s %s (%s)", methodLabel(p.Route), p.Route.Path, loc(p.Route)),
			stateFor(p.Route).hop,
		}
	}
	res.Path = pathOf(best)
	// When a button or form runs the request, say so in the Call hop (the path
	// keeps its four hops: the editor labels them by position).
	if rc := reachOf(g, best.Call); rc.reached && namesATrigger(f.Name) {
		res.Path[1] = triggerHop(rc.trigger) + " → " + res.Path[1]
	}

	// A route that reaches a table the schema doesn't have would fail, however
	// well the screen is wired to it.
	for _, p := range res.Pairs {
		if s := stateFor(p.Route); s.typo != "" {
			res.Verdict = Broken
			res.Path = pathOf(p)
			res.Reason = fmt.Sprintf("%s %s reaches its server route (%s), but the route %s", methodLabel(p.Call), p.Route.Path, loc(p.Route), s.typo)
			res.Evidence = dedupe(append(res.Evidence, s.evidence))
			return res
		}
	}

	// A line about a button whose request nothing ever runs: clicking does nothing.
	if r, done := checkTriggers(g, f, res); done {
		return r
	}

	var expected []graph.Pair
	for _, p := range res.Pairs {
		if changeExpected(f, p) {
			expected = append(expected, p)
		}
	}
	if len(expected) == 0 {
		return res
	}
	for _, p := range expected {
		if !stateFor(p.Route).noChange {
			return res
		}
	}
	p := expected[0]
	s := stateFor(p.Route)
	res.Verdict = HalfBuilt
	res.Path = pathOf(p)
	res.Reason = fmt.Sprintf("%s %s reaches its server route (%s), but the route never changes any data: it %s — and this line expects a change",
		methodLabel(p.Call), p.Route.Path, loc(p.Route), strings.TrimPrefix(s.hop, "changes no data: "))
	if s.evidence != "" {
		res.Evidence = dedupe(append(res.Evidence, s.evidence))
	}
	return res
}

// directPath is the path for a line whose code talks to the database
// itself (no server route in between): Screen, then the database use.
func directPath(g *graph.Graph, found []finding) []string {
	for _, fd := range found {
		if fd.verdict != Working {
			continue
		}
		for _, s := range fd.spans {
			for _, n := range g.NodesInFile(s.file) {
				if n.Kind == graph.DataCall && s.contains(n) {
					verb, _, _ := dataChange(n)
					use := fmt.Sprintf("%s (%s)", verb, loc(n))
					if !isPage(g, s.file) {
						use = s.label + " " + use // a component, not the page itself
					}
					return []string{screenOf(g, s.file), use}
				}
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------

func isPage(g *graph.Graph, file string) bool {
	for _, n := range g.Nodes {
		if n.Kind == graph.Page && n.File == file {
			return true
		}
	}
	return false
}

// The two hops that mean "malveon can't read this kind of app". They are short
// on purpose: the report explains them once under the CONNECTED list (see
// report.pathLimitNotes), not in every line — in a project that isn't Next.js
// the same long sentence used to follow every single CONNECTED line.
const (
	PageNotTraced    = "page not traced"
	NoDataChangeSeen = "no data change malveon can see"
)

// screenOf names the page a file is shown on: the file itself when it's a
// page, else the nearest page that imports it (directly or through other
// components).
func screenOf(g *graph.Graph, file string) string {
	pages := map[string]string{}
	for _, n := range g.Nodes {
		if n.Kind == graph.Page {
			pages[n.File] = n.Path
		}
	}
	if p, ok := pages[file]; ok {
		return "page " + p
	}
	if len(pages) == 0 {
		// No pages found at all: this isn't an app malveon can read pages in,
		// so "no page shows it" would be a claim nobody checked.
		return PageNotTraced
	}
	seen := map[string]bool{file: true}
	level := []string{file}
	for depth := 0; depth < 8 && len(level) > 0; depth++ {
		var next, hits []string
		for _, f := range level {
			for _, imp := range g.ImportersOf(f) {
				if seen[imp] {
					continue
				}
				seen[imp] = true
				if p, ok := pages[imp]; ok {
					hits = append(hits, p)
				}
				next = append(next, imp)
			}
		}
		if len(hits) > 0 {
			sort.Strings(hits)
			hits = dedupe(hits)
			if len(hits) == 1 {
				return "page " + hits[0]
			}
			return fmt.Sprintf("page %s (and %d more)", hits[0], len(hits)-1)
		}
		level = next
	}
	return "no page found that shows " + filepath.Base(file)
}

// ---------------------------------------------------------------------
// State
// ---------------------------------------------------------------------

// effectHints are names that mean the code talks to a data store or
// service malveon doesn't read — a change could be happening there.
var effectHints = map[string]bool{
	"knex": true, "mongoose": true, "sequelize": true, "kysely": true, "typeorm": true, "dynamodb": true,
	"mongodb": true, "firestore": true, "firebase": true, "redis": true, "upstash": true, "postgres": true,
	"mysql": true, "resend": true, "nodemailer": true, "sendgrid": true, "sgmail": true, "twilio": true,
	"stripe": true, "openai": true, "anthropic": true, "s3client": true, "putobjectcommand": true,
	"sendmail": true, "transporter": true,
}

// driverNames are database driver names a Drizzle or Prisma client setup file
// mentions only to connect.
var driverNames = map[string]bool{"postgres": true, "mysql": true}

// importClosure is file plus every project file it imports, depth levels
// deep (file first, nearest imports next), and the project imports that
// matched no file — code malveon can't read.
func importClosure(g *graph.Graph, file string, depth int) (files, missing []string) {
	files = []string{file}
	seen := map[string]bool{file: true}
	for d, level := 0, []string{file}; d < depth && len(level) > 0; d++ {
		var next []string
		for _, f := range level {
			for _, im := range g.Imports[f] {
				if im.File == "" {
					missing = append(missing, im.Missing)
					continue
				}
				if !seen[im.File] {
					seen[im.File] = true
					files = append(files, im.File)
					next = append(next, im.File)
				}
			}
		}
		level = next
	}
	return files, missing
}

// stateOf works out what a route changes: the route's own handler (plus
// any helper in its file that isn't another handler), and every project
// file it imports, three levels deep.
func stateOf(g *graph.Graph, route graph.Node) state {
	from, to := handlerSpan(g, route)
	others := otherHandlers(g, route)
	inScope := func(n graph.Node) bool {
		if n.File != route.File {
			return true
		}
		if n.Line >= from && (to == 0 || n.Line <= to) {
			return true
		}
		for _, o := range others {
			if n.Line >= o[0] && n.Line <= o[1] {
				return false
			}
		}
		return true
	}

	files, missing := importClosure(g, route.File, 3)

	var writes, maybes, reads []graph.Node
	var outbound []graph.Node
	var typoNode *graph.Node
	typoNear := ""
	for _, f := range files {
		for _, n := range g.NodesInFile(f) {
			if !inScope(n) {
				continue
			}
			switch n.Kind {
			case graph.DataCall:
				if near := typoTable(g, n); near != "" && typoNode == nil {
					n := n
					typoNode, typoNear = &n, near
				}
				_, write, known := dataChange(n)
				switch {
				case write:
					writes = append(writes, n)
				case !known:
					maybes = append(maybes, n)
				default:
					reads = append(reads, n)
				}
			case graph.NetworkCall:
				outbound = append(outbound, n)
			}
		}
	}

	if typoNode != nil {
		return state{hop: fmt.Sprintf("uses table `%s` (%s), which your migrations don't define", typoNode.Path, loc(*typoNode)),
			typo:     fmt.Sprintf("uses table `%s` (%s), which your migrations don't define; `%s` is — a typo?", typoNode.Path, loc(*typoNode), typoNear),
			evidence: loc(*typoNode)}
	}
	if len(writes) > 0 {
		verb, _, _ := dataChange(writes[0])
		hop := fmt.Sprintf("%s (%s)", verb, loc(writes[0]))
		if len(writes) > 1 {
			hop += fmt.Sprintf(" and %d more change(s)", len(writes)-1)
		}
		return state{hop: hop, writes: true, evidence: loc(writes[0])}
	}
	if len(maybes) > 0 {
		verb, _, _ := dataChange(maybes[0])
		return state{hop: fmt.Sprintf("%s (%s) — may change data", verb, loc(maybes[0]))}
	}
	if len(outbound) > 0 {
		o := outbound[0]
		what := o.Path
		if what == "" {
			what = "a URL built at runtime"
		}
		return state{hop: fmt.Sprintf("passes the request on to %s (%s) — what that changes isn't traced", what, loc(o))}
	}
	if len(missing) > 0 {
		return state{hop: fmt.Sprintf("imports code malveon couldn't find (%s) — state change not traced", missing[0])}
	}
	for _, f := range files {
		names := make([]string, 0, len(g.Names[f]))
		for name := range g.Names[f] {
			names = append(names, name)
		}
		sort.Strings(names) // the same report every run, not map order
		setsUpORM := g.Names[f]["drizzle"] || g.Names[f]["PrismaClient"]
		for _, name := range names {
			// The file that creates the Drizzle/Prisma client names its database
			// driver ("postgres", "mysql2"): that is the client malveon reads, not
			// a second, unread way to write.
			if setsUpORM && driverNames[strings.ToLower(name)] {
				continue
			}
			if effectHints[strings.ToLower(name)] {
				return state{hop: fmt.Sprintf("uses `%s` (%s) — a service or database malveon doesn't read, so a change there isn't traced", name, f)}
			}
		}
	}
	if !hasKind(g, graph.DataCall) {
		return state{hop: NoDataChangeSeen}
	}
	if len(reads) > 0 {
		verb, _, _ := dataChange(reads[0])
		return state{hop: fmt.Sprintf("changes no data: only %s (%s)", verb, loc(reads[0])), noChange: true, evidence: loc(reads[0])}
	}
	return state{hop: "changes no data: makes no database call in the route or the project code it imports", noChange: true}
}

// handlerSpan is the route's own handler function (a Next.js
// `export async function POST`), or 0,0 when it isn't a named function
// (an inline Express handler) — then the whole file counts.
func handlerSpan(g *graph.Graph, route graph.Node) (from, to int) {
	for _, name := range []string{route.Method, route.EnclosingFunc} {
		for _, d := range g.Defs[name] {
			if d.File == route.File && d.Line == route.Line && d.End > 0 {
				return d.Line, d.End
			}
		}
	}
	return 0, 0
}

// otherHandlers are the spans of the file's other named route handlers
// (GET next to POST), so one handler's database calls never count for another.
func otherHandlers(g *graph.Graph, route graph.Node) [][2]int {
	var out [][2]int
	for _, n := range g.NodesInFile(route.File) {
		// A page in the same file (a Remix route module) is another handler: its
		// component is the screen, not the code the route runs.
		if (n.Kind != graph.RouteHandler && n.Kind != graph.Page) || n.Line == route.Line {
			continue
		}
		if from, to := handlerSpan(g, n); to > 0 {
			out = append(out, [2]int{from, to})
		}
	}
	return out
}

// dataChange describes one database call: what it does in words, whether
// it's a change, and whether malveon knows either way.
func dataChange(n graph.Node) (verb string, write, known bool) {
	name := n.Path
	switch n.Method {
	case "TABLE":
		switch n.Op {
		case "select":
			return "reads table `" + name + "`", false, true
		case "insert":
			return "inserts into table `" + name + "`", true, true
		case "update":
			return "updates table `" + name + "`", true, true
		case "upsert":
			return "upserts into table `" + name + "`", true, true
		case "delete":
			return "deletes from table `" + name + "`", true, true
		}
		return "uses table `" + name + "`", false, false
	case "STORAGE":
		switch n.Op {
		case "upload", "update", "move", "copy", "remove":
			return "changes storage bucket `" + name + "` (`" + n.Op + "`)", true, true
		case "download", "list", "getPublicUrl", "createSignedUrl", "createSignedUrls", "exists", "info":
			return "reads storage bucket `" + name + "`", false, true
		}
		return "uses storage bucket `" + name + "`", false, false
	case "AUTH":
		// signOut ends sessions (scope "others" revokes every other device) — a change.
		if strings.HasPrefix(n.Op, "get") || strings.HasPrefix(n.Op, "on") {
			return "reads the signed-in user (`auth." + n.Op + "`)", false, true
		}
		return "changes auth (`auth." + n.Op + "`)", true, true
	case "RPC":
		return "calls database function `" + name + "`", false, false
	case "FUNCTION":
		return "calls edge function `" + name + "`", false, false
	case "SQL":
		// A statement the ORM client ran without a table malveon could name.
		switch n.Op {
		case "write":
			return "runs raw SQL that changes the database", true, true
		case "insert", "update", "delete":
			return "runs a raw SQL " + strings.ToUpper(n.Op) + " (table not named)", true, true
		}
		return "uses " + name + " — what it changes isn't readable", false, false
	}
	return "uses `" + name + "`", false, false
}

// ---------------------------------------------------------------------
// Is a file used at all
// ---------------------------------------------------------------------

// usage answers "does anything in the app use this file" for a line whose
// only fact is that a file exists (a tester: "files exist" is not a
// feature). traced is false when malveon can't tell — then it never blocks.
type usage struct {
	used, traced bool
	how          string
}

var (
	// Files malveon can follow imports into.
	importTraced = map[string]bool{".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".mjs": true, ".cjs": true, ".css": true, ".scss": true, ".less": true}
	// Next.js loads these itself; nothing imports them.
	frameworkLoaded = map[string]bool{"page": true, "layout": true, "route": true, "template": true, "loading": true,
		"error": true, "not-found": true, "global-error": true, "default": true, "middleware": true, "instrumentation": true}
	// Started by a command or a tool, not imported: can't be judged unused.
	entryNames = map[string]bool{"index": true, "main": true, "server": true, "app": true, "cli": true, "worker": true, "sw": true}
	toolFile   = regexp.MustCompile(`(?i)\.(config|test|spec|stories|d)\.[cm]?[jt]sx?$|^(eslint|postcss|tailwind|vite|next|jest|vitest|webpack|babel|prettier)\b`)
)

func usageOf(g *graph.Graph, file, name string) usage {
	if name != "" {
		return nameUsage(g, file, name)
	}
	base := filepath.Base(file)
	ext := strings.ToLower(filepath.Ext(base))
	stem := strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
	switch {
	case !importTraced[ext]:
		return usage{how: "whether anything uses a " + ext + " file isn't traced"}
	case frameworkLoaded[stem]:
		return usage{used: true, traced: true, how: "Next.js loads " + base + " itself"}
	}
	if imp := g.ImportersOf(file); len(imp) > 0 {
		how := "imported by " + imp[0]
		if len(imp) > 1 {
			how += fmt.Sprintf(" and %d more", len(imp)-1)
		}
		return usage{used: true, traced: true, how: how}
	}
	switch {
	case entryNames[stem] || toolFile.MatchString(base):
		return usage{how: base + " may be started by a command or a tool, which malveon doesn't read"}
	case (ext == ".js" || ext == ".css") && hasFileExt(g, ".html"):
		return usage{how: base + " may be loaded by an HTML page, which malveon doesn't read"}
	}
	return usage{traced: true, how: "no file imports " + base}
}

func hasFileExt(g *graph.Graph, ext string) bool {
	for _, f := range g.Files {
		if strings.EqualFold(filepath.Ext(f), ext) {
			return true
		}
	}
	return false
}

// nameUsage is usageOf for one code name the line named: some file must
// import that name (or the whole file). Its file being imported for other
// names doesn't count — the name might still be used inside its own file,
// so that case is "not traced", never "unused".
func nameUsage(g *graph.Graph, file, name string) usage {
	fileUse := usageOf(g, file, "")
	if !fileUse.traced || !importTraced[strings.ToLower(filepath.Ext(file))] {
		return fileUse
	}
	var by []string
	for _, imp := range g.ImportersOf(file) {
		for _, im := range g.Imports[imp] {
			if im.File == file && (im.Name == name || im.Name == "") {
				by = append(by, imp)
				break
			}
		}
	}
	switch {
	case len(by) > 0:
		how := "`" + name + "` is imported by " + by[0]
		if len(by) > 1 {
			how += fmt.Sprintf(" and %d more", len(by)-1)
		}
		return usage{used: true, traced: true, how: how}
	case fileUse.used && strings.HasPrefix(fileUse.how, "Next.js loads"):
		return fileUse
	case fileUse.used:
		return usage{how: "its file is imported, but not `" + name + "` itself — it may be used inside " + filepath.Base(file)}
	}
	return usage{traced: true, how: "no file imports `" + name + "` or " + filepath.Base(file)}
}
