// Package gate implements malveon check's exit-code decision — CLAUDE.md
// 3.2.11's answer to the second real gap Feeling_Sun_6436 (the first
// real tester) named directly: "make failed or skipped checks block the
// commit." Before this package existed, nothing about a `malveon check`
// run stopped a commit, no matter what it found.
//
// The rule implemented here is the stricter reading the founder chose:
// block on anything actually proven broken (a real FAIL, a proven
// contract mismatch, a real route collision, a bug pattern still
// present), on anything left genuinely unresolved by a check whose whole
// job is "prove it or refuse to guess" (wiring, contract, the command
// check), and on any check that couldn't run at all — a SKIPPED check is
// exactly the "skipped checks block the commit" half of the same
// sentence.
//
// Deliberately excluded, because blocking on them would itself be an
// overclaim of certainty this tool doesn't have:
//   - not-in-plan findings — CLAUDE.md 3.2.3 is explicit that this is a
//     human-review flag, not a verdict on whether the code should exist.
//   - incompleteness markers and frontend-overlap-risk findings — both
//     are documented, one-directional structural-risk signals, never a
//     proof of a real problem (3.2.8, 3.2.10).
//   - CONFIRMED and NOT CLAIMED from the confidence check — neither is a
//     contradiction of anything.
//   - the command check's results when --skip-exec was passed — a
//     deliberate opt-out by the tester is not an involuntary "couldn't
//     run" the way a missing session is for the other session-scoped
//     checks; blocking on it would defeat the flag's own purpose.
package gate

import (
	"fmt"

	"github.com/LadsonDavid/beta-test/internal/checks/commands"
	"github.com/LadsonDavid/beta-test/internal/checks/confidence"
	"github.com/LadsonDavid/beta-test/internal/checks/contract"
	"github.com/LadsonDavid/beta-test/internal/checks/heropatterns"
	"github.com/LadsonDavid/beta-test/internal/checks/overlap"
	"github.com/LadsonDavid/beta-test/internal/checks/planauthority"
	"github.com/LadsonDavid/beta-test/internal/checks/wiring"
)

// Input bundles every already-computed check result Evaluate needs. It's
// built from the exact same values the report package renders, so the
// exit code and the printed report can never disagree about what
// actually happened this run.
type Input struct {
	Wiring   []wiring.Result
	Contract []contract.Result
	Overlap  []overlap.Finding

	PlanAuthority planauthority.Result
	HeroPatterns  heropatterns.Report
	Confidence    confidence.Report

	// Commands is ignored when CommandsSkipped is true (the tester
	// passed --skip-exec).
	Commands        []commands.Result
	CommandsSkipped bool
}

// Decision is whether malveon check should exit non-zero, and why, in
// plain language a tester can read straight off the terminal without
// opening the full report above it.
type Decision struct {
	Blocked bool
	Reasons []string
}

// Evaluate applies the rule described in the package doc comment.
func Evaluate(in Input) Decision {
	var reasons []string

	if fail, noProof := countWiring(in.Wiring); fail+noProof > 0 {
		reasons = append(reasons, fmt.Sprintf("buttons reach the backend: %d FAIL, %d NO PROOF", fail, noProof))
	}
	if mismatch, noProof := countContract(in.Contract); mismatch+noProof > 0 {
		reasons = append(reasons, fmt.Sprintf("frontend & backend agree on data: %d MISMATCH, %d NO PROOF", mismatch, noProof))
	}
	if len(in.Overlap) > 0 {
		reasons = append(reasons, fmt.Sprintf("backend route conflicts: %d collision(s)", len(in.Overlap)))
	}

	if !in.PlanAuthority.Available {
		reasons = append(reasons, "unplanned-code check couldn't run: "+in.PlanAuthority.Reason)
	}

	if !in.HeroPatterns.Available {
		reasons = append(reasons, "new-bugs-this-session check couldn't run: "+in.HeroPatterns.Reason)
	} else if n := countStillPresent(in.HeroPatterns); n > 0 {
		reasons = append(reasons, fmt.Sprintf("new bugs this session: %d known bug pattern(s) introduced and still present", n))
	}

	if !in.Confidence.Available {
		reasons = append(reasons, "agent's-claims-vs-reality check couldn't run: "+in.Confidence.Reason)
	} else if n := countConfidenceMismatch(in.Confidence); n > 0 {
		reasons = append(reasons, fmt.Sprintf("agent's claims vs reality: %d claim(s) contradicted by what was actually verified", n))
	}

	if !in.CommandsSkipped {
		if fail, noProof := countCommands(in.Commands); fail+noProof > 0 {
			reasons = append(reasons, fmt.Sprintf("build/lint/test actually ran: %d FAIL, %d NO PROOF", fail, noProof))
		}
	}

	return Decision{Blocked: len(reasons) > 0, Reasons: reasons}
}

func countWiring(results []wiring.Result) (fail, noProof int) {
	for _, r := range results {
		switch r.Verdict {
		case wiring.Fail:
			fail++
		case wiring.NotTested:
			noProof++
		}
	}
	return
}

// countContract mirrors the report package's own bucketing exactly: a
// request-field MISMATCH counts even when the method itself MATCHed
// (report.go's `isBad`), and an unresolved method (NotTested) counts as
// NO PROOF only when it isn't already counted as a mismatch.
func countContract(results []contract.Result) (mismatch, noProof int) {
	for _, r := range results {
		switch {
		case r.Verdict == contract.Mismatch || r.BodyVerdict == contract.Mismatch:
			mismatch++
		case r.Verdict == contract.NotTested:
			noProof++
		}
	}
	return
}

func countStillPresent(r heropatterns.Report) int {
	n := 0
	for _, f := range r.Findings {
		if f.Status == heropatterns.StillPresent {
			n++
		}
	}
	return n
}

func countConfidenceMismatch(r confidence.Report) int {
	n := 0
	for _, res := range r.Results {
		if res.Verdict == confidence.Mismatch {
			n++
		}
	}
	return n
}

func countCommands(results []commands.Result) (fail, noProof int) {
	for _, r := range results {
		switch r.Verdict {
		case commands.Fail:
			fail++
		case commands.NoProof:
			noProof++
		}
	}
	return
}
