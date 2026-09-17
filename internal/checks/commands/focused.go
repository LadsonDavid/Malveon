// Focused-test mode (added 2026-09-17, opt-in via --focused-tests) answers
// a narrower question than Run's Test category: not "does the whole test
// suite pass," but "do the tests actually related to what changed this
// session pass" — faster, but strictly additive evidence, never a
// substitute for the full-suite result Run already provides.
//
// Detection only ever uses a mechanism the project's own tooling already
// provides — the same "never invent a command" discipline as the rest of
// this package:
//   - Jest and Vitest both ship real, built-in "only run related/changed
//     tests" support (`--changedSince`/`--changed`), invoked directly via
//     the binary already installed in node_modules/.bin — no npm script
//     indirection, so a wrapped "test" script that doesn't forward extra
//     flags can't silently break this.
//   - pytest has no built-in equivalent at all; this only runs if the
//     tester already installed the pytest-picked plugin themselves.
//   - Go has no built-in or common third-party mechanism a typical
//     project can be assumed to have, so this falls back to a real but
//     coarser heuristic: run `go test` scoped to just the package(s)
//     containing a changed .go file. That's package-level, not the real
//     transitive dependency-graph-aware selection Jest/Vitest do — a
//     package that imports a changed package but wasn't itself modified
//     is not re-tested. Labeled as such in Stack, never presented as
//     equivalent.
package commands

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LadsonDavid/beta-test/internal/gitutil"
	"github.com/LadsonDavid/beta-test/internal/session"
)

// FocusedReport mirrors the Available/Reason shape every other
// session-scoped check in this tool uses (planauthority, heropatterns,
// incompleteness, confidence) — focused-test mode needs a session start
// as its "since" baseline the same way they need one for their diff.
type FocusedReport struct {
	Available bool
	Reason    string
	Results   []Result
}

