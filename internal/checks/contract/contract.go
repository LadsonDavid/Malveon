// Package contract implements the shape-level check from CLAUDE.md
// section 3.2.2: HTTP method agreement between a wired call and its
// route (does the frontend actually call with the verb the backend
// registered), plus a real, narrower slice of body-field agreement —
// does every field the route handler reads off req.body actually get
// sent by the call. Field-shape is JS/TS-only and only ever checked when
// both sides resolved to a plain literal object (see internal/extractor/
// bodyshape.go); a variable, spread, or any other language reports its
// own NO PROOF rather than guessing. Full request/response body-field
// comparison beyond that (Python/Go coverage, response-shape checking,
// error-branch checking) stays real, larger future work.
//
// Results here are always labeled CONTRACT MATCH / CONTRACT MISMATCH,
// never CORRECT — this proves the two sides agree on shape, not that
// the business result is right.
package contract

import (
	"fmt"
	"sort"
	"strings"

	"github.com/LadsonDavid/beta-test/internal/features"
	"github.com/LadsonDavid/beta-test/internal/graph"
)

type Verdict string

const (
	Match     Verdict = "CONTRACT MATCH"
	Mismatch  Verdict = "CONTRACT MISMATCH"
	NotTested Verdict = "NO PROOF"
)

type Result struct {
	FeatureID   string
	FeatureName string
	Verdict     Verdict
	Reason      string
	Evidence    []string

	BodyVerdict Verdict // request-field agreement; always its own verdict, never folded into Verdict above
	BodyReason  string
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
		base.BodyVerdict = NotTested
		base.BodyReason = "no wired connection found for this feature"
		return base
	}

	base.BodyVerdict, base.BodyReason = evaluateBody(pairs)

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

// evaluateBody checks, for the first pair where both sides resolved a
// literal request-body shape, whether every field the route handler
// reads off req.body is actually among the fields the call sends. It
// never flags the reverse (frontend sends more than the backend reads) —
// that's not a bug, the handler is just allowed to ignore extra fields.
func evaluateBody(pairs []graph.Pair) (Verdict, string) {
	for _, p := range pairs {
		if p.Call.BodyFields == nil || p.Route.BodyFields == nil {
			continue
		}
		sent := make(map[string]bool, len(p.Call.BodyFields))
		for _, f := range p.Call.BodyFields {
			sent[f] = true
		}
		var missing []string
		for _, f := range p.Route.BodyFields {
			if !sent[f] {
				missing = append(missing, f)
			}
		}
		if len(missing) == 0 {
			return Match, fmt.Sprintf("every field the handler reads off req.body (%s) is sent by the call", describeFields(p.Route.BodyFields))
		}
		sort.Strings(missing)
		return Mismatch, fmt.Sprintf("handler reads %s off req.body, but the call never sends %s", describeFields(p.Route.BodyFields), describeFields(missing))
	}
	return NotTested, "request body shape couldn't be resolved statically on at least one side (not a literal object, or not JS/TS)"
}

func describeFields(fields []string) string {
	if len(fields) == 0 {
		return "no fields"
	}
	return strings.Join(fields, ", ")
}
