package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// A link is not a request until a server route answers it. `<a href="/terms">` and
// `a.href = `/api/report/${id}/report.html`` are how a page reaches a download, a report or a
// legal page that the server serves; counting every href would turn links to static files and
// pages elsewhere into calls to nowhere. So these are recorded as links (tplCall.link) and
// applyTemplateCalls keeps one only when some route of this project matches its address.

// .href = "/x"  .href = `/x/${y}/z`   (not ==, not a bare "href" attribute in markup)
var jsHrefAssign = regexp.MustCompile("\\.href\\s*=[^=]\\s*(`[^`\\n]*`|\"[^\"\\n]*\"|'[^'\\n]*')")

// scanJSLinks finds addresses a script assigns to an element's href.
func scanJSLinks(file, src string) []tplCall {
	if !strings.Contains(src, ".href") {
		return nil
	}
	cm := newCodeMap(src, jsSyntax)
	var out []tplCall
	for _, m := range jsHrefAssign.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		p, ok := literalPath(src[m[2]:m[3]])
		if !ok || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") {
			continue
		}
		if i := strings.IndexAny(p, "?#"); i >= 0 {
			p = p[:i]
		}
		out = append(out, tplCall{file: file, line: lineOf(src, m[0]), method: "GET", path: p, link: true})
	}
	return out
}

// someRouteServes: a route of the project answers GET (or any method) at this address.
func someRouteServes(g *graph.Graph, path string) bool {
	for _, n := range g.Nodes {
		if n.Kind != graph.RouteHandler || n.Path == "" {
			continue
		}
		if n.Method != "" && !strings.EqualFold(n.Method, "GET") && !strings.EqualFold(n.Method, "HEAD") {
			continue
		}
		if graph.PathsMatch(path, n.Path) {
			return true
		}
	}
	return false
}
