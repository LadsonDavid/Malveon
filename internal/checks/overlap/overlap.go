// Package overlap implements the duplicate-route-registration check.
// This is not Fowler's "Duplicated Code" smell (two chunks of logic doing
// the same computation) — it's route collision: two different handlers
// both claiming the same method+path, where only one of them will ever
// actually run. Scoped deliberately narrow to avoid false positives:
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
//
// Route-precedence awareness (added 2026-09-17, real research behind
// it — see progress.md's log). "Which one of two colliding routes
// actually wins" is not one universal answer; it genuinely depends on
// the router:
//   - Next.js App Router (file-based routing) and Go's stdlib
//     net/http.ServeMux (1.22+) / chi resolve by *specificity*: a
//     static segment always beats a dynamic one, deterministically,
//     regardless of file or registration order. A static/dynamic pair
//     here is never a real bug.
//   - Express, Flask, FastAPI, and gorilla/mux resolve by
//     *registration order*: whichever route is registered first wins.
//     A dynamic route registered *before* a static one genuinely makes
//     the static route permanently unreachable — a real bug.
//
// Malveon can only confidently name a route as Next.js App Router
// (IsNextRouteFile, a deliberate, reserved filename — a strong
// signal). It cannot yet tell gorilla/mux from chi, or Express from a
// hypothetical order-independent router, so route pairs outside the
// confirmed Next.js case keep the conservative original behavior:
// flagged, unless both are in the same file with the static one
// registered first — the one ordering that's safe under every
// researched framework, order-dependent or not.
package overlap

import (
	"fmt"

	"github.com/LadsonDavid/beta-test/internal/extractor"
	"github.com/LadsonDavid/beta-test/internal/graph"
)

type Finding struct {
	Method string
	Path   string
	Nodes  []graph.Node // every registration claiming this same method+path
	Notes  []string     // parallel to Nodes; "" unless that node looks possibly unreachable
}

// isSafePair reports whether a colliding group is a static-vs-dynamic
// pair that's provably not a real bug — see the package doc comment for
// the research behind each branch. Only handles the common two-node
// case; groups of 3+ colliding routes stay flagged regardless, since
// that shape is unusual enough to always deserve a human look.
func isSafePair(group []graph.Node) bool {
	if len(group) != 2 {
		return false
	}
	a, b := group[0], group[1]
	aStatic, bStatic := graph.IsFullyStatic(a.Path), graph.IsFullyStatic(b.Path)
	if aStatic == bStatic {
		return false // both static (genuine duplicate) or both dynamic - always worth a look
	}

	// Next.js App Router: file-based, specificity-resolved, deterministic
	// regardless of file order. A static/dynamic split between two
	// route.ts files is never ambiguous.
	if extractor.IsNextRouteFile(a.File) && extractor.IsNextRouteFile(b.File) {
		return true
	}

	// Same file, static route registered before the dynamic one: safe
	// under every router researched, order-dependent or not (Express,
	// Flask, FastAPI, gorilla/mux all resolve this correctly when
	// written in this order; specificity-based routers don't care about
	// order at all). The reverse order — dynamic before static — stays
	// flagged, since that's the one shape genuinely known to break
	// order-dependent routers.
	if a.File == b.File {
		static, dynamic := a, b
		if bStatic {
			static, dynamic = b, a
		}
		return static.Line < dynamic.Line
	}

	// Different files, not Next.js: no reliable way to know which
	// mounts first, and the router library itself is unknown — stay
	// conservative and keep flagging.
	return false
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
		if len(group) > 1 && !isSafePair(group) {
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
