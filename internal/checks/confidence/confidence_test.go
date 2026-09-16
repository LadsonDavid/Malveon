package confidence

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LadsonDavid/beta-test/internal/checks/wiring"
	"github.com/LadsonDavid/beta-test/internal/features"
	"github.com/LadsonDavid/beta-test/internal/session"
)

var testFeatures = []features.Feature{
	{ID: "refund-button", Name: "refund button"},
	{ID: "cancel-order", Name: "cancel order"},
	{ID: "profile-update", Name: "profile update"},
}

var testWiring = []wiring.Result{
	{FeatureID: "refund-button", Verdict: wiring.Pass, Reason: "reaches a registered route"},
	{FeatureID: "cancel-order", Verdict: wiring.Fail, Reason: "no path connects them"},
	{FeatureID: "profile-update", Verdict: wiring.NotTested, Reason: "dynamic URL"},
}

func TestRunExplicitSummaryFile(t *testing.T) {
	summary := "The refund button works perfectly now.\n" +
		"Cancel order is fully functional and ready to ship.\n" +
		"Fixed some logging along the way.\n"
	summaryPath := filepath.Join(t.TempDir(), "summary.txt")
	if err := os.WriteFile(summaryPath, []byte(summary), 0o644); err != nil {
		t.Fatal(err)
	}

	report := Run(testFeatures, testWiring, "", summaryPath)
	assertConfidenceVerdicts(t, report)
}

func TestRunUnavailableWithoutSummaryOrSession(t *testing.T) {
	report := Run(nil, nil, t.TempDir(), "")
	if report.Available {
		t.Fatal("expected the check to report itself unavailable with no --claimed-summary and no session")
	}
}

func initTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

func commit(t *testing.T, dir, file, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", message}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestRunAutomaticFromCommitMessages(t *testing.T) {
	dir := initTempRepo(t)
	if _, err := session.Start(dir); err != nil {
		t.Fatalf("session.Start: %v", err)
	}

	commit(t, dir, "b.go", "package a\n", "The refund button works perfectly now.")
	commit(t, dir, "c.go", "package a\n", "Cancel order is fully functional and ready to ship.")
	commit(t, dir, "d.go", "package a\n", "Fixed some logging along the way.")

	report := Run(testFeatures, testWiring, dir, "")
	assertConfidenceVerdicts(t, report)
}

func TestRunAutomaticNoCommits(t *testing.T) {
	dir := initTempRepo(t)
	if _, err := session.Start(dir); err != nil {
		t.Fatalf("session.Start: %v", err)
	}

	report := Run(testFeatures, testWiring, dir, "")
	if !report.Available {
		t.Fatalf("expected available (no commits is a valid empty state, not an error), got: %s", report.Reason)
	}
	if len(report.Results) != 0 {
		t.Fatalf("expected no results with no commits to scan, got %+v", report.Results)
	}
}

func TestRunUnavailableAutomaticWithoutSession(t *testing.T) {
	dir := initTempRepo(t)
	report := Run(testFeatures, testWiring, dir, "")
	if report.Available {
		t.Fatal("expected unavailable: no --claimed-summary and no session started")
	}
}

func assertConfidenceVerdicts(t *testing.T, report Report) {
	t.Helper()
	if !report.Available {
		t.Fatalf("expected the check to be available, got: %s", report.Reason)
	}
	byID := map[string]Result{}
	for _, r := range report.Results {
		byID[r.FeatureID] = r
	}
	if byID["refund-button"].Verdict != Confirmed {
		t.Errorf("refund-button: got %s, want %s", byID["refund-button"].Verdict, Confirmed)
	}
	if byID["cancel-order"].Verdict != Mismatch {
		t.Errorf("cancel-order: got %s, want %s", byID["cancel-order"].Verdict, Mismatch)
	}
	if byID["profile-update"].Verdict != NotClaimed {
		t.Errorf("profile-update: got %s, want %s (never mentioned)", byID["profile-update"].Verdict, NotClaimed)
	}
}
