package wiring

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// A line naming a place (a route's URL, a file, a component) and a function
// in backticks says the place uses the function: "`PATCH /api/admin/reports/[id]`
// sends `createNotification` to the reporter". The URL reaching a route and
// the function existing somewhere prove neither half of that (found
// 2026-09-30 on a real Next.js app: the route only updated a table, the helper lived
// in another file, and the line read CONNECTED). So the code the place
// reaches — its own file, the project files it imports three levels deep,
// and the server routes it calls — must mention the name.
//
// "Mention" is deliberately loose (graph.Names: any identifier-shaped word,
// a type use or a comment counts), and whole files are searched, never one
// handler: wrongly calling a name unused is the error that matters, so every
// doubt keeps the line CONNECTED. Doubt means: a place whose imports malveon
// can't follow (not JS/TS), or one that imports project code it couldn't read.
// A place that imports the file defining the name counts as reaching it —
// what it imports from there may call the name.

// checkClaimedUses runs the check over a line's findings. Everything the line
// names that is code — a file, a component, a page, a function — must be
// connected to something else the line names: it uses or calls it, directly
// or through the project files it imports and the server routes it calls, or
// it is used or called by it. "`OrdersPage` loads orders from `GET /api/orders`"
// holds when the page calls the route; "`PATCH /api/orders/[id]` sends
// `createNotification`" holds when the route reaches the function. The two
// directions are both accepted — "A renders B" and "B is rendered by A" are
// one claim — but a pair with no connection either way is HALF BUILT: both
// things exist, and nothing links them (found 2026-10-12: "`Orphan` calls
// `GET /api/items`" read CONNECTED because Orphan and the route each exist).
//
// A proven problem is left alone; so is a thing with nothing else to be
// connected to, and two bare files with no "uses/renders/calls" word between
// them (two files named in one line need not be linked).
func checkClaimedUses(g *graph.Graph, found []finding, line string) []finding {
	type ent struct {
		fi int
		s  span
	}
	var ents []ent
	seen := map[string]bool{}
	for i, fd := range found {
		for _, s := range fd.spans {
			k := fmt.Sprint(i, "|", s.file, "|", s.label, "|", s.method)
			if !seen[k] {
				seen[k] = true
				ents = append(ents, ent{i, s})
			}
		}
	}
	if len(ents) < 2 {
		return found
	}
	linked := usesVerbPattern.MatchString(line)
	for _, e := range ents {
		if e.s.route {
			continue // a route is the other end of the claim, never the claimed thing
		}
		fd := found[e.fi]
		if fd.verdict != Working && fd.verdict != "" {
			continue // BROKEN / NOT BUILT / HALF BUILT already decide this line
		}
		var others []span
		for _, o := range ents {
			if o.fi == e.fi || o.s.file == e.s.file {
				continue
			}
			if !o.s.route && e.s.name == "" && o.s.name == "" && !linked {
				continue // two bare files: only a "uses/renders/calls" line links them
			}
			others = append(others, o.s)
		}
		if len(others) == 0 {
			continue
		}
		var hit connection
		unsure := false
		for _, o := range others {
			c, u := connect(g, e.s, o)
			if c.how == "" {
				var back bool
				c, back = connect(g, o, e.s)
				u = u || back
			}
			if c.how != "" {
				hit = c
				break
			}
			unsure = unsure || u
		}
		switch {
		case hit.how != "":
			found[e.fi].reason += "; " + hit.sentence()
		case unsure:
			found[e.fi].reason += fmt.Sprintf("; malveon can't follow everything %s reaches, so it can't confirm %s is connected to it", spanLabels(others), e.s.label)
		default:
			found[e.fi] = unconnected(fd, e.s, others)
		}
	}
	return found
}

