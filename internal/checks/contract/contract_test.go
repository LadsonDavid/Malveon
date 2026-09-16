package contract

import (
	"testing"

	"github.com/LadsonDavid/beta-test/internal/extractor"
	"github.com/LadsonDavid/beta-test/internal/features"
)

// TestFixtureEndToEnd mirrors wiring's fixture test — same repo, checking
// the contract layer this time. refund-button agrees on method (POST/POST);
// update-settings is wired (same path) but disagrees on method (PUT vs
// POST); the other two features never get a wired pair at all, so the
// contract check has nothing to evaluate and must say NOT TESTED, not FAIL.
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
		"refund-button":   Match,
		"update-settings": Mismatch,
		"cancel-order":    NotTested,
		"profile-update":  NotTested,
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
}
