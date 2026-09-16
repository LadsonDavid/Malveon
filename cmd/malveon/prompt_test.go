package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestResolveFeaturesPathSingleCandidate(t *testing.T) {
	var out bytes.Buffer
	path, err := resolveFeaturesPath([]string{"PLAN.md"}, strings.NewReader(""), &out, true)
	if err != nil {
		t.Fatalf("resolveFeaturesPath: %v", err)
	}
	if path != "PLAN.md" {
		t.Errorf("got %q, want PLAN.md", path)
	}
	if !strings.Contains(out.String(), "PLAN.md") {
		t.Errorf("expected the auto-pick to be announced, got: %q", out.String())
	}
}

func TestResolveFeaturesPathMultipleCandidatesPicksAnswer(t *testing.T) {
	var out bytes.Buffer
	candidates := []string{"PLAN.md", "features.json"}
	path, err := resolveFeaturesPath(candidates, strings.NewReader("2\n"), &out, true)
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
	path, err := resolveFeaturesPath(candidates, strings.NewReader("\n"), &out, true)
	if err != nil {
		t.Fatalf("resolveFeaturesPath: %v", err)
	}
	if path != "PLAN.md" {
		t.Errorf("got %q, want PLAN.md (default on bare enter)", path)
	}
}

func TestResolveFeaturesPathNoCandidatesPromptsForPath(t *testing.T) {
	var out bytes.Buffer
	path, err := resolveFeaturesPath(nil, strings.NewReader("my-plan.txt\n"), &out, true)
	if err != nil {
		t.Fatalf("resolveFeaturesPath: %v", err)
	}
	if path != "my-plan.txt" {
		t.Errorf("got %q, want my-plan.txt", path)
	}
}

func TestResolveFeaturesPathNonInteractiveFailsInsteadOfHanging(t *testing.T) {
	var out bytes.Buffer
	if _, err := resolveFeaturesPath([]string{"PLAN.md", "features.json"}, strings.NewReader(""), &out, false); err == nil {
		t.Fatal("expected an error in non-interactive mode with an ambiguous choice, got nil")
	}
	if _, err := resolveFeaturesPath(nil, strings.NewReader(""), &out, false); err == nil {
		t.Fatal("expected an error in non-interactive mode with no candidates, got nil")
	}
}

func TestResolveFeaturesPathInvalidChoice(t *testing.T) {
	var out bytes.Buffer
	candidates := []string{"PLAN.md", "features.json"}
	if _, err := resolveFeaturesPath(candidates, strings.NewReader("9\n"), &out, true); err == nil {
		t.Fatal("expected an error for a choice out of range")
	}
}
