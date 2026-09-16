package incompleteness

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
	if err := os.WriteFile(filepath.Join(dir, "old.go"), []byte("package a\n// TODO: this predates the session, should not be flagged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

func TestRunFindsMarkersInChangedFiles(t *testing.T) {
	dir := initTempRepo(t)
	if _, err := session.Start(dir); err != nil {
		t.Fatalf("session.Start: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte(strJoin(
		"package a",
		"",
		"// TODO: wire this up properly",
		"func Stub() {",
		"  // FIXME: hardcoded for now",
		"}",
	)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "clean.go"), []byte("package a\n\nfunc Done() { return }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := Run(dir)
	if !report.Available {
		t.Fatalf("expected available, got: %s", report.Reason)
	}
	if len(report.Findings) != 2 {
		t.Fatalf("expected 2 findings (TODO + FIXME in new.go), got %d: %+v", len(report.Findings), report.Findings)
	}
	for _, f := range report.Findings {
		if f.File != "new.go" {
			t.Errorf("unexpected file flagged: %s (old.go predates the session, clean.go has no markers)", f.File)
		}
	}
}

func TestRunUnavailableWithoutSession(t *testing.T) {
	dir := initTempRepo(t)
	report := Run(dir)
	if report.Available {
		t.Fatal("expected unavailable: no session started")
	}
}

func strJoin(lines ...string) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}
