// Next.js App Router route handlers, extracted separately from the
// generic JS/TS scan (literal.go): a file literally named route.ts (or
// .tsx/.js/.jsx) exports one function per HTTP method it handles —
// export async function GET(request) {...} — and the URL path comes
// from the file's own location under app/, not from any function
// argument. That's a structurally different registration mechanism
// than Express-style app.get(path, handler), which jsRoutePattern was
// never built to recognize.
//
// Confirmed against a real external test (2026-09-16): without this,
// a real Next.js project's entire backend — 64+ genuinely built API
// routes — silently read as "no matching backend route" on every
// single feature. Not a guess about what might be missing; a real gap
// this tool's own first field test found.
package extractor

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

var (
	nextRouteFilePattern           = regexp.MustCompile(`(?i)(?:^|[/\\])route\.(ts|tsx|js|jsx|mjs)$`)
	nextExportedFuncMethodPattern  = regexp.MustCompile(`export\s+(?:async\s+)?function\s+(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*\(`)
	nextExportedConstMethodPattern = regexp.MustCompile(`export\s+const\s+(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*[:=]`)
)

// IsNextRouteFile reports whether relFile is a Next.js App Router route
// handler file, by its reserved filename — a strong, deliberate
// convention Next.js itself requires, not a heuristic guess.
func IsNextRouteFile(relFile string) bool {
	return nextRouteFilePattern.MatchString(filepath.Base(relFile))
}

// scanNextRouteHandlers extracts one RouteHandler callSite per exported
// HTTP-method function in a Next.js route.ts-style file. Confidence is
// always Extracted: the path comes from the file's real location on
// disk, which is at least as reliable as a literal string argument.
func scanNextRouteHandlers(relFile, src string) []callSite {
	cm := newCodeMap(src, jsSyntax)
	path, ok := nextRoutePath(relFile)
	if !ok {
		return nil
	}
	quoted := `"` + path + `"`

	var sites []callSite
	seen := map[string]bool{}
	for _, pat := range []*regexp.Regexp{nextExportedFuncMethodPattern, nextExportedConstMethodPattern} {
		for _, m := range pat.FindAllStringSubmatchIndex(src, -1) {
			if !cm.codeAt(src, m[0]) {
				continue
			}
			method := src[m[2]:m[3]]
			if seen[method] {
				continue // a file exporting GET both ways would double count otherwise
			}
			seen[method] = true
			site := callSite{
				kind:   graph.RouteHandler,
				method: method,
				line:   lineOf(src, m[0]),
				argRaw: quoted, fileRouted: true,
			}
			// The handler's parameter list: right at the match for
			// `function POST(`, the next "(" for `const POST = async (`.
			paren := m[1] - 1
			if src[paren] != '(' {
				paren = -1
				if i := strings.IndexByte(src[m[1]:], '('); i >= 0 && i < 40 {
					paren = m[1] + i
				}
			}
			if paren >= 0 {
				if fields, ok := nextReadFields(src, paren); ok {
					site.bodyFields = fields
					site.requiredFields = requiredFieldsIn(handlerBodyAt(src, paren), fields)
				}
				site.stub = isStubHandler(handlerBodyAt(src, paren))
			}
			sites = append(sites, site)
		}
	}
	return sites
}

var nextPageFilePattern = regexp.MustCompile(`(?i)^page\.(tsx|jsx|ts|js|mdx)$`)

// nextPageSite turns a Next.js App Router page file (app/**/page.tsx)
// into a Page node served at the same URL rules route handlers use, so a
// plan line naming "/admin/digest" can be checked against a real file.
func nextPageSite(relFile string) (callSite, bool) {
	if !nextPageFilePattern.MatchString(filepath.Base(relFile)) {
		return callSite{}, false
	}
	path, ok := nextRoutePath(relFile)
	if !ok {
		return callSite{}, false
	}
	return callSite{kind: graph.Page, line: 1, argRaw: `"` + path + `"`, fileRouted: true}, true
}

// nextRoutePath derives the URL path Next.js would register for a route
// handler file from its path relative to root: finds the nearest "app"
// directory ancestor (works for both app/ and src/app/ layouts), strips
// it and the trailing route.* filename, drops route-group segments like
// "(app)" (Next.js's own organizational folders — never part of the
// real URL), and normalizes dynamic segments ([id], [...slug],
// [[...slug]]) to the :id wildcard syntax graph.PathsMatch already
// understands, so these routes plug straight into the existing
// wiring/contract/overlap machinery with no changes there.
func nextRoutePath(relFile string) (string, bool) {
	parts := strings.Split(filepath.ToSlash(relFile), "/")
	appIdx := -1
	for i, p := range parts {
		if p == "app" {
			appIdx = i
		}
	}
	if appIdx == -1 || appIdx == len(parts)-1 {
		return "", false
	}
	segs := parts[appIdx+1 : len(parts)-1] // drop through "app/" and the route.* filename itself
	var out []string
	for _, s := range segs {
		if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
			continue
		}
		if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
			// [...slug] keeps its dots (":...slug"), [[...slug]] adds "?": a
			// catch-all matches several segments, not one (graph.PathsMatch).
			inner := strings.Trim(s, "[]")
			if strings.HasPrefix(s, "[[") && strings.HasPrefix(inner, "...") {
				inner += "?"
			}
			out = append(out, ":"+inner)
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return "/", true
	}
	return "/" + strings.Join(out, "/"), true
}

