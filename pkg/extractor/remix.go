package extractor

import (
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// Remix and React Router (framework mode). A route module is a file that
// exports a default component (a page), `loader` (answers GET) and/or `action`
// (answers POST, PUT, PATCH and DELETE); without a default component it is a
// resource route. Which URL a module serves comes from `app/routes.ts` when the
// project has one, else from Remix's flat-file naming in `app/routes/`
// (`posts.$id.tsx` is /posts/:id). Only in a project whose package.json lists a
// @remix-run/* or @react-router/* package.

type remixFacts struct {
	rel                              string
	hasDefault, hasLoader, hasAction bool
	loaderLine, actionLine           int
	defaultName                      string // the page component's name, "" when anonymous
	defaultLine                      int
	selfForms                        []selfForm
}

// selfForm is a <Form method="post"> with no action: it posts to the route
// that renders it.
type selfForm struct {
	line   int
	method string
}

type remixProject struct {
	files   map[string]map[string]*remixFacts // project root -> path under it (slashes) -> what the module exports
	configs map[string]string                 // project root -> app/routes.ts
}

func newRemixProject() *remixProject {
	return &remixProject{files: map[string]map[string]*remixFacts{}, configs: map[string]string{}}
}

var (
	remixFormTag   = regexp.MustCompile(`(?is)<(?:Form|[\w$]+\.Form)\b[^>]*>`)
	remixSubmit    = regexp.MustCompile(`\b(?:[\w$]+\s*\.\s*)?(?:submit)\s*\(`)
	remixLoadCall  = regexp.MustCompile(`\.\s*load\s*\(\s*['"](/[^'"]*)['"]`)
	remixActionKw  = regexp.MustCompile(`\baction\s*:\s*['"](/[^'"]*)['"]`)
	remixMethodKw  = regexp.MustCompile(`\bmethod\s*:\s*['"](\w+)['"]`)
	remixExports   = map[string]bool{"loader": true, "action": true}
	remixActionAny = []string{"POST", "PUT", "PATCH", "DELETE"}
)

func remixRoot(in *jsInfo, rel string) (string, bool) {
	return in.depDir(rel, "@remix-run/", "@react-router/")
}

// readFile records a module of a Remix / React Router project and returns the
// calls it makes to an explicit URL.
func (rp *remixProject) readFile(rel, src string, info *jsInfo) []callSite {
	root, ok := remixRoot(info, rel)
	if !ok {
		return nil
	}
	slash := filepath.ToSlash(rel)
	if root != "." {
		slash = strings.TrimPrefix(slash, root+"/")
	}
	if !strings.HasPrefix(slash, "app/") {
		return nil
	}
	if strings.TrimSuffix(slash, path.Ext(slash)) == "app/routes" {
		rp.configs[root] = src
		return nil
	}
	src = maskComments(src, jsSyntax)
	cm := newCodeMap(src, jsSyntax)
	f := &remixFacts{rel: rel, hasDefault: hasDefaultExport(src, cm)}
	if f.hasDefault {
		for _, m := range remixDefaultName.FindAllStringSubmatchIndex(src, -1) {
			if cm.codeAt(src, m[0]) {
				f.defaultLine = lineOf(src, m[0])
				for g := 1; g <= 3; g++ {
					if m[2*g] >= 0 {
						f.defaultName = src[m[2*g]:m[2*g+1]]
					}
				}
				break
			}
		}
	}
	for _, e := range exportedNames(src, cm, remixExports) {
		if e.name == "loader" {
			f.hasLoader, f.loaderLine = true, e.line
		} else {
			f.hasAction, f.actionLine = true, e.line
		}
	}

	var calls []callSite
	for _, m := range remixFormTag.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		tag := src[m[0]:m[1]]
		method := strings.ToUpper(attr(tplMethodAttr.FindStringSubmatch(tag)))
		if method == "" {
			method = "GET"
		}
		action := attr(tplActionAttr.FindStringSubmatch(tag))
		switch {
		case strings.HasPrefix(action, "/"):
			if strings.HasPrefix(tag, "<Form") || strings.HasPrefix(tag, "<form") {
				continue // the generic form scanner reads <Form action="/x"> already
			}
			calls = append(calls, callSite{kind: graph.NetworkCall, method: method, line: lineOf(src, m[0]), argRaw: `"` + action + `"`})
		case action == "" && !formActionAny.MatchString(tag) && method != "GET":
			f.selfForms = append(f.selfForms, selfForm{lineOf(src, m[0]), method})
		}
	}
	for _, m := range remixSubmit.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		args := readArgsList(src, m[1]-1)
		a := remixActionKw.FindStringSubmatch(args)
		if a == nil {
			continue
		}
		method := "GET"
		if mm := remixMethodKw.FindStringSubmatch(args); mm != nil {
			method = strings.ToUpper(mm[1])
		}
		calls = append(calls, callSite{kind: graph.NetworkCall, method: method, line: lineOf(src, m[0]), argRaw: `"` + a[1] + `"`})
	}
	for _, m := range remixLoadCall.FindAllStringSubmatchIndex(src, -1) {
		if cm.codeAt(src, m[0]) {
			calls = append(calls, callSite{kind: graph.NetworkCall, method: "GET", line: lineOf(src, m[0]), argRaw: `"` + src[m[2]:m[3]] + `"`})
		}
	}

	if rp.files[root] == nil {
		rp.files[root] = map[string]*remixFacts{}
	}
	rp.files[root][slash] = f
	return calls
}

