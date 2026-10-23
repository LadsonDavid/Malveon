package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// Server-rendered pages call the server from their markup: a form's action, an
// htmx attribute (hx-post="/x"), a Django link (`{% url 'name' %}`). These are the
// callers of a Django, Flask, Rails, Laravel or Express app that has no
// JavaScript fetch at all.

var tplExts = map[string]bool{
	".html": true, ".htm": true, ".jinja": true, ".jinja2": true, ".j2": true, ".njk": true,
	".twig": true, ".erb": true, ".ejs": true, ".hbs": true, ".mustache": true,
}

// tplCall is one request a template makes. Exactly one of path (a URL written
// in the markup) and urlName (a Django URL name, resolved once the URLconf is
// read) is set.
type tplCall struct {
	file    string
	line    int
	method  string
	path    string
	urlName string
}

var (
	tplHx         = regexp.MustCompile(`(?is)\bhx-(get|post|put|patch|delete)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	tplFormTag    = regexp.MustCompile(`(?is)<form\b[^>]*>`)
	tplAnchorTag  = regexp.MustCompile(`(?is)<a\b[^>]*>`)
	tplActionAttr = regexp.MustCompile(`(?is)\baction\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	tplMethodAttr = regexp.MustCompile(`(?is)\bmethod\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	tplHrefAttr   = regexp.MustCompile(`(?is)\bhref\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	tplURLTag     = regexp.MustCompile(`^\s*\{%\s*url\s+['"]([\w:.\-]+)['"][^%]*%\}\s*$`)
	tplInterp     = regexp.MustCompile(`^(?:\{\{.*\}\}|\{%.*%\}|<%.*%>|\$\{.*\}|\{\{\{.*\}\}\})$`)
)

func attr(m []string) string {
	if m == nil {
		return ""
	}
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

// scanTemplateCalls finds the requests one template file makes.
func scanTemplateCalls(file, src string) []tplCall {
	var out []tplCall
	add := func(off int, method, value string) {
		if path, name, ok := tplTarget(value); ok {
			out = append(out, tplCall{file: file, line: lineOf(src, off), method: method, path: path, urlName: name})
		}
	}
	for _, m := range tplHx.FindAllStringSubmatchIndex(src, -1) {
		v := ""
		if m[4] >= 0 {
			v = src[m[4]:m[5]]
		} else if m[6] >= 0 {
			v = src[m[6]:m[7]]
		}
		add(m[0], strings.ToUpper(src[m[2]:m[3]]), v)
	}
	for _, m := range tplFormTag.FindAllStringIndex(src, -1) {
		tag := src[m[0]:m[1]]
		action := attr(tplActionAttr.FindStringSubmatch(tag))
		if action == "" {
			continue // posts to the page it is on: not a URL written in the markup
		}
		method := strings.ToUpper(attr(tplMethodAttr.FindStringSubmatch(tag)))
		if method == "" {
			method = "GET"
		}
		add(m[0], method, action)
	}
	// A link is a request only when it names a Django URL; a plain href is navigation.
	for _, m := range tplAnchorTag.FindAllStringIndex(src, -1) {
		href := attr(tplHrefAttr.FindStringSubmatch(src[m[0]:m[1]]))
		if tplURLTag.MatchString(href) {
			add(m[0], "GET", href)
		}
	}
	return out
}

// tplTarget reads the URL a template attribute names: `{% url 'name' %}` (a
// Django URL name) or a path starting with "/" in which a whole segment may be
// filled in by the template engine (`/items/{{ item.id }}/delete`).
func tplTarget(value string) (path, urlName string, ok bool) {
	value = strings.TrimSpace(value)
	if m := tplURLTag.FindStringSubmatch(value); m != nil {
		return "", m[1], true
	}
	if !strings.HasPrefix(value, "/") {
		return "", "", false
	}
	if i := strings.IndexAny(value, "?#"); i >= 0 {
		value = value[:i]
	}
	segs := strings.Split(value, "/")
	for i, seg := range segs {
		switch {
		case seg == "":
		case tplInterp.MatchString(seg):
			segs[i] = ":param"
		case strings.ContainsAny(seg, "{<$"):
			return "", "", false // a value mixed into a segment can't be matched to a route honestly
		}
	}
	return joinURL("", strings.Join(segs, "/")), "", true
}

// applyTemplateCalls adds each template request as a NetworkCall. A Django URL
// name counts only when it names exactly one URL.
func applyTemplateCalls(g *graph.Graph, calls []tplCall, names map[string][]string) {
	for _, c := range calls {
		path := c.path
		if c.urlName != "" {
			paths := names[c.urlName]
			if len(paths) == 0 {
				if i := strings.LastIndexByte(c.urlName, ':'); i >= 0 {
					paths = names[c.urlName[i+1:]] // app_name:name
				}
			}
			paths = distinct(paths)
			if len(paths) != 1 {
				continue
			}
			path = paths[0]
		}
		g.Nodes = append(g.Nodes, graph.Node{
			Kind: graph.NetworkCall, File: c.file, Line: c.line, Method: c.method,
			Path: path, Confidence: graph.Extracted, Words: wordsFor(c.file, path),
		})
	}
}

func distinct(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

var styleBlock = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`)

// blankStyleBlocks replaces the CSS of a single-file component with spaces (keeping
// its line breaks), so a `url("/x")` in a stylesheet is never read as a request
// and every line number stays the same.
func blankStyleBlocks(src string) string {
	return styleBlock.ReplaceAllStringFunc(src, func(block string) string {
		b := []byte(block)
		for i, c := range b {
			if c != '\n' && c != '\r' {
				b[i] = ' '
			}
		}
		return string(b)
	})
}
