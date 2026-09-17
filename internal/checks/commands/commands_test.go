package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func indexByCategory(t *testing.T, results []Result) map[Category]Result {
	t.Helper()
	m := map[Category]Result{}
	for _, r := range results {
		m[r.Category] = r
	}
	return m
}

func skipUnless(t *testing.T, bin string) {
	t.Helper()
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("%s not installed in this environment", bin)
	}
}

func TestRun_NodeScripts(t *testing.T) {
	skipUnless(t, "npm")
	skipUnless(t, "node")

	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"name": "fixture",
		"scripts": {
			"build": "node -e \"process.exit(0)\"",
			"lint": "node -e \"process.exit(1)\""
		}
	}`)

	byCat := indexByCategory(t, Run(dir, 30*time.Second))

	if got := byCat[Build].Verdict; got != Pass {
		t.Errorf("build: got %s, want PASS (%s)", got, byCat[Build].Reason)
	}
	if got := byCat[Lint].Verdict; got != Fail {
		t.Errorf("lint: got %s, want FAIL", got)
	}
	if got := byCat[Typecheck].Verdict; got != NoProof {
		t.Errorf("typecheck: got %s, want NO PROOF (no script defined)", got)
	}
	if got := byCat[Test].Verdict; got != NoProof {
		t.Errorf("test: got %s, want NO PROOF (no script defined)", got)
	}
}

func TestRun_MakefileTakesPriorityOverNode(t *testing.T) {
	skipUnless(t, "make")
	skipUnless(t, "node")

	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts":{"build":"node -e \"process.exit(0)\""}}`)
	writeFile(t, dir, "Makefile", "build:\n\texit 1\n")

	byCat := indexByCategory(t, Run(dir, 30*time.Second))

	if got := byCat[Build].Stack; got != "Makefile" {
		t.Fatalf("expected Makefile to take priority over package.json, got stack %q", got)
	}
	if got := byCat[Build].Verdict; got != Fail {
		t.Errorf("build: got %s, want FAIL (Makefile's own target should run, not package.json's)", got)
	}
}

func TestRun_GoStack(t *testing.T) {
	skipUnless(t, "go")

	t.Run("build failure also fails lint and mirrors into typecheck", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "go.mod", "module fixture\n\ngo 1.21\n")
		writeFile(t, dir, "main.go", "package main\n\nfunc main() {\n\tx :=\n}\n")

		byCat := indexByCategory(t, Run(dir, 30*time.Second))

		if got := byCat[Build].Verdict; got != Fail {
			t.Errorf("build: got %s, want FAIL", got)
		}
		if got := byCat[Typecheck].Verdict; got != Fail {
			t.Errorf("typecheck: got %s, want FAIL (mirrors build)", got)
		}
		if !strings.Contains(byCat[Typecheck].Reason, "mirrors the build result") {
			t.Errorf("typecheck reason should explain it mirrors build, got %q", byCat[Typecheck].Reason)
		}
		// Lint (go vet, or golangci-lint) is an independent process — it
		// must still have run and reported something, not been silently
		// skipped just because build already failed (the Bulkhead rule).
		if _, ok := byCat[Lint]; !ok {
			t.Errorf("lint result missing entirely — one failing command must not stop the others from running")
		}
	})

	t.Run("no test files is NO PROOF, not PASS", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "go.mod", "module fixture\n\ngo 1.21\n")
		writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")

		byCat := indexByCategory(t, Run(dir, 30*time.Second))

		if got := byCat[Build].Verdict; got != Pass {
			t.Fatalf("build: got %s, want PASS (%s)", got, byCat[Build].Reason)
		}
		if got := byCat[Test].Verdict; got != NoProof {
			t.Errorf("test: got %s, want NO PROOF (no test files exist — 'go test' exiting 0 here isn't proof anything passed)", got)
		}
	})

	t.Run("a real passing test is PASS, a real failing test is FAIL", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "go.mod", "module fixture\n\ngo 1.21\n")
		writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n\nfunc Add(a, b int) int { return a + b }\n")
		writeFile(t, dir, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatal(\"wrong\")\n\t}\n}\n")

		byCat := indexByCategory(t, Run(dir, 30*time.Second))
		if got := byCat[Test].Verdict; got != Pass {
			t.Errorf("test: got %s, want PASS (%s)\n%s", got, byCat[Test].Reason, byCat[Test].Output)
		}
	})

	t.Run("a real failing test is FAIL", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "go.mod", "module fixture\n\ngo 1.21\n")
		writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n\nfunc Add(a, b int) int { return a + b }\n")
		writeFile(t, dir, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 999 {\n\t\tt.Fatal(\"wrong\")\n\t}\n}\n")

		byCat := indexByCategory(t, Run(dir, 30*time.Second))
		if got := byCat[Test].Verdict; got != Fail {
			t.Errorf("test: got %s, want FAIL", got)
		}
	})
}

