// Package confidence implements the "fake AI confidence" check: does the
// agent's own claim that a feature is done/working actually match the
// wiring check's already-computed, evidence-based verdict for the same
// feature. Same discipline as hero-act: the agent's word is never the
// proof, only ever a claim to be checked against something real.
//
// The claim source is automatic by default — commit messages since
// session start, read the same way ChangedFilesSince reads file history.
// No one has to remember to ask the agent a question and paste the
// answer; the tool only reads what already exists. --claimed-summary
// stays available as an explicit override for anyone who wants to feed
// it something else (or whose commit messages are too terse to carry
// any real claim).
package confidence

import (
	"fmt"
	"os"
	"strings"

	"github.com/LadsonDavid/beta-test/internal/checks/wiring"
	"github.com/LadsonDavid/beta-test/internal/features"
	"github.com/LadsonDavid/beta-test/internal/gitutil"
	"github.com/LadsonDavid/beta-test/internal/session"
)

type Verdict string

// NotClaimed's display string is "NOT CLAIMED" (renamed 2026-09-17),
// deliberately distinct from wiring/contract's "NO PROOF" even though
// both used to print as the same "NOT TESTED" string — they mean
// different things. NO PROOF means "we looked for evidence and found
// none." NOT CLAIMED means something upstream of that: the agent never
// said anything about this feature at all, so there's no claim here to
// even test against the evidence. Reusing one label for both hid that
// distinction from anyone reading the report.
const (
	Confirmed  Verdict = "CONFIRMED"
	Mismatch   Verdict = "CONFIDENCE MISMATCH"
	NotClaimed Verdict = "NOT CLAIMED"
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

// Run checks each feature the plan mentions against the wiring check's
// verdict for that same feature, using whatever claim text it can find.
// summaryPath, if given, is used as-is (explicit override, no session
// needed). If empty, the claim text is read automatically from commit
// messages since session start — this requires a session to have been
// started, same as the other session-scoped checks.
func Run(fs []features.Feature, wiringResults []wiring.Result, root, summaryPath string) Report {
	raw, err := claimText(root, summaryPath)
	if err != nil {
		return Report{Available: false, Reason: err.Error()}
	}
	if raw == "" {
		return Report{Available: true} // nothing claimed anywhere — a valid, honest "nothing to report"
	}
	lines := strings.Split(raw, "\n")

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
				Reason:  "no claim found about this feature — nothing to cross-check",
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
			Reason:    fmt.Sprintf("claimed this works, but the wiring check says %s: %s", wr.Verdict, wr.Reason),
		})
	}

	return Report{Available: true, Results: results}
}

// claimText resolves the raw text to scan for claims: the explicit
// --claimed-summary file if given, otherwise commit messages since
// session start.
func claimText(root, summaryPath string) (string, error) {
	if summaryPath != "" {
		raw, err := os.ReadFile(summaryPath)
		if err != nil {
			return "", fmt.Errorf("couldn't read claimed-summary file: %w", err)
		}
		return string(raw), nil
	}

	st, ok := session.Load(root)
	if !ok {
		return "", fmt.Errorf("no --claimed-summary given, and no session start recorded to read commit messages from — run `malveon session start` before the agent begins its task, or pass --claimed-summary explicitly")
	}
	messages, err := gitutil.CommitMessagesSince(root, st.StartRef)
	if err != nil {
		return "", fmt.Errorf("couldn't read commit messages: %w", err)
	}
	return strings.Join(messages, "\n"), nil
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
