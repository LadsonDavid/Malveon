package heroact

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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

func TestHeroActClassification(t *testing.T) {
	dir := initTempRepo(t)
	// utils.go exists before the session and is never touched this session.
	writeAndCommit(t, dir, "utils.go", "package main\n", "initial")

	if _, err := session.Start(dir); err != nil {
		t.Fatalf("session.Start: %v", err)
	}

	// newfeature.go is created during this session.
	if err := os.WriteFile(filepath.Join(dir, "newfeature.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := "fixed a nil check bug in newfeature.go\n" +
		"fixed an off-by-one bug in utils.go\n" +
		"fixed a typo in ghost.go\n" +
		"cleaned up some logging\n"
	reportPath := filepath.Join(t.TempDir(), "bugs.txt")
	if err := os.WriteFile(reportPath, []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}

	res := Run(dir, reportPath)
	if !res.Available {
		t.Fatalf("expected the check to be available, got: %s", res.Reason)
	}
	if len(res.Findings) != 4 {
		t.Fatalf("expected 4 findings, got %d: %+v", len(res.Findings), res.Findings)
	}

	if res.Findings[0].Verdict != SelfIntroduced {
		t.Errorf("newfeature.go: expected %s, got %s", SelfIntroduced, res.Findings[0].Verdict)
	}
	if res.Findings[1].Verdict != PreExisting {
		t.Errorf("utils.go: expected %s, got %s", PreExisting, res.Findings[1].Verdict)
	}
	if res.Findings[2].Verdict != NotResolved {
		t.Errorf("ghost.go (doesn't exist): expected %s, got %s", NotResolved, res.Findings[2].Verdict)
	}
	if res.Findings[3].Verdict != NotResolved {
		t.Errorf("no file reference: expected %s, got %s", NotResolved, res.Findings[3].Verdict)
	}
}

func TestHeroActUnavailableWithoutSession(t *testing.T) {
	dir := initTempRepo(t)
	writeAndCommit(t, dir, "a.go", "package main\n", "initial")

	res := Run(dir, "does-not-matter.txt")
	if res.Available {
		t.Fatal("expected the check to report itself unavailable with no session started")
	}
}
