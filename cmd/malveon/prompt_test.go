package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestResolveFeaturesPathSingleNameCandidateStillAsks(t *testing.T) {
	var out bytes.Buffer
	path, err := resolveFeaturesPath([]string{"PLAN.md"}, false, strings.NewReader("\n"), &out, true)
	if err != nil {
		t.Fatalf("resolveFeaturesPath: %v", err)
	}
	if path != "PLAN.md" {
		t.Errorf("got %q, want PLAN.md (confirmed via bare enter)", path)
	}
	if !strings.Contains(out.String(), "use it?") {
		t.Errorf("expected a confirmation prompt even for a single name match, got: %q", out.String())
	}
}

func TestResolveFeaturesPathSingleNameCandidateRejectedFallsBackToManualPath(t *testing.T) {
	var out bytes.Buffer
	path, err := resolveFeaturesPath([]string{"PLAN.md"}, false, strings.NewReader("n\ndocs/PLAN.md\n"), &out, true)
	if err != nil {
		t.Fatalf("resolveFeaturesPath: %v", err)
	}
	if path != "docs/PLAN.md" {
		t.Errorf("got %q, want docs/PLAN.md (typed after rejecting the suggested name match)", path)
	}
}

func TestResolveFeaturesPathSingleNameCandidateNonInteractiveFails(t *testing.T) {
	var out bytes.Buffer
	if _, err := resolveFeaturesPath([]string{"PLAN.md"}, false, strings.NewReader(""), &out, false); err == nil {
		t.Fatal("expected an error: even a name match can't be confirmed without a terminal")
	}
}

func TestResolveFeaturesPathMultipleCandidatesPicksAnswer(t *testing.T) {
	var out bytes.Buffer
	candidates := []string{"PLAN.md", "features.json"}
	path, err := resolveFeaturesPath(candidates, false, strings.NewReader("2\n"), &out, true)
	if err != nil {
		t.Fatalf("resolveFeaturesPath: %v", err)
	}
	if path != "features.json" {
		t.Errorf("got %q, want features.json (choice 2)", path)
	}
}

func TestResolveFeaturesPathMultipleCandidatesDefaultsToFirstOnEnter(t *testing.T) {
	var out bytes.Buffer
	candidates := []string{"PLAN.md", "features.json"}
	path, err := resolveFeaturesPath(candidates, false, strings.NewReader("\n"), &out, true)
	if err != nil {
		t.Fatalf("resolveFeaturesPath: %v", err)
	}
	if path != "PLAN.md" {
		t.Errorf("got %q, want PLAN.md (default on bare enter)", path)
	}
}

func TestResolveFeaturesPathNoCandidatesPromptsForPath(t *testing.T) {
	var out bytes.Buffer
	path, err := resolveFeaturesPath(nil, false, strings.NewReader("my-plan.txt\n"), &out, true)
	if err != nil {
		t.Fatalf("resolveFeaturesPath: %v", err)
	}
	if path != "my-plan.txt" {
		t.Errorf("got %q, want my-plan.txt", path)
	}
}

func TestResolveFeaturesPathNonInteractiveFailsInsteadOfHanging(t *testing.T) {
	var out bytes.Buffer
	if _, err := resolveFeaturesPath([]string{"PLAN.md", "features.json"}, false, strings.NewReader(""), &out, false); err == nil {
		t.Fatal("expected an error in non-interactive mode with an ambiguous choice, got nil")
	}
	if _, err := resolveFeaturesPath(nil, false, strings.NewReader(""), &out, false); err == nil {
		t.Fatal("expected an error in non-interactive mode with no candidates, got nil")
	}
}

func TestResolveFeaturesPathInvalidChoice(t *testing.T) {
	var out bytes.Buffer
	candidates := []string{"PLAN.md", "features.json"}
	if _, err := resolveFeaturesPath(candidates, false, strings.NewReader("9\n"), &out, true); err == nil {
		t.Fatal("expected an error for a choice out of range")
	}
}

// TestResolveFeaturesPathContentScanRequiresConfirmation covers the real
// bug this session found: a content-only match (a guess, not a
// deliberate name) must never be silently trusted, even when it's the
// only candidate.
func TestResolveFeaturesPathContentScanRequiresConfirmation(t *testing.T) {
	var out bytes.Buffer
	path, err := resolveFeaturesPath([]string{"README.md"}, true, strings.NewReader("\n"), &out, true)
	if err != nil {
		t.Fatalf("resolveFeaturesPath: %v", err)
	}
	if path != "README.md" {
		t.Errorf("got %q, want README.md (confirmed via bare enter)", path)
	}
	if !strings.Contains(out.String(), "use it?") {
		t.Errorf("expected a confirmation prompt, got: %q", out.String())
	}
}

func TestResolveFeaturesPathContentScanRejectedFallsBackToManualPath(t *testing.T) {
	var out bytes.Buffer
	// "n" rejects README.md, then the real plan path is typed instead.
	path, err := resolveFeaturesPath([]string{"README.md"}, true, strings.NewReader("n\ndocs/PLAN.md\n"), &out, true)
	if err != nil {
		t.Fatalf("resolveFeaturesPath: %v", err)
	}
	if path != "docs/PLAN.md" {
		t.Errorf("got %q, want docs/PLAN.md (typed after rejecting the content-scan guess)", path)
	}
}

func TestResolveFeaturesPathContentScanNonInteractiveFails(t *testing.T) {
	var out bytes.Buffer
	if _, err := resolveFeaturesPath([]string{"README.md"}, true, strings.NewReader(""), &out, false); err == nil {
		t.Fatal("expected an error: a content-scan guess can't be confirmed without a terminal")
	}
}