func TestRun_PythonStack(t *testing.T) {
	skipUnless(t, "python3")

	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[project]\nname = \"fixture\"\nversion = \"0.1.0\"\n")
	writeFile(t, dir, "bad.py", "import os\n")
	writeFile(t, dir, "typed.py", "def add(a: int, b: int) -> int:\n    return a + b\n")
	writeFile(t, dir, "tests/test_sample.py", "def test_ok():\n    assert 1 == 1\n")

	byCat := indexByCategory(t, Run(dir, 30*time.Second))

	if got := byCat[Build].Verdict; got != NoProof {
		t.Errorf("build: got %s, want NO PROOF (no [build-system] declared, Python apps don't have a build step by default)", got)
	}

	if _, err := exec.LookPath("ruff"); err == nil {
		if got := byCat[Lint].Verdict; got != Fail {
			t.Errorf("lint: got %s, want FAIL (bad.py has an unused import)", got)
		}
	}
	if _, err := exec.LookPath("mypy"); err == nil {
		if got := byCat[Typecheck].Verdict; got != Pass {
			t.Errorf("typecheck: got %s, want PASS (%s)", got, byCat[Typecheck].Reason)
		}
	}
	if _, err := exec.LookPath("pytest"); err == nil {
		if got := byCat[Test].Verdict; got != Pass {
			t.Errorf("test: got %s, want PASS (%s)\n%s", got, byCat[Test].Reason, byCat[Test].Output)
		}
	}
}

func TestExecute_Timeout(t *testing.T) {
	skipUnless(t, "sleep")

	spec := commandSpec{argv: []string{"sleep", "5"}, workDir: t.TempDir(), stack: "test"}
	r := execute(".", Build, spec, 200*time.Millisecond)

	if r.Verdict != NoProof {
		t.Fatalf("got %s, want NO PROOF on timeout", r.Verdict)
	}
	if !strings.Contains(r.Reason, "timed out") {
		t.Errorf("reason doesn't mention the timeout: %q", r.Reason)
	}
}

func TestRun_OneTimeoutDoesNotBlockOthers(t *testing.T) {
	skipUnless(t, "npm")
	skipUnless(t, "node")

	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts": {
		"build": "node -e \"process.exit(0)\"",
		"lint": "node -e \"setTimeout(()=>{}, 10000)\""
	}}`)

	// 3s, not the ~200-300ms this test originally used: npm's own cold-start
	// overhead (resolving package.json, spawning node) measured at 866ms on
	// a real Windows machine for this exact trivial script — a short
	// timeout here was timing out npm's own startup, not proving anything
	// about the Bulkhead property this test exists to check. lint's hang is
	// 10s so it reliably still exceeds this timeout on any machine.
	byCat := indexByCategory(t, Run(dir, 3*time.Second))

	if got := byCat[Build].Verdict; got != Pass {
		t.Errorf("build should have run and passed despite lint hanging, got %s", got)
	}
	if got := byCat[Lint].Verdict; got != NoProof || !strings.Contains(byCat[Lint].Reason, "timed out") {
		t.Errorf("lint should have timed out as NO PROOF, got %s / %q", got, byCat[Lint].Reason)
	}
}

func TestRun_NoManifestFound(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "readme.txt", "nothing to see here\n")

	results := Run(dir, 5*time.Second)
	if len(results) != 0 {
		t.Errorf("expected no results with no recognized manifest anywhere, got %d", len(results))
	}
}
