package gitutil

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
	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", msg}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestCurrentRefAndChangedFiles(t *testing.T) {
	dir := initTempRepo(t)
	writeFile(t, dir, "a.go", "package a\n")
	commitAll(t, dir, "initial")

	baseRef, err := CurrentRef(dir)
	if err != nil {
		t.Fatalf("CurrentRef: %v", err)
	}
	if baseRef == "" {
		t.Fatal("expected a non-empty ref")
	}

	// modify a tracked file, add a new untracked one
	writeFile(t, dir, "a.go", "package a\n\nfunc B() {}\n")
	writeFile(t, dir, "b.go", "package a\n")

	changed, err := ChangedFilesSince(dir, baseRef)
	if err != nil {
		t.Fatalf("ChangedFilesSince: %v", err)
	}
	if !changed["a.go"] {
		t.Errorf("expected a.go (modified tracked file) to be reported changed: %v", changed)
	}
	if !changed["b.go"] {
		t.Errorf("expected b.go (new untracked file) to be reported changed: %v", changed)
	}
}

func TestCurrentRefNoCommits(t *testing.T) {
	dir := initTempRepo(t)
	if _, err := CurrentRef(dir); err == nil {
		t.Fatal("expected an error when the repo has no commits yet")
	}
}
