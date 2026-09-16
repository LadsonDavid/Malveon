package heropatterns

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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
