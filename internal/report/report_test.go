package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LadsonDavid/beta-test/internal/checks/commands"
	"github.com/LadsonDavid/beta-test/internal/checks/contract"
	"github.com/LadsonDavid/beta-test/internal/checks/uioverlap"
	"github.com/LadsonDavid/beta-test/internal/checks/wiring"
)

// TestWriteWiringGroupsNoProofByReason proves the real fix: many features
// sharing one NO PROOF reason must print that reason once, not once per
// feature — the whole point of the redesign this test protects.
func TestWriteWiringGroupsNoProofByReason(t *testing.T) {
	results := []wiring.Result{
		{FeatureName: "A", Verdict: wiring.NotTested, Reason: "no matching frontend or backend node found for this feature"},
		{FeatureName: "B", Verdict: wiring.NotTested, Reason: "no matching frontend or backend node found for this feature"},
		{FeatureName: "C", Verdict: wiring.Fail, Reason: "matching frontend and backend nodes exist, but no path connects them", Evidence: []string{"a.js:1"}},
		{FeatureName: "D", Verdict: wiring.Pass, Reason: "POST /x reaches a registered route with a matching path", Evidence: []string{"a.js:1", "b.js:2"}},
	}
	var buf bytes.Buffer
	summary := WriteWiring(&buf, results)

	out := buf.String()
	if strings.Count(out, "no matching frontend or backend node found for this feature") != 1 {
		t.Errorf("expected the shared NO PROOF reason to print exactly once, got:\n%s", out)
	}
	if !strings.Contains(out, "- A") || !strings.Contains(out, "- B") {
		t.Errorf("expected both A and B listed under the shared reason, got:\n%s", out)
	}
	if summary.Line != "1 PASS · 1 FAIL · 2 NO PROOF" {
		t.Errorf("unexpected summary line: %q", summary.Line)
	}
}

// TestWriteContractFieldsMismatchIsBadEvenWithMethodMatch proves a real
// bucketing edge case: a method MATCH whose request fields MISMATCH is a
// real contract problem and must land in the full-detail bucket, not get
// buried in the compact MATCH list.
func TestWriteContractFieldsMismatchIsBadEvenWithMethodMatch(t *testing.T) {
	results := []contract.Result{
		{FeatureName: "X", Verdict: contract.Match, Reason: "call uses POST, route is registered for POST",
			BodyVerdict: contract.Mismatch, BodyReason: "handler reads field 'email' the call never sends"},
	}
	var buf bytes.Buffer
	WriteContract(&buf, results)
	out := buf.String()
	if !strings.Contains(out, "MISMATCH (1)") {
		t.Errorf("expected the fields-mismatch item in the MISMATCH bucket, got:\n%s", out)
	}
	if strings.Contains(out, "agree\n") {
		t.Errorf("expected no compact MATCH bucket to print at all (the only item is in MISMATCH), got:\n%s", out)
	}
}

// TestWriteUIOverlapGroupsByFile proves findings in the same file are
// grouped under one file header instead of repeating the risk sentence
// once per finding.
func TestWriteUIOverlapGroupsByFile(t *testing.T) {
	findings := []uioverlap.Finding{
		{File: "Modal.tsx", Line: 10, Classes: "absolute", Reason: "shared reason"},
		{File: "Modal.tsx", Line: 20, Classes: "absolute", Reason: "shared reason"},
		{File: "Other.tsx", Line: 5, Classes: "absolute", Reason: "shared reason"},
	}
	var buf bytes.Buffer
	summary := WriteUIOverlap(&buf, findings)
	out := buf.String()
	if strings.Count(out, "shared reason") != 1 {
		t.Errorf("expected the shared reason sentence to print exactly once, got:\n%s", out)
	}
	if summary.Line != "3 risk across 2 file(s)" {
		t.Errorf("unexpected summary line: %q", summary.Line)
	}
}