// apply adds the pages, resource routes and own-route form posts of every
// module that serves a URL.
func (rp *remixProject) apply(g *graph.Graph) {
	roots := make([]string, 0, len(rp.files))
	for r := range rp.files {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	for _, root := range roots {
		urls := map[string][]string{} // module path under the root -> the URLs it serves
		add := func(file string, us []string) {
			for _, u := range us {
				if !contains(urls[file], u) {
					urls[file] = append(urls[file], u)
				}
			}
		}
		useFlat := true
		if cfg, ok := rp.configs[root]; ok {
			routes, flat := parseRRConfig(cfg)
			for _, r := range routes {
				add(path.Clean(path.Join("app", r.file)), r.urls)
			}
			useFlat = flat
		}
		if useFlat {
			for slash := range rp.files[root] {
				if rest, ok := strings.CutPrefix(slash, "app/routes/"); ok {
					add(slash, flatRoutePaths(strings.Split(rest, "/")))
				}
			}
		}
		files := make([]string, 0, len(rp.files[root]))
		for f := range rp.files[root] {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, slash := range files {
			f, us := rp.files[root][slash], urls[slash]
			if len(us) == 0 {
				continue
			}
			var sites []callSite
			if f.hasDefault {
				line := 1
				if f.defaultLine > 0 {
					line = f.defaultLine
				}
				page := routeSites(graph.Page, line, nil, us)
				if f.defaultName != "" {
					page = handlerNamed(f.defaultName, page) // the component is not the action's code
				}
				sites = append(sites, page...)
			} else if f.hasLoader {
				sites = append(sites, handlerNamed("loader", routeSites(graph.RouteHandler, f.loaderLine, []string{"GET"}, us))...)
			}
			if f.hasAction {
				sites = append(sites, handlerNamed("action", routeSites(graph.RouteHandler, f.actionLine, remixActionAny, us))...)
			}
			for _, sf := range f.selfForms {
				for _, s := range routeSites(graph.NetworkCall, sf.line, []string{sf.method}, us) {
					s.fileRouted = false
					sites = append(sites, s)
				}
			}
			for _, s := range sites {
				g.Nodes = append(g.Nodes, toNode(f.rel, s))
			}
		}
	}
}

// flatRoutePaths is the URL(s) of a file under app/routes in Remix's flat-file
// naming: dots are slashes, $name is a parameter, $ alone is a splat, a leading
// underscore is a layout that adds no URL, a trailing one keeps the URL but not
// the layout, (x) is optional, [.] is a literal dot, and `route.tsx` inside a
// folder takes the folder's name. Not routes: .server/.client modules and tests.
func flatRoutePaths(rest []string) []string {
	file := rest[len(rest)-1]
	stem := stemOf(file)
	if strings.HasSuffix(stem, ".server") || strings.HasSuffix(stem, ".client") || strings.Contains(stem, ".test") || strings.Contains(stem, ".spec") {
		return nil
	}
	dirs := append([]string{}, rest[:len(rest)-1]...)
	// remix-flat-routes: a folder named `api+` holds files of the flat route `api.<file>`.
	for i, d := range dirs {
		dirs[i] = strings.TrimSuffix(d, "+")
	}
	var name string
	if stem == "route" && len(dirs) > 0 {
		name = strings.Join(dirs, ".")
	} else {
		name = strings.Join(append(append([]string{}, dirs...), stem), ".")
	}
	paths := []string{""}
	for _, seg := range splitFlat(name) {
		var out string
		optional := false
		switch {
		case seg == "_index":
			continue
		case strings.HasPrefix(seg, "_"):
			continue // a pathless layout
		case seg == "$":
			out = ":...splat?"
		case strings.HasPrefix(seg, "($") && strings.HasSuffix(seg, ")"):
			out, optional = ":"+seg[2:len(seg)-1], true
		case strings.HasPrefix(seg, "(") && strings.HasSuffix(seg, ")"):
			out, optional = seg[1:len(seg)-1], true
		case strings.HasPrefix(seg, "$"):
			out = ":" + seg[1:]
		default:
			out = strings.TrimSuffix(seg, "_")
		}
		out = strings.ReplaceAll(out, "[.]", ".")
		if optional {
			var next []string
			for _, p := range paths {
				next = append(next, p, p+"/"+out)
			}
			paths = next
			continue
		}
		for i := range paths {
			paths[i] += "/" + out
		}
	}
	for i := range paths {
		if paths[i] == "" {
			paths[i] = "/"
		}
	}
	return paths
}

// splitFlat splits a flat route name at its dots, keeping [.] together.
func splitFlat(name string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case '[':
			depth++
		case ']':
			depth--
		case '.':
			if depth == 0 {
				out = append(out, name[start:i])
				start = i + 1
			}
		}
	}
	return append(out, name[start:])
}

