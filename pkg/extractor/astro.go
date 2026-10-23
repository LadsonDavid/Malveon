package extractor

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// Astro: every file under src/pages is a route by its location — `.astro`, `.md`
// and `.mdx` are pages, `.ts`/`.js` files are endpoints that export one function
// per HTTP method (`GET`, `POST`, ..., `ALL`). A file or folder whose name starts
// with `_` is not a route. Astro Actions (src/actions/index.ts) are POST
// endpoints at /_actions/<name>, called as `actions.<name>()`.

var astroVerbs = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "HEAD": true, "OPTIONS": true, "ALL": true}

func (in *jsInfo) astroRoot(file string) (string, bool) { return in.depDir(file, "astro") }

// astroSegments are the URL segments of a file under src/pages; ok is false
// outside it, outside an Astro project, or for an underscore file.
func astroSegments(rel string, info *jsInfo, endpoint bool) ([]string, bool) {
	root, ok := info.astroRoot(rel)
	if !ok {
		return nil, false
	}
	rest, ok := below(rel, root, "src/pages")
	if !ok {
		return nil, false
	}
	for _, seg := range rest {
		if strings.HasPrefix(seg, "_") {
			return nil, false
		}
	}
	last := rest[len(rest)-1]
	if endpoint {
		last = strings.TrimSuffix(last, filepath.Ext(last)) // sitemap.xml.ts serves /sitemap.xml
	} else {
		last = stemOf(last)
	}
	segs := rest[:len(rest)-1]
	if last != "index" {
		segs = append(segs, last)
	}
	return segs, true
}

// astroPageSite turns an .astro, .md or .mdx file under src/pages into a page.
func astroPageSite(rel string, info *jsInfo) []callSite {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".astro", ".md", ".mdx":
	default:
		return nil
	}
	segs, ok := astroSegments(rel, info, false)
	if !ok {
		return nil
	}
	return routeSites(graph.Page, 1, nil, routePaths(segs, true))
}

// scanAstroEndpoint reads a .ts/.js file under src/pages as an endpoint.
func scanAstroEndpoint(rel, src string, info *jsInfo) []callSite {
	if !serverExts[strings.ToLower(filepath.Ext(rel))] {
		return nil
	}
	segs, ok := astroSegments(rel, info, true)
	if !ok {
		return nil
	}
	urls := routePaths(segs, true)
	var sites []callSite
	for _, v := range exportedNames(src, newCodeMap(src, jsSyntax), astroVerbs) {
		method := v.name
		if method == "ALL" {
			method = ""
		}
		sites = append(sites, routeSites(graph.RouteHandler, v.line, []string{method}, urls)...)
	}
	return sites
}

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

type astroActions struct {
	file  string
	line  int
	names map[string]int // dotted action name -> line
	cands []trpcCand     // actions.a.b(...) calls
}

var (
	astroServerObj = regexp.MustCompile(`export\s+const\s+server\s*(?::[^=]+)?=\s*\{`)
	astroCall      = regexp.MustCompile(`\bactions\s*\.\s*((?:[A-Za-z_$][\w$]*\s*\.\s*)*[A-Za-z_$][\w$]*)\s*\(`)
)

func newAstroActions() *astroActions { return &astroActions{names: map[string]int{}} }

// readFile collects the action definitions of src/actions/index.ts and every
// `actions.x()` call in a file that imports astro:actions.
func (a *astroActions) readFile(rel, src string, info *jsInfo) {
	src = maskComments(src, jsSyntax)
	cm := newCodeMap(src, jsSyntax)
	if strings.Contains(src, "astro:actions") {
		for _, m := range astroCall.FindAllStringSubmatchIndex(src, -1) {
			if cm.codeAt(src, m[0]) {
				a.cands = append(a.cands, trpcCand{file: rel, line: lineOf(src, m[0]), path: strings.ReplaceAll(strings.ReplaceAll(src[m[2]:m[3]], " ", ""), "\n", "")})
			}
		}
	}
	root, ok := info.astroRoot(rel)
	if !ok {
		return
	}
	slash := filepath.ToSlash(rel)
	if root != "." {
		slash = strings.TrimPrefix(slash, root+"/")
	}
	if slash != "src/actions/index.ts" && slash != "src/actions/index.js" && slash != "src/actions.ts" && slash != "src/actions.js" {
		return
	}
	for _, m := range astroServerObj.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		a.file, a.line = rel, lineOf(src, m[0])
		a.readObject(src[m[1]-1:], "", lineOf(src, m[1]-1))
	}
}

// readObject names the defineAction(...) entries of an object, nested objects with dots.
func (a *astroActions) readObject(text, prefix string, baseLine int) {
	obj := readBalanced(text, 0)
	if len(obj) < 2 {
		return
	}
	inner := obj[1 : len(obj)-1]
	for _, e := range spans(inner) {
		key, value, ok := keyValue(strings.TrimSpace(e.text))
		if !ok {
			continue
		}
		key = strings.Trim(key, `'"`)
		line := baseLine + strings.Count(inner[:e.off], "\n")
		switch {
		case strings.HasPrefix(value, "{"):
			a.readObject(value, prefix+key+".", line)
		case strings.Contains(value, "defineAction"):
			a.names[prefix+key] = line
		}
	}
}

// apply adds each action as a POST endpoint and each call to one as a call.
func (a *astroActions) apply(g *graph.Graph) {
	for name, line := range a.names {
		n := graph.Node{Kind: graph.RouteHandler, File: a.file, Line: line, Method: "POST",
			Path: "/_actions/" + name, Confidence: graph.Extracted, RPC: true}
		n.Words = wordsFor(a.file, n.Path)
		g.Nodes = append(g.Nodes, n)
	}
	for _, c := range a.cands {
		if _, ok := a.names[c.path]; !ok {
			continue
		}
		n := graph.Node{Kind: graph.NetworkCall, File: c.file, Line: c.line, Method: "POST",
			Path: "/_actions/" + c.path, Confidence: graph.Extracted, RPC: true}
		n.Words = wordsFor(c.file, n.Path)
		g.Nodes = append(g.Nodes, n)
	}
}