// RunFocused requires a session to have been started — "focused" means
// "relative to the session-start ref," the same baseline every other
// session-scoped check in this tool already uses, not a separately
// invented notion of "changed."
func RunFocused(root string, timeout time.Duration) FocusedReport {
	st, ok := session.Load(root)
	if !ok {
		return FocusedReport{Available: false, Reason: "no session start recorded — run `malveon session start` before the agent begins its task"}
	}
	changed, err := gitutil.ChangedFilesSince(root, st.StartRef)
	if err != nil {
		return FocusedReport{Available: false, Reason: err.Error()}
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	var results []Result
	for _, dir := range detectToolchainDirs(root) {
		spec, found := planFocusedTest(dir, st.StartRef, changed)
		if !found {
			results = append(results, Result{
				Category: FocusedTest, Stack: dir.stackLabel(), Dir: dir.rel,
				Verdict: NoProof, Reason: noFocusedReason(dir),
			})
			continue
		}
		results = append(results, execute(dir.rel, FocusedTest, spec, timeout))
	}
	return FocusedReport{Available: true, Results: results}
}

var focusedMakeTargetNames = []string{"focused-test", "test-focused", "test-changed", "test-affected"}

// focusedMakefileTarget wins over every language-specific detection below
// when the project itself already defines one of these targets — the
// same "the project's own explicit definition always wins" priority
// Run's regular plan already follows for build/lint/typecheck/test.
func focusedMakefileTarget(dir string) (string, bool) {
	names := rawMakefileTargetNames(dir)
	for _, name := range focusedMakeTargetNames {
		if names[name] {
			return name, true
		}
	}
	return "", false
}

// planFocusedTest picks, for one toolchain directory, the one real
// mechanism available to scope a test run to what changed. Priority
// mirrors Run's plan: an explicit Makefile target first, then whichever
// single stack this directory is (Node/Go/Python) — never more than one
// mechanism attempted per directory, same as Run.
func planFocusedTest(dir toolchainDir, ref string, changed map[string]bool) (commandSpec, bool) {
	if dir.has("Makefile") {
		if target, ok := focusedMakefileTarget(dir.abs); ok {
			return commandSpec{argv: []string{"make", target}, workDir: dir.abs, stack: "Makefile"}, true
		}
	}
	if dir.has("Node") {
		return focusedNodePlan(dir.abs, ref)
	}
	if dir.has("Go") {
		return focusedGoPlan(dir, changed)
	}
	if dir.has("Python") {
		return focusedPythonPlan(dir.abs, ref)
	}
	return commandSpec{}, false
}

func noFocusedReason(dir toolchainDir) string {
	switch {
	case dir.has("Node"):
		return dirLabel(dir.rel) + ": no Jest or Vitest binary found in node_modules/.bin — focused-test selection needs one of the project's own test runners to support it natively"
	case dir.has("Go"):
		return dirLabel(dir.rel) + ": no changed Go file under this directory this session — nothing to focus tests on"
	case dir.has("Python"):
		return dirLabel(dir.rel) + ": pytest-picked isn't installed, and pytest itself has no built-in equivalent — install pytest-picked to enable this"
	default:
		return dirLabel(dir.rel) + ": no supported test runner found for focused-test selection (Jest, Vitest, or pytest-picked)"
	}
}

// focusedNodePlan invokes Jest/Vitest's own binary directly rather than
// going through a package.json "test" script — a wrapped script (e.g.
// piped through cross-env, or one that doesn't forward extra flags)
// could otherwise silently swallow --changedSince/--changed and run
// everything anyway, which would be exactly the kind of unproven claim
// this tool refuses to make.
func focusedNodePlan(dir, ref string) (commandSpec, bool) {
	if bin := filepath.Join(dir, "node_modules", ".bin", "jest"); fileExists(bin) {
		return commandSpec{argv: []string{bin, "--changedSince", ref}, workDir: dir, stack: "Node (Jest, --changedSince)"}, true
	}
	if bin := filepath.Join(dir, "node_modules", ".bin", "vitest"); fileExists(bin) {
		return commandSpec{argv: []string{bin, "run", "--changed", ref}, workDir: dir, stack: "Node (Vitest, --changed)"}, true
	}
	return commandSpec{}, false
}

// focusedPythonPlan only ever fires when the tester already installed
// pytest-picked themselves — pytest itself has no built-in "changed
// files" concept, and inventing a naming-convention guess (foo.py ->
// test_foo.py) would be exactly the kind of guess this tool refuses to
// make elsewhere.
//
// Stated limitation, verified against the real tool (not assumed):
// --mode=branch diffs against git, and `git diff` never shows a plain
// untracked file — pytest-picked needs a new file to be at least
// `git add`-ed (staged) to see it, unlike gitutil.ChangedFilesSince
// (used everywhere else in this tool), which already treats a brand-new
// untracked file as "changed." Malveon never stages files on the
// tester's behalf — that's their git index to control, not this tool's
// to touch — so a genuinely untracked new test file can go undetected by
// this specific mode until it's staged or committed.
func focusedPythonPlan(dir, ref string) (commandSpec, bool) {
	py := pythonBin()
	if !hasModule(py, "pytest_picked") {
		return commandSpec{}, false
	}
	pytest, err := exec.LookPath("pytest")
	if err != nil {
		return commandSpec{}, false
	}
	return commandSpec{
		argv: []string{pytest, "--picked", "--mode=branch", "--parent-branch=" + ref}, workDir: dir,
		stack: "Python (pytest-picked)",
	}, true
}

// focusedGoPlan is the one deliberately coarser fallback in this file —
// see the package doc comment for why Go gets a real but package-level
// heuristic instead of NO PROOF outright.
func focusedGoPlan(dir toolchainDir, changed map[string]bool) (commandSpec, bool) {
	pkgs := focusedGoPackages(dir, changed)
	if len(pkgs) == 0 {
		return commandSpec{}, false
	}
	return commandSpec{
		argv: append([]string{"go", "test"}, pkgs...), workDir: dir.abs,
		stack: "Go (package-level, not dependency-graph-aware)",
	}, true
}

// focusedGoPackages finds every package directory (relative to dir) that
// contains a .go file changed this session — package-level granularity
// only, since real transitive-dependency analysis would need its own
// build-graph implementation this tool doesn't have.
func focusedGoPackages(dir toolchainDir, changed map[string]bool) []string {
	pkgSet := map[string]bool{}
	for f := range changed {
		if filepath.Ext(f) != ".go" {
			continue
		}
		rel, err := filepath.Rel(dir.rel, f)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue // not under this toolchain directory
		}
		pkgSet[filepath.ToSlash(filepath.Dir(rel))] = true
	}

	pkgs := make([]string, 0, len(pkgSet))
	for p := range pkgSet {
		if p == "." {
			pkgs = append(pkgs, ".")
		} else {
			pkgs = append(pkgs, "./"+p)
		}
	}
	sort.Strings(pkgs)
	return pkgs
}