// ---------------------------------------------------------------------------
// app/routes.ts
// ---------------------------------------------------------------------------

type rrRoute struct {
	file string // as written in routes.ts: "./auth/login.tsx"
	urls []string
}

var rrCall = regexp.MustCompile(`^(route|index|layout|prefix)\s*\(`)

// parseRRConfig reads a React Router routes.ts: nested route(), index(),
// layout() and prefix() calls (also spread). flat reports a flatRoutes() call, which
// adds the file-naming routes of app/routes.
func parseRRConfig(src string) (routes []rrRoute, flat bool) {
	flat = strings.Contains(src, "flatRoutes(")
	i := strings.Index(src, "export default")
	if i < 0 {
		return nil, flat
	}
	rest := src[i:]
	open := strings.IndexByte(rest, '[')
	if open < 0 {
		return nil, flat
	}
	arr := readBalanced(rest, open)
	if len(arr) < 2 {
		return nil, flat
	}
	parseRRList(arr[1:len(arr)-1], []string{"/"}, &routes)
	return routes, flat
}

func parseRRList(inner string, parents []string, out *[]rrRoute) {
	for _, e := range splitTopLevelArgs(inner) {
		e = strings.TrimPrefix(strings.TrimSpace(e), "...")
		m := rrCall.FindStringSubmatch(e)
		if m == nil {
			continue
		}
		args := splitTopLevelArgs(readArgsList(e, strings.IndexByte(e, '(')))
		str := func(i int) string {
			if i < len(args) {
				s, _ := jsStr(args[i])
				return s
			}
			return ""
		}
		children := func(i int) (string, bool) {
			if i < len(args) && strings.HasPrefix(strings.TrimSpace(args[i]), "[") {
				a := readBalanced(strings.TrimSpace(args[i]), 0)
				return a[1 : len(a)-1], true
			}
			return "", false
		}
		switch m[1] {
		case "route":
			urls := rrJoin(parents, str(0))
			*out = append(*out, rrRoute{file: str(1), urls: urls})
			if c, ok := children(2); ok {
				parseRRList(c, urls, out)
			}
		case "index":
			*out = append(*out, rrRoute{file: str(0), urls: parents})
		case "layout":
			if c, ok := children(1); ok {
				parseRRList(c, parents, out)
			}
		case "prefix":
			if c, ok := children(1); ok {
				parseRRList(c, rrJoin(parents, str(0)), out)
			}
		}
	}
}

// rrJoin puts a route path (`users/:id/edit?`, `files/*`) under each parent; an
// optional segment gives two URLs.
func rrJoin(parents []string, p string) []string {
	rels := []string{""}
	for _, seg := range strings.Split(strings.Trim(p, "/"), "/") {
		if seg == "" {
			continue
		}
		switch {
		case seg == "*":
			for i := range rels {
				rels[i] += "/:...splat?"
			}
		case strings.HasSuffix(seg, "?"):
			var next []string
			for _, r := range rels {
				next = append(next, r, r+"/"+strings.TrimSuffix(seg, "?"))
			}
			rels = next
		default:
			for i := range rels {
				rels[i] += "/" + seg
			}
		}
	}
	var out []string
	for _, par := range parents {
		for _, r := range rels {
			out = append(out, joinURL(par, r))
		}
	}
	return out
}

// handlerNamed marks the routes as served by the function of that name, so what the
// route changes is judged from that function's lines and not from the whole module
// (which also holds the page's own forms).
func handlerNamed(fn string, sites []callSite) []callSite {
	for i := range sites {
		sites[i].enclosingFunc = fn
	}
	return sites
}

var remixDefaultName = regexp.MustCompile(`export\s+default\s+(?:async\s+)?(?:function\s*\*?\s*([A-Za-z_$][\w$]*)|class\s+([A-Za-z_$][\w$]*)|([A-Za-z_$][\w$]*)\s*;?\s*$)`)
