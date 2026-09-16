// Package confidence implements the "fake AI confidence" check: the
// agent's own plain-text summary claims certain features are done/working,
// and this cross-references that claim against the wiring check's already
// -computed, evidence-based verdict for the same feature. Same discipline
// as hero-act: the agent's word is never the proof, only ever a claim to
// be checked against something real.
package confidence

import (
	"fmt"
	"os"
	"strings"

	"github.com/LadsonDavid/beta-test/internal/checks/wiring"
	"github.com/LadsonDavid/beta-test/internal/features"
)

type Verdict string

const (
	Confirmed  Verdict = "CONFIRMED"
	Mismatch   Verdict = "CONFIDENCE MISMATCH"
	NotClaimed Verdict = "NOT TESTED"
)

type Result struct {
	FeatureID   string
	FeatureName string
	Verdict     Verdict
	ClaimLine   string
	Reason      string
}

type Report struct {
	Available bool
	Reason    string
	Results   []Result
}

var confidencePhrases = []string{
	"works", "working", "done", "complete", "completed",
	"fully functional", "perfectly", "ready", "finished", "good to go",
}

// Run reads summaryPath — the agent's own plain-text claim about what it
// built — and checks each feature the plan mentions against the wiring
// check's verdict for that same feature.
func Run(fs []features.Feature, wiringResults []wiring.Result, summaryPath string) Report {
	if summaryPath == "" {
		return Report{Available: false, Reason: "no --claimed-summary file given"}
	}
	raw, err := os.ReadFile(summaryPath)
	if err != nil {
		return Report{Available: false, Reason: fmt.Sprintf("couldn't read claimed-summary file: %v", err)}
	}
	lines := strings.Split(string(raw), "\n")

	wiringByID := make(map[string]wiring.Result, len(wiringResults))
	for _, r := range wiringResults {
		wiringByID[r.FeatureID] = r
	}

	var results []Result
	for _, f := range fs {
		claimLine, claimed := findClaim(lines, f.Words())
		if !claimed {
			results = append(results, Result{
				FeatureID: f.ID, FeatureName: f.Name,
				Verdict: NotClaimed,
				Reason:  "the summary doesn't claim anything about this feature — nothing to cross-check",
			})
			continue
		}

		wr, known := wiringByID[f.ID]
		if !known || wr.Verdict == wiring.Pass {
			verdict := Confirmed
			reason := "wiring check agrees this feature actually reaches a real backend route"
			if !known {
				verdict, reason = NotClaimed, "feature isn't in the wiring check results to cross-reference against"
			}
			results = append(results, Result{
				FeatureID: f.ID, FeatureName: f.Name, Verdict: verdict,
				ClaimLine: claimLine, Reason: reason,
			})
			continue
		}

		results = append(results, Result{
			FeatureID: f.ID, FeatureName: f.Name, Verdict: Mismatch,
			ClaimLine: claimLine,
			Reason:    fmt.Sprintf("summary claims this works, but the wiring check says %s: %s", wr.Verdict, wr.Reason),
		})
	}

	return Report{Available: true, Results: results}
}

func findClaim(lines []string, featureWords []string) (string, bool) {
	for _, line := range lines {
		lower := strings.ToLower(line)
		if !containsAny(lower, featureWords) {
			continue
		}
		if containsAny(lower, confidencePhrases) {
			return strings.TrimSpace(line), true
		}
	}
	return "", false
}

func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if n != "" && strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}
