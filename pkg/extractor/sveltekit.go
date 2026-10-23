package extractor

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// SvelteKit: `src/routes/<url>/+server.ts` exports one function per HTTP method,
// `+page.svelte` is the page at that URL, and `+page.server.ts` can export
// `actions` that the page's own `<form method="POST">` posts to. Folder names
// work like Next.js ([id], [...rest], [[optional]], (group)). The folders only
// mean this in a project whose package.json lists @sveltejs/kit.

var (
	svelteActions = regexp.MustCompile(`export\s+(?:const|let)\s+actions\b`)
	svelteVerbs   = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "OPTIONS": true, "HEAD": true, "QUERY": true, "fallback": true}
	formActionAny = regexp.MustCompile(`(?is)\baction\s*=`)
)

func (in *jsInfo) svelteRoot(file string) (string, bool) { return in.depDir(file, "@sveltejs/kit") }

// svelteURLs is the URL (or URLs, for an optional parameter) the route folder of
// file serves; ok is false outside src/routes of a SvelteKit project.
func svelteURLs(rel string, info *jsInfo) ([]string, bool) {
	root, ok := info.svelteRoot(rel)
	if !ok {
		return nil, false
	}
	rest, ok := below(rel, root, "src/routes")
	if !ok {
		return nil, false
	}
	return routePaths(rest[:len(rest)-1], true), true
}

// scanSvelteKitServer reads +server.ts (endpoints) and +page.server.ts (form actions).
func scanSvelteKitServer(rel, src string, info *jsInfo) []callSite {
	base := filepath.Base(rel)
	isServer := base == "+server.ts" || base == "+server.js"
	isPageServer := base == "+page.server.ts" || base == "+page.server.js"
	if !isServer && !isPageServer {
		return nil
	}
	urls, ok := svelteURLs(rel, info)
	if !ok {
		return nil
	}
	cm := newCodeMap(src, jsSyntax)
	if isPageServer {
		for _, m := range svelteActions.FindAllStringIndex(src, -1) {
			if cm.codeAt(src, m[0]) {
				return routeSites(graph.RouteHandler, lineOf(src, m[0]), []string{"POST"}, urls)
			}
		}
		return nil
	}
	verbs := exportedNames(src, cm, svelteVerbs)
	var sites []callSite
	for _, v := range verbs {
		method := v.name
		if method == "fallback" {
			method = "" // any method without its own export
		}
		sites = append(sites, routeSites(graph.RouteHandler, v.line, []string{method}, urls)...)
	}
	return sites
}

// svelteKitPage turns +page.svelte into a page, and finds the forms in it that
// post to the page's own actions.
func svelteKitPage(rel, src string, info *jsInfo) []callSite {
	if filepath.Base(rel) != "+page.svelte" {
		return nil
	}
	urls, ok := svelteURLs(rel, info)
	if !ok {
		return nil
	}
	sites := routeSites(graph.Page, 1, nil, urls)
	for _, m := range tplFormTag.FindAllStringIndex(src, -1) {
		tag := src[m[0]:m[1]]
		if !strings.EqualFold(attr(tplMethodAttr.FindStringSubmatch(tag)), "post") {
			continue
		}
		action := attr(tplActionAttr.FindStringSubmatch(tag))
		if action == "" && formActionAny.MatchString(tag) {
			continue // action={something}: not a URL malveon can read
		}
		if action == "" || strings.HasPrefix(action, "?") {
			for _, s := range routeSites(graph.NetworkCall, lineOf(src, m[0]), []string{"POST"}, urls) {
				s.fileRouted = false
				sites = append(sites, s)
			}
		}
	}
	return sites
}
