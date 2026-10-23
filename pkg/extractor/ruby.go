package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// rubyRoutePattern matches Rails' routes.rb DSL verb calls: get '/path',
// post('/path', ...), etc. — parens are optional in Ruby, so the pattern
// allows either. Centralized in one file (config/routes.rb) rather than
// scattered across handler files the way Express/Flask/Django routes
// are — a genuinely different registration shape from every other
// language this package already supports, not a variant of one.
//
// Stated limitation: `resources :things` (Rails' resourceful-routing
// shorthand, which expands to a set of seven conventional routes) is
// deliberately not expanded here — doing so correctly needs the actual
// pluralization/nesting rules Rails applies, not a regex guess, and a
// wrong expansion would be exactly the kind of confidently-wrong result
// this tool exists to avoid. Only explicit verb calls are extracted.
var rubyRoutePattern = regexp.MustCompile(`(?m)^\s*(get|post|put|patch|delete)\s*\(?\s*['"]([^'"]+)['"]`)

// scanRuby extracts route registrations from a Rails routes.rb file.
// Ruby has no separate outbound-call convention this tool tracks yet
// (no frontend/network-call side) — these languages are backend-only
// additions; the frontend half of a wiring pair still comes from JS/TS.
func scanRuby(src string) []callSite {
	cm := newCodeMap(src, hashSyntax)
	var sites []callSite
	for _, m := range rubyRoutePattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		method := strings.ToUpper(src[m[2]:m[3]])
		path := src[m[4]:m[5]]
		sites = append(sites, callSite{
			kind:   graph.RouteHandler,
			method: method,
			line:   lineOf(src, m[0]),
			// argRaw is quote-wrapped so it flows through the shared
			// literalPath classifier every other language uses — except
			// when the path contains Ruby's own #{...} interpolation,
			// which literalPath has no way to recognize (it only knows
			// JS's ${...} and Python's f-string {...}). Passing the raw,
			// unquoted text through in that case makes literalPath's own
			// "first char must be a quote" check reject it as
			// unresolved — the correct outcome — without teaching a
			// shared, cross-language function a Ruby-specific rule.
			argRaw: rubyArgRaw(path),
		})
	}
	return sites
}

func rubyArgRaw(path string) string {
	if strings.Contains(path, "#{") {
		return path // deliberately unquoted — see scanRuby's comment
	}
	return "\"" + path + "\""
}
