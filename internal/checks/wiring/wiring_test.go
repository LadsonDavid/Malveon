package wiring

import (
	"testing"

	"github.com/LadsonDavid/beta-test/internal/extractor"
	"github.com/LadsonDavid/beta-test/internal/features"
)

// TestFixtureEndToEnd is the one runnable check for the wiring logic: it
// runs the real extractor against testdata/fixture and asserts the three
// verdicts (PASS, FAIL, NOT TESTED) it was deliberately built to exercise.
// If this ever goes green for the wrong reason, the fixture or the logic
// has drifted — treat a change here as a signal to re-read both.
func TestFixtureEndToEnd(t *testing.T) {
	g, err := extractor.Extract("../../../testdata/fixture")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	fs, err := features.Load("../../../testdata/fixture/features.json")
	if err != nil {
		t.Fatalf("load features: %v", err)
	}

	results := Run(g, fs)
	want := map[string]Verdict{
		"refund-button":   Pass,
		"cancel-order":    Fail,
		"profile-update":  NotTested,
		"update-settings": Pass, // wiring only checks the path connects, not the method
	}

	got := map[string]Verdict{}
	for _, r := range results {
		got[r.FeatureID] = r.Verdict
	}

	for id, wantVerdict := range want {
		if got[id] != wantVerdict {
			t.Errorf("feature %q: got %s, want %s", id, got[id], wantVerdict)
		}
	}

	// PASS must carry real evidence, not just a bare verdict.
	for _, r := range results {
		if r.FeatureID == "refund-button" && len(r.Evidence) < 2 {
			t.Errorf("refund-button PASS should cite both the call site and the route: got %v", r.Evidence)
		}
	}
}
