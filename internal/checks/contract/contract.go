// Package contract implements the shape-level check from CLAUDE.md
// section 3.2.2. v1 scope is deliberately narrow: HTTP method agreement
// between a wired call and its route (does the frontend actually call
// with the verb the backend registered). That's the shape signal this
// tool can extract and prove correctly today. Full request/response
// body-field comparison is a real, larger feature — not built yet, and
// this package must never claim to check more than it does.
//
// Results here are always labeled CONTRACT MATCH / CONTRACT MISMATCH,
// never CORRECT — this proves the two sides agree on shape, not that
// the business result is right.
package contract

import (
	"fmt"

	"github.com/LadsonDavid/beta-test/internal/features"
	"github.com/LadsonDavid/beta-test/internal/graph"
)

type Verdict string

const (
	Match     Verdict = "CONTRACT MATCH"
	Mismatch  Verdict = "CONTRACT MISMATCH"
	NotTested Verdict = "NOT TESTED"
)

type Result struct {
	FeatureID   string
	FeatureName string
	Verdict     Verdict
	Reason      string
	Evidence    []string
}

// Run checks method agreement only for features the wiring check has
// already proven are connected — a contract can't be evaluated for a
// connection that doesn't demonstrably exist.
func Run(g *graph.Graph, fs []features.Feature) []Result {
	results := make([]Result, 0, len(fs))
	for _, f := range fs {
		results = append(results, evaluate(g, f))
	}
	return results
}

func evaluate(g *graph.Graph, f features.Feature) Result {
	base := Result{FeatureID: f.ID, FeatureName: f.Name}
	pairs := g.MatchedPairs(f.Words())

	if len(pairs) == 0 {
		base.Verdict = NotTested
		base.Reason = "no wired connection found for this feature (see wiring check) — nothing to check the contract of"
		return base
	}

	resolvable := 0
	for _, p := range pairs {
		if p.Call.Method == "" || p.Route.Method == "" {
			continue
		}
		resolvable++
		if p.Call.Method == p.Route.Method {
			base.Verdict = Match
			base.Reason = fmt.Sprintf("call uses %s, route is registered for %s", p.Call.Method, p.Route.Method)
			base.Evidence = []string{
				fmt.Sprintf("%s:%d", p.Call.File, p.Call.Line),
				fmt.Sprintf("%s:%d", p.Route.File, p.Route.Line),
			}
			return base
		}
	}

	if resolvable > 0 {
		p := pairs[0]
		base.Verdict = Mismatch
		base.Reason = fmt.Sprintf("call uses %s but the matching route is registered for %s", p.Call.Method, p.Route.Method)
		base.Evidence = []string{
			fmt.Sprintf("%s:%d", p.Call.File, p.Call.Line),
			fmt.Sprintf("%s:%d", p.Route.File, p.Route.Line),
		}
		return base
	}

	base.Verdict = NotTested
	base.Reason = "HTTP method couldn't be resolved for at least one side of the wired connection"
	return base
}
