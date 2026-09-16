// Package overlap implements the duplicate-route-registration check.
// This is not Fowler's "Duplicated Code" smell (two chunks of logic doing
// the same computation) — it's route collision: two different handlers
// both claiming the same method+path, where only one of them will ever
// actually run, and which one depends on registration order/framework
// internals the tool has no visibility into. Scoped deliberately narrow
// to avoid false positives:
//   - only Extracted (literal) paths are compared — never guess whether
//     a dynamic path collides with anything.
//   - only routes with a known method are compared — a same-path,
//     different-method pair (GET /refund vs POST /refund) is two
//     legitimate routes, not a conflict.
//   - reachability isn't checked — a route registered inside code that's
//     never actually called still counts as a registration here. That's
//     a stated limitation, not an oversight.
package overlap

import "github.com/LadsonDavid/beta-test/internal/graph"

type Finding struct {
	Method string
	Path   string
	Nodes  []graph.Node // every registration claiming this same method+path
}

func Run(g *graph.Graph) []Finding {
	var routes []graph.Node
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && n.Confidence == graph.Extracted && n.Method != "" {
			routes = append(routes, n)
		}
	}

	used := make([]bool, len(routes))
	var findings []Finding
	for i := range routes {
		if used[i] {
			continue
		}
		group := []graph.Node{routes[i]}
		for j := i + 1; j < len(routes); j++ {
			if used[j] {
				continue
			}
			if routes[i].Method == routes[j].Method && graph.PathsMatch(routes[i].Path, routes[j].Path) {
				group = append(group, routes[j])
				used[j] = true
			}
		}
		if len(group) > 1 {
			findings = append(findings, Finding{
				Method: routes[i].Method,
				Path:   routes[i].Path,
				Nodes:  group,
			})
		}
	}
	return findings
}
