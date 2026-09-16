package heropatterns

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/LadsonDavid/beta-test/internal/session"
	"github.com/LadsonDavid/beta-test/internal/watch"
)

func TestRunDetectsFixedKnownPattern(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "app.js")
	clean := filepath.Join(root, "clean.js")
	if err := os.WriteFile(target, []byte("// start\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clean, []byte("// never touched\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	snapshots := make(chan watch.Generation, 10)
	go func() {
		done <- watch.Run(ctx, watch.Options{
			Root: root, Debounce: 80 * time.Millisecond, HeartbeatInterval: 100 * time.Millisecond,
			OnSnapshot: func(g watch.Generation) { snapshots <- g },
		})
	}()
	waitForWatchStatus(t, root)

	// Introduce the bug.
	if err := os.WriteFile(target, []byte("function check(x) {\n  if (x = 1) { return true; }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForOne(t, snapshots)

	// Fix it, still within the same session.
	if err := os.WriteFile(target, []byte("function check(x) {\n  if (x == 1) { return true; }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForOne(t, snapshots)

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("watch.Run returned an error: %v", err)
	}

	report := Run(root)
	if !report.Available {
		t.Fatalf("expected available, got: %s", report.Reason)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(report.Findings), report.Findings)
	}
	f := report.Findings[0]
	if f.File != "app.js" || f.Pattern != "assignment-in-condition" {
		t.Errorf("unexpected finding: %+v", f)
	}
}

func TestRunUnavailableWithoutWatch(t *testing.T) {
	root := t.TempDir()
	report := Run(root)
	if report.Available {
		t.Fatal("expected unavailable: malveon watch was never run")
	}
}

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
	if err := os.WriteFile(filepath.Join(dir, "old.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

// TestRunFindsPatternStillPresent covers the case a reviewer corrected
// this check on: it must find a bug the session introduced and never
// fixed, not just the narrower "introduced and fixed" case — and it
// must do that from git alone, with no `malveon watch` involved at all.
func TestRunFindsPatternStillPresent(t *testing.T) {
	dir := initTempRepo(t)
	if _, err := session.Start(dir); err != nil {
		t.Fatalf("session.Start: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "handler.go"), []byte(
		"package a\n\nfunc do() error {\n\terr := risky()\n\tif err != nil {\n\t}\n\treturn nil\n}\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	report := Run(dir)
	if !report.Available {
		t.Fatalf("expected available (session started), got: %s", report.Reason)
	}
	if report.WatchAvailable {
		t.Fatalf("expected WatchAvailable=false (malveon watch never ran), got true")
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(report.Findings), report.Findings)
	}
	f := report.Findings[0]
	if f.File != "handler.go" || f.Pattern != "empty-error-handling" || f.Status != StillPresent {
		t.Errorf("unexpected finding: %+v", f)
	}
}

// TestRunSkipsPreExistingPattern proves the git-baseline signal doesn't
// flag a pattern that was already in the file before the session
// started, even if the file is touched again for something unrelated.
func TestRunSkipsPreExistingPattern(t *testing.T) {
	dir := initTempRepo(t)
	buggy := "package a\n\nfunc do() error {\n\terr := risky()\n\tif err != nil {\n\t}\n\treturn nil\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "handler.go"), []byte(buggy), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "pre-existing bug")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	if _, err := session.Start(dir); err != nil {
		t.Fatalf("session.Start: %v", err)
	}
	// Touch the file for something unrelated - the pre-existing bug is
	// still there, but it predates the session, so it must not be
	// flagged as introduced by it.
	touched := buggy + "\n// unrelated comment\n"
	if err := os.WriteFile(filepath.Join(dir, "handler.go"), []byte(touched), 0o644); err != nil {
		t.Fatal(err)
	}

	report := Run(dir)
	if !report.Available {
		t.Fatalf("expected available, got: %s", report.Reason)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected 0 findings (bug predates the session), got %d: %+v", len(report.Findings), report.Findings)
	}
}

func waitForWatchStatus(t *testing.T, root string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := watch.LoadStatus(root); ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for watch status to appear")
}

func waitForOne(t *testing.T, ch <-chan watch.Generation) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a snapshot")
	}
}
