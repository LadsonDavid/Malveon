package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// springMappingPattern matches Spring's method-level HTTP-mapping
// annotations: @GetMapping("/path"), @PostMapping("/path"), etc., plus
// the more generic @RequestMapping("/path") (method left unknown, the
// same "never guess a method from an ambiguous form" rule scanPython
// already applies to Flask's @app.route(...)).
//
// Stated limitation: a class-level @RequestMapping("/prefix") base path
// is not concatenated onto the method-level path — correctly tracking
// which class a given method belongs to needs real class-boundary
// parsing, a materially bigger and more error-prone problem than reading
// one literal string. The method-level path alone is extracted as
// written, which can undercount the real route, but never overclaims a
// wrong combined one — the safe direction of error, same principle
// contract.go's own request-field check already documents for itself.
var springMappingPattern = regexp.MustCompile(`@(Get|Post|Put|Delete|Patch|Request)Mapping\s*\(`)

// scanJava extracts Spring route registrations from Java source.
func scanJava(src string) []callSite {
	cm := newCodeMap(src, cLikeSyn)
	var sites []callSite
	for _, m := range springMappingPattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		openParen := m[1] - 1
		verb := strings.ToUpper(src[m[2]:m[3]])
		if verb == "REQUEST" {
			verb = "" // ambiguous form — method not determinable from the annotation name alone
		}
		sites = append(sites, callSite{
			kind:   graph.RouteHandler,
			method: verb,
			line:   lineOf(src, m[0]),
			argRaw: readFirstArg(src, openParen),
		})
	}
	return sites
}
