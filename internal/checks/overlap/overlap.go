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
//   - reachability is a best-effort heuristic, not a proof: a route whose
//     enclosing named function is never mentioned anywhere else in the
//     codebase gets a "possibly unreachable" note, since Go's AST gives
//     enclosing-function identity for free and JS/Python get it from a
//     brace/indentation-based scan (internal/extractor). Anonymous
//     functions and top-level registrations can't be judged this way and
//     are never flagged — a route registered inside genuinely dead code
//     that this heuristic can't identify still counts as a plain
//     registration, same as before.
package overlap

import (
	"fmt"

	"github.com/LadsonDavid/beta-test/internal/graph"
)

type Finding struct {
	Method string
	Path   string
	Nodes  []graph.Node // every registration claiming this same method+path
	Notes  []string     // parallel to Nodes; "" unless that node looks possibly unreachable
}

func Run(g *graph.Graph, root string) []Finding {
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
			notes := make([]string, len(group))
			for k, n := range group {
				if n.EnclosingFunc != "" && !referencedElsewhere(root, n.EnclosingFunc) {
					notes[k] = fmt.Sprintf("possibly unreachable — handler function %q is never referenced anywhere else in the codebase", n.EnclosingFunc)
				}
			}
			findings = append(findings, Finding{
				Method: routes[i].Method,
				Path:   routes[i].Path,
				Nodes:  group,
				Notes:  notes,
			})
		}
	}
	return findings
}