// usesVerbPattern: the line says one thing uses, renders, calls or loads another.
var usesVerbPattern = regexp.MustCompile(`(?i)\b(?:uses?|using|used by|renders?|rendered|shows?|displays?|imports?|imported|calls?|called|loads?|wraps?|includes?|contains?|mounts?|embeds?|opens?|triggers?|sends?|posts?|fetch(?:es)?|reads?|exports?|wires?|hooks?|connects?|links?)\b`)

// connection is how one named thing reaches another: from's code (or the
// files it imports, or the routes it calls) uses to.
type connection struct {
	from, to span
	how      string // where: "src/lib/x.ts", "src/a.tsx:12"
	viaFile  bool   // reached through another file rather than its own code
	viaProp  bool   // the component's parent hands it what makes the request
}

func (c connection) sentence() string {
	switch {
	case c.viaProp:
		return fmt.Sprintf("%s is handed what makes the request by its parent (%s)", c.from.label, c.how)
	case c.to.route:
		return fmt.Sprintf("%s calls %s (%s)", c.from.label, c.to.label, c.how)
	case c.viaFile:
		return fmt.Sprintf("%s reaches %s through %s", c.from.label, c.to.label, c.how)
	}
	return fmt.Sprintf("%s uses %s (%s)", c.from.label, c.to.label, c.how)
}

// connect looks for from reaching to: when to is a route, a call to it from
// from's code or what it reaches; otherwise to's file imported, or to's name
// mentioned, in the code from reaches. unsure is true when some of that code
// couldn't be read (imports are followed in JS/TS only), so "no" proves nothing.
func connect(g *graph.Graph, from, to span) (c connection, unsure bool) {
	files, unsure := reachedFiles(g, from)
	if to.route {
		for _, r := range g.Nodes {
			if r.Kind != graph.RouteHandler || r.File != to.file || r.Path != to.url || (to.method != "" && r.Method != to.method) {
				continue
			}
			for _, call := range g.CallsToRoute(r) {
				if !graph.MethodsCompatible(call, r) || !hasString(files, call.File) {
					continue
				}
				if from.to != 0 && call.File == from.file && !from.contains(call) {
					continue // the call is in another function of the same file
				}
				return connection{from: from, to: to, how: loc(call)}, unsure
			}
		}
		// The component may not make the request itself: its parent hands it the
		// function (<RefundButton onRefund={doRefund} />).
		for _, r := range g.Nodes {
			if r.Kind != graph.RouteHandler || r.File != to.file || r.Path != to.url || (to.method != "" && r.Method != to.method) {
				continue
			}
			for _, call := range g.CallsToRoute(r) {
				if !graph.MethodsCompatible(call, r) {
					continue
				}
				if how, ok := handedDown(g, from, call); ok {
					return connection{from: from, to: to, how: how, viaProp: true}, unsure
				}
			}
		}
		return connection{}, unsure
	}
	for _, f := range files {
		switch {
		case f == to.file && f != from.file:
			return connection{from: from, to: to, how: f, viaFile: f != from.file}, unsure
		case to.name != "" && g.NameIn(f, to.name):
			return connection{from: from, to: to, how: f, viaFile: f != from.file}, unsure
		}
	}
	return connection{}, unsure
}

// reachedFiles is the code a span reaches: its own file, the project files
// that file imports three levels deep, and the same for every server route it
// calls. unsure when a file isn't JS/TS or an import matched no file.
func reachedFiles(g *graph.Graph, s span) (files []string, unsure bool) {
	for _, sp := range withCalledRoutes(g, []span{s}) {
		if !jsFile(sp.file) {
			unsure = true
			files = append(files, sp.file)
			continue
		}
		fs, missing := importClosure(g, sp.file, 3)
		files = append(files, fs...)
		if len(missing) > 0 {
			unsure = true
		}
	}
	return dedupe(files), unsure
}