var pagesExts = map[string]bool{".ts": true, ".tsx": true, ".js": true, ".jsx": true}

// nextPagesPath is the URL the Next.js Pages Router serves a file at:
// pages/api/items.ts is /api/items, pages/shop/[id].tsx is /shop/:id,
// index files stand for their folder. api says it is under pages/api.
// ok is false for anything that isn't a routed file (outside pages/, _app,
// _document, tests).
// monorepoAppDirs are the folders whose subfolders are separate apps, each with its own pages/.
var monorepoAppDirs = map[string]bool{"apps": true, "packages": true, "examples": true, "services": true, "projects": true}

func nextPagesPath(relFile string) (path string, api, ok bool) {
	parts := strings.Split(filepath.ToSlash(relFile), "/")
	at := -1
	// pages/ or src/pages/, or one app down in a monorepo: apps/web/pages, apps/web/src/pages.
	for _, i := range []int{0, 1, 2, 3} {
		if i >= len(parts)-1 || parts[i] != "pages" {
			continue
		}
		if i == 0 || i == 1 && parts[0] == "src" || monorepoAppDirs[parts[0]] && (i == 2 || i == 3 && parts[2] == "src") {
			at = i
			break
		}
	}
	if at < 0 {
		return "", false, false
	}
	file := parts[len(parts)-1]
	ext := strings.ToLower(filepath.Ext(file))
	stem := strings.TrimSuffix(file, filepath.Ext(file))
	if !pagesExts[ext] || strings.HasPrefix(stem, "_") || strings.Contains(stem, ".test") || strings.Contains(stem, ".spec") {
		return "", false, false
	}
	segs := append(append([]string{}, parts[at+1:len(parts)-1]...), stem)
	if segs[len(segs)-1] == "index" {
		segs = segs[:len(segs)-1]
	}
	api = len(segs) > 0 && segs[0] == "api"
	var out []string
	for _, s := range segs {
		if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
			inner := strings.Trim(s, "[]")
			if strings.HasPrefix(s, "[[") && strings.HasPrefix(inner, "...") {
				inner += "?"
			}
			s = ":" + inner
		}
		out = append(out, s)
	}
	return "/" + strings.Join(out, "/"), api, true
}

var pagesDefaultExport = regexp.MustCompile(`export\s+default\b`)

// pagesMethodUse finds the HTTP methods a Pages Router handler compares
// req.method against: `req.method === 'GET'`, `method !== 'POST'` (a guard
// that turns every other method away), or `case 'DELETE':` in a switch.
var pagesMethodUse = regexp.MustCompile(`\bmethod\s*(===?|!==?)\s*['"](GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)['"]|\bcase\s*['"](GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)['"]\s*:`)

// pagesHandlerMethods lists the methods a handler serves, in source order:
// the ones it asks for, else — when it only guards with a single `!==` — that
// one. Nothing recognizable returns nil (the handler serves every method).
func pagesHandlerMethods(src string, cm codeMap) []string {
	if !strings.Contains(src, "method") {
		return nil
	}
	seen := map[string]bool{}
	var asked, guarded []string
	for _, m := range pagesMethodUse.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		switch {
		case m[6] >= 0: // case 'X':
			if v := src[m[6]:m[7]]; !seen["="+v] {
				seen["="+v] = true
				asked = append(asked, v)
			}
		case src[m[2]:m[3]] == "!==" || src[m[2]:m[3]] == "!=":
			if v := src[m[4]:m[5]]; !seen["!"+v] {
				seen["!"+v] = true
				guarded = append(guarded, v)
			}
		default:
			if v := src[m[4]:m[5]]; !seen["="+v] {
				seen["="+v] = true
				asked = append(asked, v)
			}
		}
	}
	if len(asked) > 0 {
		return asked
	}
	if len(guarded) == 1 {
		return guarded
	}
	return nil
}

// scanNextPagesRouter: a file under pages/api with a default export is a
// route handler (it branches on req.method itself, so the method is unknown);
// any other routed file under pages/ with one is a page.
func scanNextPagesRouter(relFile, src string, info *jsInfo) []callSite {
	path, api, ok := nextPagesPath(relFile)
	if !ok {
		// pages/ at any depth: the folder of the nearest package.json that depends on next is the app
		if root, found := info.depDir(relFile, "next"); found && root != "." {
			path, api, ok = nextPagesPath(strings.TrimPrefix(filepath.ToSlash(relFile), root+"/"))
		}
	}
	if !ok {
		return nil
	}
	cm := newCodeMap(src, jsSyntax)
	loc := pagesDefaultExport.FindStringIndex(src)
	for loc != nil && !cm.codeAt(src, loc[0]) {
		next := pagesDefaultExport.FindStringIndex(src[loc[1]:])
		if next == nil {
			loc = nil
			break
		}
		loc = []int{loc[1] + next[0], loc[1] + next[1]}
	}
	if loc == nil {
		return nil
	}
	if api {
		// The handler picks the method itself; read which ones it serves. A
		// handler that never looks at the method serves all of them: unknown.
		methods := pagesHandlerMethods(src, cm)
		if len(methods) == 0 {
			methods = []string{""}
		}
		var sites []callSite
		for _, m := range methods {
			sites = append(sites, callSite{kind: graph.RouteHandler, method: m, line: lineOf(src, loc[0]), argRaw: `"` + path + `"`, fileRouted: true})
		}
		return sites
	}
	return []callSite{{kind: graph.Page, line: 1, argRaw: `"` + path + `"`, fileRouted: true}}
}
