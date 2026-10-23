package extractor

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// Express-style mounting: app.use('/api', require('./routes/items')) puts every
// route registered in routes/items.js under /api. Without following it, a
// route registered as '/items' never matched a call to '/api/items' and two
// routers both registering '/list' looked like a collision (found by audit,
// 2026-10-10).

type jsMount struct {
	prefix string // "" when mounted at the root
	spec   string // the module mounted, as written: './routes/items'
}

var (
	jsUseCall      = regexp.MustCompile(`\b\w+\.use\s*\(`)
	requireSpec    = regexp.MustCompile(`^require\s*\(\s*['"]([^'"]+)['"]\s*\)(?:\.\w+)?$`)
	plainIdent     = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)
	routerDeclCall = regexp.MustCompile(`\bRouter\s*\(`)
)

// scanJSMounts finds app.use([prefix,] router) calls whose router is a
// require('./x') or a name imported from a module. A prefix that isn't a plain
// literal makes the mount unknown, so it is skipped.
func scanJSMounts(src string) []jsMount {
	cm := newCodeMap(src, jsSyntax)
	var out []jsMount
	var imports map[string]string
	for _, m := range jsUseCall.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		args := splitTopLevelArgs(readArgsList(src, m[1]-1))
		if len(args) == 0 || args[0] == "" {
			continue
		}
		prefix := ""
		if c := args[0][0]; c == '\'' || c == '"' || c == '`' {
			p, ok := literalPath(args[0])
			if !ok {
				continue
			}
			prefix = p
		}
		target := args[len(args)-1]
		spec := ""
		if rm := requireSpec.FindStringSubmatch(target); rm != nil {
			spec = rm[1]
		} else if plainIdent.MatchString(target) {
			if imports == nil {
				imports = importTable(src) // once per file, and only if a .use(name) needs it
			}
			spec = imports[target]
		}
		if spec != "" {
			out = append(out, jsMount{prefix: prefix, spec: spec})
		}
	}
	return out
}

// jsDeclaresRouter: the file creates an Express-style Router.
func jsDeclaresRouter(src string) bool {
	cm := newCodeMap(src, jsSyntax)
	for _, m := range routerDeclCall.FindAllStringIndex(src, -1) {
		if cm.codeAt(src, m[0]) {
			return true
		}
	}
	return false
}

type mountEdge struct {
	from, to, prefix string
}

// applyMounts gives every route in a mounted file its mount prefix(es), and
// marks routes of a router file that nothing mounts as Unmounted: their real
// URL is unknown, so they can't be compared with other files' routes.
func applyMounts(g *graph.Graph, mounts map[string][]jsMount, routerFiles map[string]bool) {
	bySlash := map[string]string{}
	for _, f := range g.Files {
		bySlash[filepath.ToSlash(f)] = f
	}
	byTarget := map[string][]mountEdge{}
	for from, ms := range mounts {
		for _, m := range ms {
			bases, _ := specBases(from, m.spec)
			if to, ok := firstFile(bySlash, bases); ok && to != from {
				byTarget[to] = append(byTarget[to], mountEdge{from: from, to: to, prefix: m.prefix})
			}
		}
	}

	var prefixesOf func(file string, depth int) []string
	prefixesOf = func(file string, depth int) []string {
		edges := byTarget[file]
		if len(edges) == 0 || depth > 6 {
			return nil
		}
		seen := map[string]bool{}
		var out []string
		for _, e := range edges {
			parents := prefixesOf(e.from, depth+1)
			if len(parents) == 0 {
				parents = []string{""}
			}
			for _, p := range parents {
				if full := joinURL(p, e.prefix); !seen[full] {
					seen[full] = true
					out = append(out, full)
				}
			}
		}
		return out
	}

	nodes := make([]graph.Node, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		if n.Kind != graph.RouteHandler || n.Confidence != graph.Extracted {
			nodes = append(nodes, n)
			continue
		}
		if ps := prefixesOf(n.File, 0); len(ps) > 0 {
			for _, p := range ps {
				c := n
				c.Path = joinURL(p, n.Path)
				c.Words = wordsFor(n.File, c.Path)
				nodes = append(nodes, c)
			}
			continue
		}
		if routerFiles[n.File] {
			n.Unmounted = true
		}
		nodes = append(nodes, n)
	}
	g.Nodes = nodes
}

// joinURL joins two URL parts with exactly one slash between them.
func joinURL(a, b string) string {
	a, b = strings.Trim(a, "/"), strings.Trim(b, "/")
	switch {
	case a == "" && b == "":
		return "/"
	case a == "":
		return "/" + b
	case b == "":
		return "/" + a
	}
	return "/" + a + "/" + b
}

// specBases are the project paths a module specifier may mean, relative to
// the importing file (see resolveImports). project is false for a bare name,
// which is a package unless it matches a project file.
func specBases(file, spec string) (bases []string, project bool) {
	dir := path.Dir(filepath.ToSlash(file))
	switch {
	case strings.HasPrefix(spec, "."):
		bases, project = []string{path.Join(dir, spec)}, true
	case strings.HasPrefix(spec, "@/"), strings.HasPrefix(spec, "~/"):
		bases, project = []string{path.Join("src", spec[2:]), spec[2:], path.Join("app", spec[2:])}, true
	case strings.HasPrefix(spec, "$lib/"):
		bases, project = []string{path.Join("src/lib", spec[5:])}, true // SvelteKit
	default:
		bases, project = []string{path.Join("src", spec), spec}, false
	}
	// TypeScript's ESM style writes the compiled name: './items.js' is items.ts.
	for _, b := range bases {
		for _, ext := range []string{".js", ".jsx", ".mjs", ".cjs"} {
			if strings.HasSuffix(b, ext) {
				bases = append(bases, strings.TrimSuffix(b, ext))
			}
		}
	}
	return bases, project
}