// handedDown: the component defined in from's file is given a handler by a
// parent that renders it (<RefundButton onRefund={doRefund} />), and that parent
// reaches the file that makes the call — itself, or through what it imports.
// The component then makes the request through what it was handed, though its
// own file never touches it. All three must hold: the parent renders this
// component (not another one), the prop is a handler, and the parent reaches the
// request; a parent that gives a plain value, nothing, or code unrelated to the
// request proves nothing. how is where the parent hands it over.
func handedDown(g *graph.Graph, from span, call graph.Node) (how string, ok bool) {
	// Which component the line means: the code name it gave, else the one a file
	// is named after (RefundButton.tsx -> RefundButton). Another component in the
	// same file being handed something proves nothing about this one.
	want := from.name
	if want == "" {
		want = strings.TrimSuffix(filepath.Base(from.file), filepath.Ext(from.file))
	}
	for _, t := range g.Triggers {
		if t.Handler.Kind == graph.HandlerNone || t.Tag != want || !definedIn(g, t.Tag, from.file) {
			continue
		}
		if files, _ := reachedFiles(g, span{file: t.File}); hasString(files, call.File) {
			return triggerLoc(t), true
		}
	}
	return "", false
}

// definedIn: name is a function or component defined in file.
func definedIn(g *graph.Graph, name, file string) bool {
	for _, l := range g.Defs[name] {
		if l.File == file {
			return true
		}
	}
	return false
}

func hasString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// unconnected is the HALF BUILT finding for a thing that exists while nothing
// the line names uses it or is used by it.
func unconnected(fd finding, s span, others []span) finding {
	if s.name != "" {
		return unusedName(fd, s, others)
	}
	ev := append([]string(nil), fd.evidence...)
	for _, o := range others {
		ev = append(ev, o.file)
	}
	return finding{verdict: HalfBuilt, evidence: dedupe(ev), pairs: fd.pairs, spans: fd.spans,
		reason: fmt.Sprintf("%s exists (%s), but nothing connects it to %s — it doesn't use or call it, and it isn't used or called by it (checked its code, the project files it imports and the server routes it calls)",
			s.label, s.file, spanLabels(others))}
}

// unusedName is the HALF BUILT finding for a function that exists while the
// place the line names never uses it.
func unusedName(fd finding, def span, at []span) finding {
	who := spanLabels(at) + " never uses it"
	if len(at) > 1 {
		who = "none of " + spanLabels(at) + " uses it"
	}
	ev := append([]string(nil), fd.evidence...)
	for _, p := range at {
		ev = append(ev, p.file)
	}
	return finding{verdict: HalfBuilt, evidence: dedupe(ev), pairs: fd.pairs, spans: fd.spans,
		reason: fmt.Sprintf("`%s` exists (%s:%d), but %s — checked its code, the project files it imports and the server routes it calls",
			def.name, def.file, def.from, who)}
}

func spanLabels(spans []span) string {
	var labels []string
	for _, s := range spans {
		labels = append(labels, s.label)
	}
	return strings.Join(dedupe(labels), " and ")
}

// nameHit is where a name was found: the file that mentions it, reached from
// one of the line's places.
type nameHit struct {
	file string
	from span
}

// reachesName looks for name in the code each place reaches. unsure is true
// when some of that code couldn't be read, so "not found" proves nothing.
func reachesName(g *graph.Graph, places []span, name string) (hit nameHit, unsure bool) {
	for _, p := range places {
		for _, s := range withCalledRoutes(g, []span{p}) {
			if !jsFile(s.file) {
				unsure = true // imports are followed in JS/TS only
				if g.NameIn(s.file, name) {
					return nameHit{file: s.file, from: p}, unsure
				}
				continue
			}
			files, missing := importClosure(g, s.file, 3)
			for _, f := range files {
				if g.NameIn(f, name) {
					return nameHit{file: f, from: p}, unsure
				}
			}
			if len(missing) > 0 {
				unsure = true
			}
		}
	}
	return nameHit{}, unsure
}

func jsFile(file string) bool {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return true
	}
	return false
}
