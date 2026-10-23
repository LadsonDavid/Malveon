package extractor

import (
	"path"
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// Nuxt (and its server engine, Nitro): `server/api/**` is served under /api,
// `server/routes/**` at the root, and `pages/**.vue` are pages. A file name's
// `.get`/`.post`/... suffix limits it to that method. The folders only mean this
// in a project whose package.json lists nuxt.

var (
	nuxtServerFile = regexp.MustCompile(`^(.*)\.(get|post|put|patch|delete|head|options|connect|trace)$`)
	nuxtHandler    = regexp.MustCompile(`\b(?:defineEventHandler|eventHandler|defineCachedEventHandler|defineLazyEventHandler|defineWebSocketHandler|export\s+default)\b`)
	serverExts     = map[string]bool{".ts": true, ".js": true, ".mjs": true, ".mts": true, ".cjs": true}
)

func (in *jsInfo) nuxtRoot(file string) (string, bool) {
	return in.depDir(file, "nuxt", "nuxt3", "nuxt-edge")
}

// scanNuxtServerRoute reads a file under server/api or server/routes.
func scanNuxtServerRoute(rel, src string, info *jsInfo) []callSite {
	if !serverExts[strings.ToLower(path.Ext(rel))] {
		return nil
	}
	root, ok := info.nuxtRoot(rel)
	if !ok {
		return nil
	}
	var segs []string
	if rest, ok := below(rel, root, "server/api", "src", "app"); ok {
		segs = append([]string{"api"}, rest...)
	} else if rest, ok := below(rel, root, "server/routes", "src", "app"); ok {
		segs = rest
	} else {
		return nil
	}
	cm := newCodeMap(src, jsSyntax)
	at := -1
	for _, m := range nuxtHandler.FindAllStringIndex(src, -1) {
		if cm.codeAt(src, m[0]) {
			at = m[0]
			break
		}
	}
	if at < 0 {
		return nil // a helper in the folder, not a handler
	}
	last := stemOf(segs[len(segs)-1])
	segs = segs[:len(segs)-1]
	method := ""
	if m := nuxtServerFile.FindStringSubmatch(last); m != nil {
		last, method = m[1], strings.ToUpper(m[2])
	}
	if last != "index" {
		segs = append(segs, last)
	}
	var methods []string
	if method != "" {
		methods = []string{method}
	}
	return routeSites(graph.RouteHandler, lineOf(src, at), methods, routePaths(segs, false))
}

// nuxtPageSite turns pages/**.vue into pages.
func nuxtPageSite(rel string, info *jsInfo) []callSite {
	if strings.ToLower(path.Ext(rel)) != ".vue" {
		return nil
	}
	root, ok := info.nuxtRoot(rel)
	if !ok {
		return nil
	}
	rest, ok := below(rel, root, "pages", "src", "app")
	if !ok {
		return nil
	}
	last := stemOf(rest[len(rest)-1])
	segs := rest[:len(rest)-1]
	if last != "index" {
		segs = append(segs, last)
	}
	return routeSites(graph.Page, 1, nil, routePaths(segs, true))
}