// TestWriteCommandsNotRequestedIsDistinctFromNoToolchain proves the two
// different "nothing ran" states print (and summarize) differently: not
// passing the opt-in --exec flag is not the same fact as malveon
// genuinely finding no Makefile/package.json/go.mod/Python manifest
// anywhere in the project.
func TestWriteCommandsNotRequestedIsDistinctFromNoToolchain(t *testing.T) {
	var notRequested, empty bytes.Buffer
	notRequestedSummary := WriteCommands(&notRequested, nil, true)
	emptySummary := WriteCommands(&empty, nil, false)

	if notRequestedSummary.Line == emptySummary.Line {
		t.Errorf("expected distinct summaries for not-requested vs. no toolchain found, both got %q", notRequestedSummary.Line)
	}
	if !strings.Contains(notRequested.String(), "--exec") {
		t.Errorf("expected the not-requested section to mention --exec, got:\n%s", notRequested.String())
	}
}

// TestWriteCommandsGroupsNoProofByReason mirrors the same collapsing
// rule proven above for wiring — many NO PROOF rows sharing one reason
// print that reason once, not once per row.
func TestWriteCommandsGroupsNoProofByReason(t *testing.T) {
	results := []commands.Result{
		{Category: commands.Typecheck, Stack: "Node (npm)", Dir: ".", Verdict: commands.NoProof, Reason: "no \"typecheck\" script in package.json"},
		{Category: commands.Test, Stack: "Node (npm)", Dir: ".", Verdict: commands.NoProof, Reason: "no \"typecheck\" script in package.json"},
		{Category: commands.Build, Stack: "Node (npm)", Dir: ".", Verdict: commands.Fail, Reason: "`npm run build` exited with an error", Output: "line1\nline2"},
		{Category: commands.Lint, Stack: "Node (npm)", Dir: ".", Verdict: commands.Pass, Command: "npm run lint"},
	}
	var buf bytes.Buffer
	summary := WriteCommands(&buf, results, false)
	out := buf.String()

	if strings.Count(out, `no "typecheck" script in package.json`) != 1 {
		t.Errorf("expected the shared NO PROOF reason to print exactly once, got:\n%s", out)
	}
	if !strings.Contains(out, "line1") || !strings.Contains(out, "line2") {
		t.Errorf("expected the FAIL row's output tail printed, got:\n%s", out)
	}
	if summary.Line != "1 PASS · 1 FAIL · 2 NO PROOF" {
		t.Errorf("unexpected summary line: %q", summary.Line)
	}
}

// TestWriteFocusedTestsUnavailableIsSkipped proves the opt-in check's
// "asked for it, involuntarily didn't get it" state renders distinctly
// from a normal empty/clean result, the same SKIPPED convention every
// other session-scoped check in this tool already uses.
func TestWriteFocusedTestsUnavailableIsSkipped(t *testing.T) {
	var buf bytes.Buffer
	summary := WriteFocusedTests(&buf, commands.FocusedReport{Available: false, Reason: "no session start recorded"})
	if summary.Line != "SKIPPED" {
		t.Errorf("unexpected summary line: %q", summary.Line)
	}
	if !strings.Contains(buf.String(), "no session start recorded") {
		t.Errorf("expected the reason printed, got:\n%s", buf.String())
	}
}

func TestWriteFocusedTestsBucketsFailFirst(t *testing.T) {
	results := []commands.Result{
		{Category: commands.FocusedTest, Stack: "Go (package-level, not dependency-graph-aware)", Dir: ".", Verdict: commands.Fail, Reason: "boom", Output: "assertion failed"},
		{Category: commands.FocusedTest, Stack: "Node (Jest, --changedSince)", Dir: "frontend", Verdict: commands.Pass, Command: "jest --changedSince abc"},
	}
	var buf bytes.Buffer
	summary := WriteFocusedTests(&buf, commands.FocusedReport{Available: true, Results: results})
	out := buf.String()
	if !strings.Contains(out, "assertion failed") {
		t.Errorf("expected the FAIL row's output shown, got:\n%s", out)
	}
	if summary.Line != "1 PASS · 1 FAIL · 0 NO PROOF" {
		t.Errorf("unexpected summary line: %q", summary.Line)
	}
}

func TestWriteOverviewPrintsEveryLabel(t *testing.T) {
	var buf bytes.Buffer
	WriteOverview(&buf, []CheckSummary{
		{Label: "WIRING", Line: "1 PASS · 0 FAIL · 0 NO PROOF"},
		{Label: "OVERLAP", Line: "clean"},
	})
	out := buf.String()
	if !strings.Contains(out, "WIRING") || !strings.Contains(out, "OVERLAP") {
		t.Errorf("expected both labels in the overview, got:\n%s", out)
	}
}
