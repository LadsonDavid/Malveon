package confidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LadsonDavid/beta-test/internal/checks/wiring"
	"github.com/LadsonDavid/beta-test/internal/features"
)

func TestRun(t *testing.T) {
	fs := []features.Feature{
		{ID: "refund-button", Name: "refund button"},
		{ID: "cancel-order", Name: "cancel order"},
		{ID: "profile-update", Name: "profile update"},
	}
	wiringResults := []wiring.Result{
		{FeatureID: "refund-button", Verdict: wiring.Pass, Reason: "reaches a registered route"},
		{FeatureID: "cancel-order", Verdict: wiring.Fail, Reason: "no path connects them"},
		{FeatureID: "profile-update", Verdict: wiring.NotTested, Reason: "dynamic URL"},
	}

	summary := "The refund button works perfectly now.\n" +
		"Cancel order is fully functional and ready to ship.\n" +
		"Fixed some logging along the way.\n"
	summaryPath := filepath.Join(t.TempDir(), "summary.txt")
	if err := os.WriteFile(summaryPath, []byte(summary), 0o644); err != nil {
		t.Fatal(err)
	}

	report := Run(fs, wiringResults, summaryPath)
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
		t.Errorf("profile-update: got %s, want %s (never mentioned in the summary)", byID["profile-update"].Verdict, NotClaimed)
	}
}

func TestRunUnavailableWithoutSummary(t *testing.T) {
	report := Run(nil, nil, "")
	if report.Available {
		t.Fatal("expected the check to report itself unavailable with no --claimed-summary given")
	}
}
