package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

func writeAndCommit(t *testing.T, dir, name, content string) {
	t.Helper()
	writeFile(t, dir, name, content)
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "baseline"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func stageAll(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add -A: %v\n%s", err, out)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRunFocused_NoSession(t *testing.T) {
	dir := t.TempDir()
	r := RunFocused(dir, 5*time.Second)
	if r.Available {
		t.Fatal("expected focused-test mode to be unavailable without a session start")
	}
}

func TestRunFocused_JestBinaryInvokedWithChangedSince(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shebang script isn't a real Windows executable")
	}
	dir := initTempRepo(t)
	writeAndCommit(t, dir, "package.json", `{"name":"fixture"}`)
	st, err := session.Start(dir)
	if err != nil {
		t.Fatalf("session start: %v", err)
	}

	// A fake Jest binary standing in for the real one — this proves
	// malveon's own detection/invocation wiring (right path, right
	// flags), not Jest's own --changedSince behavior, which is Jest's
	// tested feature, not this tool's.
	writeExecutable(t, filepath.Join(dir, "node_modules", ".bin", "jest"), "#!/bin/sh\necho \"jest called with: $@\"\nexit 0\n")

	writeFile(t, dir, "app.js", "console.log('changed');\n")

	report := RunFocused(dir, 10*time.Second)
	if !report.Available {
		t.Fatalf("expected focused-test mode available, got unavailable: %s", report.Reason)
	}
	byCat := indexByCategory(t, report.Results)
	r := byCat[FocusedTest]
	if r.Verdict != Pass {
		t.Fatalf("got %s, want PASS (%s)\n%s", r.Verdict, r.Reason, r.Output)
	}
	if !strings.Contains(r.Command, "--changedSince") || !strings.Contains(r.Command, st.StartRef) {
		t.Errorf("expected the command to invoke --changedSince with the session ref, got %q", r.Command)
	}
	if !strings.Contains(r.Stack, "Jest") {
		t.Errorf("expected the stack label to name Jest, got %q", r.Stack)
	}
}

func TestRunFocused_VitestPreferredWhenJestBinaryAbsent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shebang script isn't a real Windows executable")
	}
	dir := initTempRepo(t)
	writeAndCommit(t, dir, "package.json", `{"name":"fixture"}`)
	session.Start(dir)

	writeExecutable(t, filepath.Join(dir, "node_modules", ".bin", "vitest"), "#!/bin/sh\nexit 0\n")
	writeFile(t, dir, "app.js", "// changed\n")

	report := RunFocused(dir, 10*time.Second)
	byCat := indexByCategory(t, report.Results)
	r := byCat[FocusedTest]
	if r.Verdict != Pass {
		t.Fatalf("got %s, want PASS (%s)", r.Verdict, r.Reason)
	}
	if !strings.Contains(r.Command, "--changed") || !strings.Contains(r.Stack, "Vitest") {
		t.Errorf("expected a Vitest --changed invocation, got stack=%q command=%q", r.Stack, r.Command)
	}
}

func TestRunFocused_NodeWithoutJestOrVitestIsNoProof(t *testing.T) {
	dir := initTempRepo(t)
	writeAndCommit(t, dir, "package.json", `{"name":"fixture","scripts":{"test":"echo ok"}}`)
	session.Start(dir)
	writeFile(t, dir, "app.js", "// changed\n")

	report := RunFocused(dir, 10*time.Second)
	byCat := indexByCategory(t, report.Results)
	r := byCat[FocusedTest]
	if r.Verdict != NoProof {
		t.Fatalf("got %s, want NO PROOF (no Jest/Vitest binary present)", r.Verdict)
	}
	if !strings.Contains(r.Reason, "Jest or Vitest") {
		t.Errorf("reason should explain no Jest/Vitest binary was found, got %q", r.Reason)
	}
}

