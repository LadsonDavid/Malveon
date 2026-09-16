package planauthority

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LadsonDavid/beta-test/internal/extractor"
	"github.com/LadsonDavid/beta-test/internal/features"
	"github.com/LadsonDavid/beta-test/internal/session"
)

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
	return dir
}

func writeAndCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", msg}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestNotInPlanFlagsOnlyNewUnplannedNodes(t *testing.T) {
	dir := initTempRepo(t)
	writeAndCommit(t, dir, "server.js",
		`const app = require("express")();
app.post("/refund", function (req, res) {});
`, "initial: refund route only")

	if _, err := session.Start(dir); err != nil {
		t.Fatalf("session.Start: %v", err)
	}

	// This session's own (unplanned) addition — should get flagged.
	if err := os.WriteFile(filepath.Join(dir, "debug.js"),
		[]byte(`const app = require("express")();
app.get("/debug-panel", function (req, res) {});
`), 0o644); err != nil {
		t.Fatal(err)
	}

	g, err := extractor.Extract(dir)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	fs := []features.Feature{{ID: "refund-button", Name: "refund button"}}

	res := Run(g, fs, dir)
	if !res.Available {
		t.Fatalf("expected the check to be available, got: %s", res.Reason)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 not-in-plan finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	if res.Findings[0].Path != "/debug-panel" {
		t.Errorf("expected the debug-panel route to be flagged, got %+v", res.Findings[0])
	}
}

func TestNotInPlanUnavailableWithoutSession(t *testing.T) {
	dir := initTempRepo(t)
	writeAndCommit(t, dir, "server.js", `const app = require("express")();`, "initial")

	g, err := extractor.Extract(dir)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	res := Run(g, nil, dir)
	if res.Available {
		t.Fatal("expected the check to report itself unavailable with no session started")
	}
}
