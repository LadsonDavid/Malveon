package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

func TestStartAndLoad(t *testing.T) {
	dir := initTempRepo(t)

	if _, ok := Load(dir); ok {
		t.Fatal("expected no session before Start is called")
	}

	st, err := Start(dir)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if st.StartRef == "" {
		t.Fatal("expected a non-empty start ref")
	}

	loaded, ok := Load(dir)
	if !ok {
		t.Fatal("expected Load to find the session Start just wrote")
	}
	if loaded.StartRef != st.StartRef {
		t.Errorf("loaded ref %q != started ref %q", loaded.StartRef, st.StartRef)
	}
}

func TestStartNoCommits(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if _, err := Start(dir); err == nil {
		t.Fatal("expected Start to fail cleanly when there are no commits yet")
	}
}
