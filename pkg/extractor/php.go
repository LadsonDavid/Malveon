package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// laravelRoutePattern matches Laravel's Route facade calls
// (Route::get('/path', ...), Route::post(...), etc.) — the same
// "receiver-separator-VerbName(" shape scanJSLike already extracts for
// Express (app.get(...)), just with PHP's static-call "::" separator
// instead of ".". Argument scanning reuses the exact same
// readFirstArg/lineOf helpers, so a dynamic path built from a variable
// or concatenation is rejected by literalPath exactly the same way.
var laravelRoutePattern = regexp.MustCompile(`(?i)\bRoute::(get|post|put|patch|delete)\s*\(`)

// scanPHP extracts Laravel route registrations from PHP source.
func scanPHP(src string) []callSite {
	cm := newCodeMap(src, phpSyntax)
	var sites []callSite
	for _, m := range laravelRoutePattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		openParen := m[1] - 1
		sites = append(sites, callSite{
			kind:   graph.RouteHandler,
			method: strings.ToUpper(src[m[2]:m[3]]),
			line:   lineOf(src, m[0]),
			argRaw: phpArgRaw(readFirstArg(src, openParen)),
		})
	}
	return sites
}

// phpArgRaw guards against PHP double-quoted string interpolation
// ($var and {$var}) inside a route path — a real gap in the shared
// literalPath classifier, which only recognizes JS's backtick ${...}
// and Python's f-string {...}. PHP interpolates inside a plain "..."
// string, a case literalPath has no way to know about on its own. A
// single-quoted PHP string never interpolates, so it's left untouched.
// Stripping the surrounding quotes on the suspect case makes
// literalPath's own "must start with a quote" check correctly reject it
// as unresolved, without teaching a shared, cross-language function a
// PHP-specific rule.
func phpArgRaw(raw string) string {
	if len(raw) >= 2 && raw[0] == '"' && strings.Contains(raw, "$") {
		return raw[1 : len(raw)-1]
	}
	return raw
}