func TestRunFocused_PythonRequiresPytestPicked(t *testing.T) {
	skipUnless(t, "python3")
	skipUnless(t, "pytest")

	dir := initTempRepo(t)
	writeAndCommit(t, dir, "pyproject.toml", "[project]\nname = \"fixture\"\n")
	st, err := session.Start(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "tests/test_sample.py", "def test_ok():\n    assert 1 == 1\n")
	// pytest-picked's --mode=branch diffs against git, which never shows
	// a plain untracked file — it has to be at least staged to be seen
	// (verified against the real tool, not assumed; see focusedPythonPlan's
	// doc comment). `git add` only, deliberately not committed, to prove
	// staged-but-uncommitted is enough.
	stageAll(t, dir)

	report := RunFocused(dir, 30*time.Second)
	byCat := indexByCategory(t, report.Results)
	r := byCat[FocusedTest]

	if hasModule(pythonBin(), "pytest_picked") {
		if r.Verdict != Pass {
			t.Fatalf("got %s, want PASS (%s)\n%s", r.Verdict, r.Reason, r.Output)
		}
		if !strings.Contains(r.Command, "--picked") || !strings.Contains(r.Command, st.StartRef) {
			t.Errorf("expected a pytest --picked invocation naming the session ref, got %q", r.Command)
		}
	} else {
		if r.Verdict != NoProof {
			t.Fatalf("got %s, want NO PROOF (pytest-picked not installed)", r.Verdict)
		}
		if !strings.Contains(r.Reason, "pytest-picked") {
			t.Errorf("reason should name pytest-picked, got %q", r.Reason)
		}
	}
}

func TestRunFocused_GoScopesToPackagesWithChangedFiles(t *testing.T) {
	skipUnless(t, "go")

	dir := initTempRepo(t)
	writeAndCommit(t, dir, "go.mod", "module fixture\n\ngo 1.21\n")
	writeAndCommit(t, dir, "unrelated/unrelated.go", "package unrelated\n\nfunc Noop() {}\n")
	session.Start(dir)

	// Only backend/ changes this session — focused mode must scope to
	// just that package, never touching (or even knowing about) the
	// unrelated/ package that also exists in the module.
	writeFile(t, dir, "backend/handler.go", "package backend\n\nfunc Add(a, b int) int { return a + b }\n")
	writeFile(t, dir, "backend/handler_test.go", "package backend\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatal(\"wrong\")\n\t}\n}\n")

	report := RunFocused(dir, 30*time.Second)
	if !report.Available {
		t.Fatalf("expected available, got: %s", report.Reason)
	}
	byCat := indexByCategory(t, report.Results)
	r := byCat[FocusedTest]

	if r.Verdict != Pass {
		t.Fatalf("got %s, want PASS (%s)\n%s", r.Verdict, r.Reason, r.Output)
	}
	if !strings.Contains(r.Command, "./backend") {
		t.Errorf("expected the command scoped to ./backend, got %q", r.Command)
	}
	if strings.Contains(r.Command, "unrelated") {
		t.Errorf("expected the unrelated/ package never mentioned, got %q", r.Command)
	}
	if !strings.Contains(r.Stack, "package-level") {
		t.Errorf("expected the stack label to disclose the package-level, non-dependency-graph-aware scope, got %q", r.Stack)
	}
}

func TestRunFocused_GoNoChangedFilesIsNoProof(t *testing.T) {
	skipUnless(t, "go")

	dir := initTempRepo(t)
	writeAndCommit(t, dir, "go.mod", "module fixture\n\ngo 1.21\n")
	writeAndCommit(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	session.Start(dir) // nothing changes after this

	report := RunFocused(dir, 10*time.Second)
	byCat := indexByCategory(t, report.Results)
	r := byCat[FocusedTest]
	if r.Verdict != NoProof {
		t.Fatalf("got %s, want NO PROOF (nothing changed to focus on)", r.Verdict)
	}
}

func TestRunFocused_MakefileTargetTakesPriority(t *testing.T) {
	skipUnless(t, "make")

	dir := initTempRepo(t)
	writeAndCommit(t, dir, "go.mod", "module fixture\n\ngo 1.21\n")
	writeAndCommit(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	writeAndCommit(t, dir, "Makefile", "test-changed:\n\texit 0\n")
	session.Start(dir)
	writeFile(t, dir, "main.go", "package main\n\nfunc main() { println(1) }\n")

	report := RunFocused(dir, 10*time.Second)
	byCat := indexByCategory(t, report.Results)
	r := byCat[FocusedTest]
	if r.Stack != "Makefile" {
		t.Fatalf("expected the Makefile's own focused-test target to win over the Go fallback, got stack %q", r.Stack)
	}
	if r.Verdict != Pass {
		t.Errorf("got %s, want PASS", r.Verdict)
	}
}
