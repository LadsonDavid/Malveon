package wiring

import (
	"strings"
	"testing"

	"github.com/LadsonDavid/beta-test/internal/extractor"
	"github.com/LadsonDavid/beta-test/internal/features"
	"github.com/LadsonDavid/beta-test/internal/graph"
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

// TestPythonFixtureEndToEnd and TestGoFixtureEndToEnd prove the pipeline
// end-to-end for the two languages that, until now, were only ever unit
// tested in isolation (extractor_test.go's TestScanPython/TestScanGo) —
// never run through a full multi-file fixture the way JS/TS already was.
// Same four verdict shapes as the JS/TS fixture: PASS, FAIL, NOT TESTED,
// and a wired-but-method-mismatched PASS (contract's job to catch that).
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
		"refund":          Pass,
		"cancel-order":    Fail,
		"profile":         NotTested, // f-string interpolated path, never resolvable
		"update-settings": Pass,      // wiring only checks the path connects, not the method
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
		"refund":          Pass,
		"cancel-order":    Fail,
		"profile":         NotTested, // route exists, nothing calls it
		"update-settings": Pass,
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

// TestMissingSideReasonIsSpecific covers the case a reviewer actually
// asked about: "the backend exists but there's no frontend for it"
// (and its mirror) should say exactly that, not the old one-size-fits-all
// "frontend and/or backend" message that couldn't tell the two apart.
func TestMissingSideReasonIsSpecific(t *testing.T) {
	backendOnly := &graph.Graph{Nodes: []graph.Node{
		{Kind: graph.RouteHandler, Path: "/delete-account", Confidence: graph.Extracted, Words: []string{"delete", "account"}},
	}}
	frontendOnly := &graph.Graph{Nodes: []graph.Node{
		{Kind: graph.NetworkCall, Path: "/export-data", Confidence: graph.Extracted, Words: []string{"export", "data"}},
	}}
	neither := &graph.Graph{}

	fs := []features.Feature{
		{ID: "delete-account", Name: "delete account"},
		{ID: "export-data", Name: "export data"},
		{ID: "archive-orders", Name: "archive orders"},
	}

	cases := []struct {
		name    string
		g       *graph.Graph
		id      string
		wantSub string
	}{
		{"backend only", backendOnly, "delete-account", "backend route found, but no matching frontend call"},
		{"frontend only", frontendOnly, "export-data", "frontend call found, but no matching backend route"},
		{"neither", neither, "archive-orders", "no matching frontend or backend node found"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results := Run(tc.g, fs)
			var got string
			var verdict Verdict
			for _, r := range results {
				if r.FeatureID == tc.id {
					got, verdict = r.Reason, r.Verdict
				}
			}
			if got == "" {
				t.Fatalf("no result found for feature %q", tc.id)
			}
			if verdict != NotTested {
				t.Errorf("verdict = %s, want NOT TESTED — a missing side must never be guessed to PASS/FAIL", verdict)
			}
			if !strings.Contains(got, tc.wantSub) {
				t.Errorf("reason = %q, want it to contain %q", got, tc.wantSub)
			}
		})
	}
}
