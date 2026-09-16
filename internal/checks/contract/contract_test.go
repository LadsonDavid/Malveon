package contract

import (
	"strings"
	"testing"

	"github.com/LadsonDavid/beta-test/internal/extractor"
	"github.com/LadsonDavid/beta-test/internal/features"
	"github.com/LadsonDavid/beta-test/internal/graph"
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

// TestPythonFixtureEndToEnd and TestGoFixtureEndToEnd mirror the JS/TS
// fixture test above, proving the contract check end-to-end for the two
// languages that were previously only unit tested in isolation.
func TestPythonFixtureEndToEnd(t *testing.T) {
	g, err := extractor.Extract("../../../testdata/fixture-python")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	fs, err := features.Load("../../../testdata/fixture-python/features.json")
	if err != nil {
		t.Fatalf("load features: %v", err)
	}

	want := map[string]Verdict{
		"refund":          Match,
		"update-settings": Mismatch,
		"cancel-order":    NotTested,
		"profile":         NotTested,
	}
	got := map[string]Verdict{}
	for _, r := range Run(g, fs) {
		got[r.FeatureID] = r.Verdict
	}
	for id, wantVerdict := range want {
		if got[id] != wantVerdict {
			t.Errorf("feature %q: got %s, want %s", id, got[id], wantVerdict)
		}
	}
}

func TestGoFixtureEndToEnd(t *testing.T) {
	g, err := extractor.Extract("../../../testdata/fixture-go")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	fs, err := features.Load("../../../testdata/fixture-go/features.json")
	if err != nil {
		t.Fatalf("load features: %v", err)
	}

	want := map[string]Verdict{
		"refund":          Match,
		"update-settings": Mismatch,
		"cancel-order":    NotTested,
		"profile":         NotTested,
	}
	got := map[string]Verdict{}
	for _, r := range Run(g, fs) {
		got[r.FeatureID] = r.Verdict
	}
	for id, wantVerdict := range want {
		if got[id] != wantVerdict {
			t.Errorf("feature %q: got %s, want %s", id, got[id], wantVerdict)
		}
	}
}

// TestBodyFieldAgreement covers the real, scoped-down slice of the
// "full request/response body-field comparison" future work from
// CLAUDE.md 3.2.2: does the call actually send every field the handler
// reads off req.body.
func TestBodyFieldAgreement(t *testing.T) {
	fs := []features.Feature{
		{ID: "refund", Name: "refund"},
		{ID: "cancel", Name: "cancel"},
		{ID: "settings", Name: "settings"},
	}

	cases := []struct {
		name        string
		g           *graph.Graph
		id          string
		wantVerdict Verdict
		wantSub     string
	}{
		{
			name: "match - call sends every field the handler reads",
			g: &graph.Graph{Nodes: []graph.Node{
				{Kind: graph.NetworkCall, File: "c.js", Method: "POST", Path: "/refund", Confidence: graph.Extracted, Words: []string{"refund"}, BodyFields: []string{"amount", "reason"}},
				{Kind: graph.RouteHandler, File: "r.js", Method: "POST", Path: "/refund", Confidence: graph.Extracted, Words: []string{"refund"}, BodyFields: []string{"amount"}},
			}},
			id:          "refund",
			wantVerdict: Match,
		},
		{
			name: "mismatch - handler reads a field the call never sends",
			g: &graph.Graph{Nodes: []graph.Node{
				{Kind: graph.NetworkCall, File: "c.js", Method: "POST", Path: "/cancel", Confidence: graph.Extracted, Words: []string{"cancel"}, BodyFields: []string{"orderId"}},
				{Kind: graph.RouteHandler, File: "r.js", Method: "POST", Path: "/cancel", Confidence: graph.Extracted, Words: []string{"cancel"}, BodyFields: []string{"orderId", "reason"}},
			}},
			id:          "cancel",
			wantVerdict: Mismatch,
			wantSub:     "reason",
		},
		{
			name: "not tested - one side unresolved",
			g: &graph.Graph{Nodes: []graph.Node{
				{Kind: graph.NetworkCall, File: "c.js", Method: "PUT", Path: "/settings", Confidence: graph.Extracted, Words: []string{"settings"}, BodyFields: nil},
				{Kind: graph.RouteHandler, File: "r.js", Method: "PUT", Path: "/settings", Confidence: graph.Extracted, Words: []string{"settings"}, BodyFields: []string{"theme"}},
			}},
			id:          "settings",
			wantVerdict: NotTested,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got Result
			for _, r := range Run(tc.g, fs) {
				if r.FeatureID == tc.id {
					got = r
				}
			}
			if got.BodyVerdict != tc.wantVerdict {
				t.Fatalf("BodyVerdict = %s, want %s (reason: %s)", got.BodyVerdict, tc.wantVerdict, got.BodyReason)
			}
			if tc.wantSub != "" && !strings.Contains(got.BodyReason, tc.wantSub) {
				t.Errorf("BodyReason = %q, want it to mention %q", got.BodyReason, tc.wantSub)
			}
		})
	}
}
