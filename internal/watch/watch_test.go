package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunCapturesSnapshotsAndStatus(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "server.go")
	if err := os.WriteFile(target, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	snapshots := make(chan Generation, 10)

	go func() {
		done <- Run(ctx, Options{
			Root:              root,
			Debounce:          80 * time.Millisecond,
			HeartbeatInterval: 100 * time.Millisecond,
			OnSnapshot:        func(g Generation) { snapshots <- g },
		})
	}()

	waitForFile(t, statusPath(root))

	// First edit — should produce one generation.
	if err := os.WriteFile(target, []byte("package a\n\nfunc Broken() { if x = 1 {} }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gen1 := waitForSnapshot(t, snapshots)
	if len(gen1.Files) != 1 || gen1.Files[0] != "server.go" {
		t.Fatalf("unexpected generation 1: %+v", gen1)
	}
	content1, ok := SnapshotContent(root, gen1, "server.go")
	if !ok || content1 != "package a\n\nfunc Broken() { if x = 1 {} }\n" {
		t.Fatalf("unexpected snapshot content: ok=%v content=%q", ok, content1)
	}

	// Second edit — the "fix" — should produce a second generation.
	if err := os.WriteFile(target, []byte("package a\n\nfunc Fixed() { if x == 1 {} }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gen2 := waitForSnapshot(t, snapshots)
	if gen2.Seq != gen1.Seq+1 {
		t.Fatalf("expected sequential generations, got %d then %d", gen1.Seq, gen2.Seq)
	}

	history, err := LoadHistory(root)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 generations in history, got %d", len(history))
	}

	st, ok := LoadStatus(root)
	if !ok || !st.Running {
		t.Fatalf("expected status to show running while watch is active, got %+v (ok=%v)", st, ok)
	}
	usable, reason := Usable(st)
	if !usable {
		t.Fatalf("expected a fresh heartbeat to be usable, got reason: %s", reason)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}

	st, ok = LoadStatus(root)
	if !ok || st.Running {
		t.Fatalf("expected status to show stopped after cancel, got %+v (ok=%v)", st, ok)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func waitForSnapshot(t *testing.T, ch <-chan Generation) Generation {
	t.Helper()
	select {
	case g := <-ch:
		return g
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a snapshot")
		return Generation{}
	}
}
