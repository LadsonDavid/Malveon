// Package wiring implements the one check malveon check started with:
// does a frontend action that matches a feature name actually reach a
// real backend route. See CLAUDE.md section 3.2.1 for the spec this
// implements — this file should never drift from it without both being
// updated together.
package wiring

import (
	"fmt"

	"github.com/LadsonDavid/beta-test/internal/features"
	"github.com/LadsonDavid/beta-test/internal/graph"
)

type Verdict string

const (
	Pass      Verdict = "PASS"
	Fail      Verdict = "FAIL"
	NotTested Verdict = "NOT TESTED"
)

type Result struct {
	FeatureID   string
	FeatureName string
	Verdict     Verdict
	Reason      string
	Evidence    []string // "file:line" entries backing the verdict
}

// Run evaluates every feature against the graph and returns one Result
// per feature, in the same order features were given.
func Run(g *graph.Graph, fs []features.Feature) []Result {
	results := make([]Result, 0, len(fs))
	for _, f := range fs {
		results = append(results, evaluate(g, f))
	}
	return results
}

func evaluate(g *graph.Graph, f features.Feature) Result {
	words := f.Words()
	calls := g.FindNodesMatching(graph.NetworkCall, words)
	routes := g.FindNodesMatching(graph.RouteHandler, words)

	base := Result{FeatureID: f.ID, FeatureName: f.Name}

	if len(calls) == 0 || len(routes) == 0 {
		base.Verdict = NotTested
		base.Reason = "no matching frontend and/or backend node found for this feature"
		return base
	}

	// A pair here is the only thing this check is allowed to call PASS —
	// same shared definition of "connected" the contract check builds on.
	if pairs := g.MatchedPairs(words); len(pairs) > 0 {
		p := pairs[0]
		base.Verdict = Pass
		base.Reason = fmt.Sprintf("%s %s reaches a registered route with a matching path", callLabel(p.Call), p.Route.Path)
		base.Evidence = []string{
			fmt.Sprintf("%s:%d", p.Call.File, p.Call.Line),
			fmt.Sprintf("%s:%d", p.Route.File, p.Route.Line),
		}
		return base
	}

	// No literal match. If every candidate on either side is a real
	// literal path, the mismatch is a real FAIL, not an unknown.
	if allExtracted(calls) && allExtracted(routes) {
		base.Verdict = Fail
		base.Reason = "matching frontend and backend nodes exist, but no path connects them"
		base.Evidence = evidenceFor(calls, routes)
		return base
	}

	base.Verdict = NotTested
	base.Reason = "path couldn't be resolved statically for at least one matching node (dynamic URL, variable, or template)"
	base.Evidence = evidenceFor(calls, routes)
	return base
}

func allExtracted(nodes []graph.Node) bool {
	for _, n := range nodes {
		if n.Confidence != graph.Extracted {
			return false
		}
	}
	return true
}

func evidenceFor(calls, routes []graph.Node) []string {
	var ev []string
	for _, c := range calls {
		ev = append(ev, fmt.Sprintf("%s:%d", c.File, c.Line))
	}
	for _, r := range routes {
		ev = append(ev, fmt.Sprintf("%s:%d", r.File, r.Line))
	}
	return ev
}

func callLabel(n graph.Node) string {
	if n.Method != "" {
		return n.Method
	}
	return "call"
}
