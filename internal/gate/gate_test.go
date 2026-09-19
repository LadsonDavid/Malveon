package gate

import (
	"testing"

	"github.com/LadsonDavid/beta-test/internal/checks/commands"
	"github.com/LadsonDavid/beta-test/internal/checks/confidence"
	"github.com/LadsonDavid/beta-test/internal/checks/heropatterns"
	"github.com/LadsonDavid/beta-test/internal/checks/overlap"
	"github.com/LadsonDavid/beta-test/internal/checks/planauthority"
	"github.com/LadsonDavid/beta-test/internal/checks/wiring"
)

func cleanInput() Input {
	return Input{
		PlanAuthority: planauthority.Result{Available: true},
		HeroPatterns:  heropatterns.Report{Available: true},
		Confidence:    confidence.Report{Available: true},
	}
}

func TestEvaluate_CleanInputDoesNotBlock(t *testing.T) {
	d := Evaluate(cleanInput())
	if d.Blocked {
		t.Fatalf("expected a clean run not to block, got reasons: %v", d.Reasons)
	}
}

func TestEvaluate_WiringFailBlocks(t *testing.T) {
	in := cleanInput()
	in.Wiring = []wiring.Result{{FeatureName: "x", Verdict: wiring.Fail}}
	d := Evaluate(in)
	if !d.Blocked {
		t.Fatal("expected a wiring FAIL to block")
	}
}

func TestEvaluate_WiringNoProofBlocks(t *testing.T) {
	in := cleanInput()
	in.Wiring = []wiring.Result{{FeatureName: "x", Verdict: wiring.NotTested}}
	d := Evaluate(in)
	if !d.Blocked {
		t.Fatal("expected a wiring NO PROOF to block — the stricter reading of Feeling_Sun_6436's own ask")
	}
}

func TestEvaluate_WiringPassDoesNotBlock(t *testing.T) {
	in := cleanInput()
	in.Wiring = []wiring.Result{{FeatureName: "x", Verdict: wiring.Pass}}
	d := Evaluate(in)
	if d.Blocked {
		t.Fatalf("expected a wiring PASS not to block, got: %v", d.Reasons)
	}
}

func TestEvaluate_OverlapCollisionBlocks(t *testing.T) {
	in := cleanInput()
	in.Overlap = []overlap.Finding{{Method: "GET", Path: "/x"}}
	d := Evaluate(in)
	if !d.Blocked {
		t.Fatal("expected a route collision to block")
	}
}

func TestEvaluate_UnavailableCheckBlocks(t *testing.T) {
	for name, mutate := range map[string]func(*Input){
		"planauthority": func(in *Input) { in.PlanAuthority = planauthority.Result{Available: false, Reason: "no session"} },
		"heropatterns":  func(in *Input) { in.HeroPatterns = heropatterns.Report{Available: false, Reason: "no session"} },
		"confidence":    func(in *Input) { in.Confidence = confidence.Report{Available: false, Reason: "no session"} },
	} {
		t.Run(name, func(t *testing.T) {
			in := cleanInput()
			mutate(&in)
			d := Evaluate(in)
			if !d.Blocked {
				t.Fatalf("expected an unavailable/SKIPPED %s check to block, per Feeling_Sun_6436's own wording", name)
			}
		})
	}
}

func TestEvaluate_NotInPlanFindingsDoNotBlockAlone(t *testing.T) {
	in := cleanInput()
	in.PlanAuthority = planauthority.Result{
		Available: true,
		Findings:  []planauthority.Finding{{File: "x.js", Line: 1, Kind: "route", Path: "/y"}},
	}
	d := Evaluate(in)
	if d.Blocked {
		t.Fatalf("not-in-plan is a human-review flag, not a verdict — it must not block on its own, got: %v", d.Reasons)
	}
}

func TestEvaluate_HeroPatternsFixedSameSessionDoesNotBlock(t *testing.T) {
	in := cleanInput()
	in.HeroPatterns = heropatterns.Report{
		Available: true,
		Findings:  []heropatterns.Finding{{File: "x.go", Pattern: "empty-error-handling", Status: heropatterns.FixedSameSession}},
	}
	d := Evaluate(in)
	if d.Blocked {
		t.Fatalf("a bug introduced and fixed within the session is not a live problem — must not block, got: %v", d.Reasons)
	}
}

