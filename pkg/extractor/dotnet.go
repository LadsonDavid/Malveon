package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// aspnetMinimalPattern matches ASP.NET Core's Minimal API route
// registrations (app.MapGet("/path", handler), app.MapPost(...), etc.)
// — Microsoft's current recommended default style since .NET 6, and the
// same "receiver.VerbName(" shape scanJSLike already extracts for
// Express, just with .NET's "Map"-prefixed, capitalized verb names.
//
// Stated limitation: attribute routing ([HttpGet("/path")] on a
// controller action, combined with a class-level [Route("api/[controller]")]
// template) is deliberately not extracted here. Correctly resolving it
// needs real [controller]/[action] token substitution against the
// controller's own class name — a materially different, more complex
// problem than reading a literal string, and a wrong substitution would
// be exactly the confidently-wrong result this tool exists to avoid.
// Minimal API covers the modern default; attribute routing is real,
// scoped future work once it can be done honestly.
var aspnetMinimalPattern = regexp.MustCompile(`\b\w+\.Map(Get|Post|Put|Delete|Patch)\s*\(`)

// scanDotnet extracts ASP.NET Core Minimal API route registrations from
// C# source.
func scanDotnet(src string) []callSite {
	cm := newCodeMap(src, cLikeSyn)
	var sites []callSite
	for _, m := range aspnetMinimalPattern.FindAllStringSubmatchIndex(src, -1) {
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
	return sites
}
