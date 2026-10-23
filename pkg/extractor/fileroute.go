package extractor

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// File-based routing, shared by Nuxt, SvelteKit and Astro: the URL a file serves
// comes from where it sits, and a folder or file name written [id] is a
// parameter. What differs (which folder is the root, what the file names mean) is
// each framework's own file.

// routeSegment turns one folder or file name into a URL segment.
//
//	[id] [id=integer]   -> :id
//	[...slug]           -> :...slug   (one or more segments; none or more when
//	                                   the framework allows an empty rest, restOptional)
//	[[...slug]]         -> :...slug?  (none or more)
//	[[lang]]            -> optional :lang
//	[...]               -> :..._      (Nuxt's unnamed catch-all)
//	(group)             -> nothing    (organizes files, not URLs)
func routeSegment(seg string, restOptional bool) (out string, skip, optional bool) {
	switch {
	case strings.HasPrefix(seg, "(") && strings.HasSuffix(seg, ")"):
		return "", true, false
	case strings.HasPrefix(seg, "[[") && strings.HasSuffix(seg, "]]"):
		inner := seg[2 : len(seg)-2]
		if strings.HasPrefix(inner, "...") {
			return ":" + inner + "?", false, false
		}
		return ":" + inner, false, true
	case strings.HasPrefix(seg, "[") && strings.HasSuffix(seg, "]"):
		inner := seg[1 : len(seg)-1]
		if strings.HasPrefix(inner, "...") && restOptional {
			return ":" + inner + "?", false, false
		}
		if inner == "..." {
			return ":..._", false, false
		}
		if i := strings.IndexByte(inner, '='); i > 0 {
			inner = inner[:i]
		}
		return ":" + inner, false, false
	}
	// Text around a parameter (`@[user]`, `[org]-[repo]`): the URL pattern is
	// :user, not a literal. Matching it as one wildcard segment is the safe
	// direction — it never says a page is missing that exists.
	if i := strings.IndexByte(seg, '['); i >= 0 {
		if j := strings.IndexByte(seg[i:], ']'); j > 0 {
			inner := strings.TrimPrefix(seg[i+1:i+j], "...")
			if k := strings.IndexByte(inner, '='); k > 0 {
				inner = inner[:k]
			}
			return ":" + inner, false, false
		}
	}
	return seg, false, false
}

// routePaths joins file-location segments into the URL or URLs they serve (an
// optional parameter serves two: with it and without it).
func routePaths(segs []string, restOptional bool) []string {
	paths := []string{""}
	for _, s := range segs {
		out, skip, optional := routeSegment(s, restOptional)
		if skip {
			continue
		}
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

// routeSites makes one site per URL per method; "" means the file answers any method.
func routeSites(kind graph.Kind, line int, methods []string, urls []string) []callSite {
	if len(methods) == 0 {
		methods = []string{""}
	}
	var out []callSite
	for _, u := range urls {
		for _, m := range methods {
			out = append(out, callSite{kind: kind, method: m, line: line, argRaw: `"` + u + `"`, fileRouted: true})
		}
	}
	return out
}

// below returns the segments of file under the folder `dir` of a framework root
// ("pages", "server/api"), and ok=false when the file is not inside it. The
// folder has to sit directly in the project root or one of the usual source
// folders, so `components/pages/Card.vue` is not a page.
func below(file, root string, dir string, sourceDirs ...string) (rest []string, ok bool) {
	slash := filepath.ToSlash(file)
	if root != "." {
		slash = strings.TrimPrefix(slash, root+"/")
	}
	for _, base := range append([]string{""}, sourceDirs...) {
		prefix := dir + "/"
		if base != "" {
			prefix = base + "/" + dir + "/"
		}
		if strings.HasPrefix(slash, prefix) {
			return strings.Split(strings.TrimPrefix(slash, prefix), "/"), true
		}
	}
	return nil, false
}

// stemOf is a file name without its extension.
func stemOf(name string) string { return strings.TrimSuffix(name, path.Ext(name)) }

var (
	exportDecl = regexp.MustCompile(`export\s+(?:async\s+)?function\s*\*?\s*([A-Za-z_$][\w$]*)|export\s+(?:const|let|var)\s+([A-Za-z_$][\w$]*)`)
	exportList = regexp.MustCompile(`export\s*\{([^}]*)\}`)
)

type exportAt struct {
	name string
	line int
}

// exportedNames lists, in source order, the names a module exports that are in
// allowed: `export function GET`, `export const GET`, and `export { a as GET }`.
func exportedNames(src string, cm codeMap, allowed map[string]bool) []exportAt {
	var out []exportAt
	seen := map[string]bool{}
	add := func(name string, off int) {
		if allowed[name] && !seen[name] {
			seen[name] = true
			out = append(out, exportAt{name, lineOf(src, off)})
		}
	}
	for _, m := range exportDecl.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		if m[2] >= 0 {
			add(src[m[2]:m[3]], m[0])
		} else {
			add(src[m[4]:m[5]], m[0])
		}
	}
	for _, m := range exportList.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		for _, part := range strings.Split(src[m[2]:m[3]], ",") {
			if f := strings.Fields(part); len(f) > 0 {
				add(f[len(f)-1], m[0]) // `a as GET` exports GET
			}
		}
	}
	return out
}

// hasDefaultExport reports whether the module has `export default` in code.
func hasDefaultExport(src string, cm codeMap) bool {
	for _, m := range pagesDefaultExport.FindAllStringIndex(src, -1) {
		if cm.codeAt(src, m[0]) {
			return true
		}
	}
	return false
}
