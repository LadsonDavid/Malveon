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

	"github.com/LadsonDavid/beta-test/internal/graph"
)

var (
	nextRouteFilePattern           = regexp.MustCompile(`(?i)(?:^|[/\\])route\.(ts|tsx|js|jsx|mjs)$`)
	nextExportedFuncMethodPattern  = regexp.MustCompile(`export\s+(?:async\s+)?function\s+(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*\(`)
	nextExportedConstMethodPattern = regexp.MustCompile(`export\s+const\s+(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*[:=]`)
)

// isNextRouteFile reports whether relFile is a Next.js App Router route
// handler file, by its reserved filename — a strong, deliberate
// convention Next.js itself requires, not a heuristic guess.
func isNextRouteFile(relFile string) bool {
	return nextRouteFilePattern.MatchString(filepath.Base(relFile))
}

// scanNextRouteHandlers extracts one RouteHandler callSite per exported
// HTTP-method function in a Next.js route.ts-style file. Confidence is
// always Extracted: the path comes from the file's real location on
// disk, which is at least as reliable as a literal string argument.
func scanNextRouteHandlers(relFile, src string) []callSite {
	path, ok := nextRoutePath(relFile)
	if !ok {
		return nil
	}
	quoted := `"` + path + `"`

	var sites []callSite
	seen := map[string]bool{}
	for _, pat := range []*regexp.Regexp{nextExportedFuncMethodPattern, nextExportedConstMethodPattern} {
		for _, m := range pat.FindAllStringSubmatchIndex(src, -1) {
			method := src[m[2]:m[3]]
			if seen[method] {
				continue // a file exporting GET both ways would double count otherwise
			}
			seen[method] = true
			sites = append(sites, callSite{
				kind:   graph.RouteHandler,
				method: method,
				line:   lineOf(src, m[0]),
				argRaw: quoted,
			})
		}
	}
	return sites
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
			inner := strings.TrimPrefix(strings.Trim(s, "[]"), "...")
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
