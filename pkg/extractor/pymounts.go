package extractor

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// Python routers mounted from another file: in main.py,
//
//	from things import bp
//	app.register_blueprint(bp, url_prefix="/v2")
//
// puts every route that hangs on `bp` in things.py under /v2 (on top of the
// blueprint's own url_prefix, which the same-file scan already applied). Found
// by audit, 2026-10-10: without it the real URL read BROKEN and the unprefixed
// one read CONNECTED.

type pyImport struct {
	module string // dotted module, with leading dots when relative
	name   string // the imported name in that module ("" for `import module`)
}

var (
	pyFromImport   = regexp.MustCompile(`(?m)^[ \t]*from[ \t]+([.\w]+)[ \t]+import[ \t]+([^\n#(]+)`)
	pyPlainImport  = regexp.MustCompile(`(?m)^[ \t]*import[ \t]+([\w.]+)(?:[ \t]+as[ \t]+(\w+))?[ \t]*(?:#.*)?$`)
	pyMountCallAny = regexp.MustCompile(`\.(?:include_router|register_blueprint)\s*\(`)
)

// pyMountRef is one mount call: which router (a variable, as imported) and the prefix.
type pyMountRef struct {
	prefix string
	module string // module that defines the router variable
	name   string // the router variable in that module
	// alt: when the router is written module.var, the module may be a package
	// attribute (from pkg import mod): the module path to try second.
	alt string
}

// scanPyMounts finds include_router / register_blueprint calls whose router is
// an imported name or an imported module's attribute.
func scanPyMounts(src string) []pyMountRef {
	cm := newCodeMap(src, pySyntax)
	names := map[string]pyImport{} // local name -> what it was imported as
	for _, m := range pyFromImport.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		module := src[m[2]:m[3]]
		for _, part := range strings.Split(src[m[4]:m[5]], ",") {
			f := strings.Fields(strings.TrimSpace(part))
			switch {
			case len(f) == 1:
				names[f[0]] = pyImport{module: module, name: f[0]}
			case len(f) == 3 && f[1] == "as":
				names[f[2]] = pyImport{module: module, name: f[0]}
			}
		}
	}
	for _, m := range pyPlainImport.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		module := src[m[2]:m[3]]
		local := module
		if m[4] >= 0 {
			local = src[m[4]:m[5]]
		}
		names[local] = pyImport{module: module}
	}

	var out []pyMountRef
	for _, m := range pyMountCallAny.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		argsList := readArgsList(src, m[1]-1)
		args := splitTopLevelArgs(argsList)
		if len(args) == 0 {
			continue
		}
		prefix := ""
		if kw := pyPrefixKw.FindStringSubmatch(argsList); kw != nil {
			prefix = kw[1]
		}
		expr := strings.TrimSpace(args[0])
		if dot := strings.LastIndex(expr, "."); dot > 0 { // items.router, pkg.items.router
			head, attr := expr[:dot], expr[dot+1:]
			imp, ok := names[strings.Split(head, ".")[0]]
			if !ok {
				continue
			}
			module := imp.module
			if imp.name != "" { // from pkg import items  ->  items.router is in pkg.items
				out = append(out, pyMountRef{prefix: prefix, module: imp.module + "." + imp.name, name: attr, alt: imp.module})
				continue
			}
			if rest := strings.TrimPrefix(head, strings.Split(head, ".")[0]); rest != "" {
				module += rest
			}
			out = append(out, pyMountRef{prefix: prefix, module: module, name: attr})
			continue
		}
		if imp, ok := names[expr]; ok && imp.name != "" {
			out = append(out, pyMountRef{prefix: prefix, module: imp.module, name: imp.name})
		}
	}
	return out
}

// pyModuleFile resolves a dotted module (relative if it starts with dots) to a
// scanned project file.
func pyModuleFile(bySlash map[string]string, fromFile, module string) (string, bool) {
	dots := len(module) - len(strings.TrimLeft(module, "."))
	rest := strings.ReplaceAll(strings.TrimLeft(module, "."), ".", "/")
	var bases []string
	if dots > 0 {
		dir := path.Dir(filepath.ToSlash(fromFile))
		for i := 1; i < dots; i++ {
			dir = path.Dir(dir)
		}
		bases = []string{path.Join(dir, rest)}
	} else {
		dir := path.Dir(filepath.ToSlash(fromFile))
		bases = []string{rest, path.Join("src", rest), path.Join(dir, rest)}
	}
	for _, b := range bases {
		for _, cand := range []string{b + ".py", path.Join(b, "__init__.py")} {
			if f, ok := bySlash[cand]; ok {
				return f, true
			}
		}
	}
	return "", false
}

// applyPyMounts puts mount prefixes on the routes of mounted Python routers,
// and clears Unmounted on routers that turned out to be mounted.
func applyPyMounts(g *graph.Graph, mounts map[string][]pyMountRef) {
	bySlash := map[string]string{}
	for _, f := range g.Files {
		bySlash[filepath.ToSlash(f)] = f
	}
	type target struct{ file, router string }
	prefixes := map[target][]string{}
	for from, ms := range mounts {
		for _, m := range ms {
			file, ok := pyModuleFile(bySlash, from, m.module)
			if !ok && m.alt != "" {
				file, ok = pyModuleFile(bySlash, from, m.alt)
			}
			if ok && file != from {
				t := target{file, m.name}
				prefixes[t] = append(prefixes[t], m.prefix)
			}
		}
	}
	if len(prefixes) == 0 {
		return
	}
	nodes := make([]graph.Node, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		ps := prefixes[target{n.File, n.Receiver}]
		if n.Kind != graph.RouteHandler || n.Confidence != graph.Extracted || n.Receiver == "" || len(ps) == 0 {
			nodes = append(nodes, n)
			continue
		}
		for _, p := range ps {
			c := n
			c.Path = joinURL(p, n.Path)
			c.Words = wordsFor(n.File, c.Path)
			c.Unmounted = false
			nodes = append(nodes, c)
		}
	}
	g.Nodes = nodes
}