func TestEvaluate_HeroPatternsStillPresentBlocks(t *testing.T) {
	in := cleanInput()
	in.HeroPatterns = heropatterns.Report{
		Available: true,
		Findings:  []heropatterns.Finding{{File: "x.go", Pattern: "empty-error-handling", Status: heropatterns.StillPresent}},
	}
	d := Evaluate(in)
	if !d.Blocked {
		t.Fatal("a bug pattern introduced this session and still present should block")
	}
}

func TestEvaluate_ConfidenceMismatchBlocksButNotClaimedDoesNot(t *testing.T) {
	in := cleanInput()
	in.Confidence = confidence.Report{
		Available: true,
		Results: []confidence.Result{
			{FeatureID: "a", Verdict: confidence.NotClaimed},
			{FeatureID: "b", Verdict: confidence.Confirmed},
		},
	}
	if d := Evaluate(in); d.Blocked {
		t.Fatalf("NOT CLAIMED/CONFIRMED must not block on their own, got: %v", d.Reasons)
	}

	in.Confidence.Results = append(in.Confidence.Results, confidence.Result{FeatureID: "c", Verdict: confidence.Mismatch})
	if d := Evaluate(in); !d.Blocked {
		t.Fatal("a real CONFIDENCE MISMATCH should block")
	}
}

func TestEvaluate_CommandsFailBlocksUnlessNotRequested(t *testing.T) {
	in := cleanInput()
	in.Commands = []commands.Result{{Category: commands.Build, Verdict: commands.Fail}}

	if d := Evaluate(in); !d.Blocked {
		t.Fatal("a real command FAIL should block")
	}

	in.CommandsNotRequested = true
	if d := Evaluate(in); d.Blocked {
		t.Fatalf("not passing --exec is a deliberate opt-in feature not requested — it must never block, got: %v", d.Reasons)
	}
}

func TestEvaluate_CommandsNoProofBlocks(t *testing.T) {
	in := cleanInput()
	in.Commands = []commands.Result{{Category: commands.Test, Verdict: commands.NoProof}}
	if d := Evaluate(in); !d.Blocked {
		t.Fatal("a command check NO PROOF should block — the stricter reading of Feeling_Sun_6436's own ask")
	}
}

func TestEvaluate_FocusedTestsNotRequestedNeverBlocks(t *testing.T) {
	in := cleanInput()
	// Not requested at all — the zero-value FocusedReport is
	// {Available: false}, which would look like a SKIPPED check if
	// FocusedTestsRequested weren't gating it.
	d := Evaluate(in)
	if d.Blocked {
		t.Fatalf("focused tests never requested should never block, got: %v", d.Reasons)
	}
}

func TestEvaluate_FocusedTestsUnavailableWhenRequestedBlocks(t *testing.T) {
	in := cleanInput()
	in.FocusedTestsRequested = true
	in.FocusedTests = commands.FocusedReport{Available: false, Reason: "no session start recorded"}
	d := Evaluate(in)
	if !d.Blocked {
		t.Fatal("explicitly requesting --focused-tests and getting nothing should block, same as any other SKIPPED check")
	}
}

func TestEvaluate_FocusedTestsFailBlocksButNoProofDoesNot(t *testing.T) {
	in := cleanInput()
	in.FocusedTestsRequested = true
	in.FocusedTests = commands.FocusedReport{
		Available: true,
		Results:   []commands.Result{{Category: commands.FocusedTest, Verdict: commands.NoProof}},
	}
	if d := Evaluate(in); d.Blocked {
		t.Fatalf("a focused-test NO PROOF is a best-effort/additive signal — it must not block, got: %v", d.Reasons)
	}

	in.FocusedTests.Results = append(in.FocusedTests.Results, commands.Result{Category: commands.FocusedTest, Verdict: commands.Fail})
	if d := Evaluate(in); !d.Blocked {
		t.Fatal("a real focused-test FAIL is real evidence and should block")
	}
}

func TestEvaluate_EmptyCommandsWithoutSkipDoesNotBlock(t *testing.T) {
	// No toolchain detected at all (e.g. a docs-only repo) is an honest
	// "nothing to run," not a failure to run something — must not block.
	in := cleanInput()
	d := Evaluate(in)
	if d.Blocked {
		t.Fatalf("no detected toolchain should not block, got: %v", d.Reasons)
	}
}
