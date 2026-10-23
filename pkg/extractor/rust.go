package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// Rust has two dominant web frameworks with genuinely different
// route-registration shapes, so scanRust runs both patterns rather than
// picking one:
//
//   - Actix-web: an attribute macro directly on the handler function,
//     #[get("/path")] / #[post("/path")] / etc. — the verb is the
//     attribute's own name.
//   - Axum: a builder call, .route("/path", get(handler)) — the verb is
//     named by a nested function call inside the argument list instead
//     of the outer method name, a shape none of scanJSLike's patterns
//     match (that shape assumes the verb IS the method name), so it
//     gets its own small extractor rather than being forced through the
//     shared "receiver.Verb(" helper the other new languages reuse.
var (
	actixAttrPattern      = regexp.MustCompile(`#\[(get|post|put|delete|patch)\s*\(`)
	axumRoutePattern      = regexp.MustCompile(`\.route\s*\(`)
	axumVerbInArgsPattern = regexp.MustCompile(`\b(get|post|put|delete|patch)\s*\(`)
)

func scanRust(src string) []callSite {
	cm := newCodeMap(src, rustSyntax)
	var sites []callSite

	for _, m := range actixAttrPattern.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		openParen := m[1] - 1
		sites = append(sites, callSite{
			kind:   graph.RouteHandler,
			method: strings.ToUpper(src[m[2]:m[3]]),
			line:   lineOf(src, m[0]),
			argRaw: readFirstArg(src, openParen),
		})
	}

	for _, m := range axumRoutePattern.FindAllStringIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		openParen := m[1] - 1
		pathArg := readFirstArg(src, openParen)
		argsList := readArgsList(src, openParen)
		// One route can chain several verbs: get(list).post(add). Each is its own
		// registration; none found leaves the method unknown, never guessed.
		seen := map[string]bool{}
		var methods []string
		for _, vm := range axumVerbInArgsPattern.FindAllStringSubmatch(argsList, -1) {
			if v := strings.ToUpper(vm[1]); !seen[v] {
				seen[v] = true
				methods = append(methods, v)
			}
		}
		if len(methods) == 0 {
			methods = []string{""}
		}
		for _, method := range methods {
			sites = append(sites, callSite{
				kind:   graph.RouteHandler,
				method: method,
				line:   lineOf(src, m[0]),
				argRaw: pathArg,
			})
		}
	}

	return sites
}
