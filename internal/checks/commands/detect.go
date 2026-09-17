package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/LadsonDavid/beta-test/internal/skipdirs"
)

// toolchainDir is one directory in the project that defines at least one
// of the manifests this check knows how to read (Makefile, package.json,
// go.mod, a Python manifest). A monorepo with a Node frontend and a Go
// backend in separate directories gets one entry per directory, so
// neither stack's results get silently dropped or merged into the
// other's.
type toolchainDir struct {
	rel   string // relative to root; "." for the project root
	abs   string
	kinds []string // every manifest kind found here: "Makefile", "Node", "Go", "Python"
}

func (d toolchainDir) stackLabel() string {
	if len(d.kinds) == 0 {
		return "unknown"
	}
	return strings.Join(d.kinds, " + ")
}

func (d toolchainDir) has(kind string) bool { return containsStr(d.kinds, kind) }

// detectToolchainDirs walks the whole project tree — the same skip list
// the extractor and plan-file detector already use — looking for the
// files a project itself uses to define its own build/lint/typecheck/test
// commands.
func detectToolchainDirs(root string) []toolchainDir {
	byDir := map[string]*toolchainDir{}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry — skip it, don't abort the whole scan
		}
		if d.IsDir() {
			if path != root && skipdirs.Names[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		var kind string
		switch d.Name() {
		case "Makefile", "makefile", "GNUmakefile":
			kind = "Makefile"
		case "package.json":
			kind = "Node"
		case "go.mod":
			kind = "Go"
		case "pyproject.toml", "setup.py", "requirements.txt", "setup.cfg", "Pipfile":
			kind = "Python"
		default:
			return nil
		}

		dirPath := filepath.Dir(path)
		rel, relErr := filepath.Rel(root, dirPath)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		entry, ok := byDir[rel]
		if !ok {
			entry = &toolchainDir{rel: rel, abs: dirPath}
			byDir[rel] = entry
		}
		if !containsStr(entry.kinds, kind) {
			entry.kinds = append(entry.kinds, kind)
		}
		return nil
	})

	dirs := make([]toolchainDir, 0, len(byDir))
	for _, d := range byDir {
		dirs = append(dirs, *d)
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].rel < dirs[j].rel })
	return dirs
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirLabel(rel string) string {
	if rel == "." {
		return "the project root"
	}
	return rel
}

// commandSpec is one category's resolved real command for one toolchain
// directory — or, if unrunnable is set, a command the project defined
// that malveon can't actually execute right now (e.g. the package
// manager isn't installed), which is its own honest NO PROOF reason
// rather than being silently treated the same as "nothing was found at
// all."
type commandSpec struct {
	argv       []string
	workDir    string
	stack      string
	mirrors    Category // non-empty only for Go's implicit typecheck-mirrors-build case
	unrunnable string
	// interpret, if set, can override the default exit-code-based verdict
	// for a case the exit code alone can't honestly distinguish (see
	// interpretGoTest). ok=false means "use the default handling."
	interpret func(err error, output string) (Verdict, string, bool)
}

// planFor decides, for one toolchain directory, which real command (if
// any) answers each of build/lint/typecheck/test. Priority within a
// directory: a Makefile target the project defined itself always wins —
// the most explicit, least-guessed signal there is — over a language
// default; Node/Go/Python-specific detection only fills in whatever the
// Makefile didn't already cover.
func planFor(dir toolchainDir) map[Category]commandSpec {
	plan := map[Category]commandSpec{}

	if dir.has("Makefile") {
		for cat, target := range makefileTargets(dir.abs) {
			plan[cat] = commandSpec{argv: []string{"make", target}, workDir: dir.abs, stack: "Makefile"}
		}
	}
	if dir.has("Node") {
		addNodePlan(plan, dir.abs)
	}
	if dir.has("Go") {
		addGoPlan(plan, dir.abs)
	}
	if dir.has("Python") {
		addPythonPlan(plan, dir.abs)
	}

	return plan
}

// noCommandReason explains, per manifest actually present in dir, why
// that manifest didn't yield a real command for cat — specific to what
// was actually checked, not one generic catch-all sentence.
func noCommandReason(dir toolchainDir, cat Category) string {
	var checked []string
	if dir.has("Makefile") {
		checked = append(checked, fmt.Sprintf("no %q target in the Makefile", string(cat)))
	}
	if dir.has("Node") {
		line := fmt.Sprintf("no %q script in package.json", string(cat))
		if cat == Typecheck {
			line += " (malveon never invents a tsc invocation the project didn't define itself — add a \"typecheck\" script to enable this)"
		}
		checked = append(checked, line)
	}
	if dir.has("Python") {
		checked = append(checked, "no matching Python tool found on PATH, or no signal a test suite/build step exists")
	}
	if len(checked) == 0 {
		checked = append(checked, "no recognized manifest defined this command")
	}
	return fmt.Sprintf("%s: %s", dirLabel(dir.rel), strings.Join(checked, "; "))
}

var makeTargetPattern = regexp.MustCompile(`(?m)^([A-Za-z0-9_.-]+)\s*:[^=]`)

// rawMakefileTargetNames reports every target name declared in dir's
// Makefile, with no interpretation of what any of them do — shared by
// makefileTargets (build/lint/typecheck/test) and focusedMakefileTarget
// (the focused-test names), so both read the exact same real targets.
func rawMakefileTargetNames(dir string) map[string]bool {
	names := map[string]bool{}
	for _, fname := range []string{"Makefile", "makefile", "GNUmakefile"} {
		raw, err := os.ReadFile(filepath.Join(dir, fname))
		if err != nil {
			continue
		}
		for _, m := range makeTargetPattern.FindAllStringSubmatch(string(raw), -1) {
			names[m[1]] = true
		}
		break // only one of these three actually exists in a given directory
	}
	return names
}

// makefileTargets reports which of build/lint/typecheck/test the
// Makefile in dir actually declares as a target — never a guess at what
// a target named something else might do.
func makefileTargets(dir string) map[Category]string {
	names := rawMakefileTargetNames(dir)
	out := map[Category]string{}
	if names["build"] {
		out[Build] = "build"
	}
	if names["lint"] {
		out[Lint] = "lint"
	}
	if names["typecheck"] {
		out[Typecheck] = "typecheck"
	} else if names["type-check"] {
		out[Typecheck] = "type-check"
	}
	if names["test"] {
		out[Test] = "test"
	}
	return out
}

func detectNodePM(dir string) string {
	if fileExists(filepath.Join(dir, "pnpm-lock.yaml")) {
		return "pnpm"
	}
	if fileExists(filepath.Join(dir, "yarn.lock")) {
		return "yarn"
	}
	return "npm"
}

// addNodePlan fills in whatever build/lint/typecheck/test a package.json
// in dir explicitly defines as a script — never a script name the
// project didn't itself define.
func addNodePlan(plan map[Category]commandSpec, dir string) {
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(raw, &pkg) != nil {
		return
	}

	pm := detectNodePM(dir)
	stack := fmt.Sprintf("Node (%s)", pm)
	_, pmErr := exec.LookPath(pm)

	assign := func(cat Category, scriptNames ...string) {
		if _, exists := plan[cat]; exists {
			return
		}
		for _, name := range scriptNames {
			if _, has := pkg.Scripts[name]; !has {
				continue
			}
			if pmErr != nil {
				plan[cat] = commandSpec{
					stack:      stack,
					unrunnable: fmt.Sprintf("found a %q script in package.json, but %q isn't installed on PATH to run it", name, pm),
				}
				return
			}
			plan[cat] = commandSpec{argv: []string{pm, "run", name}, workDir: dir, stack: stack}
			return
		}
	}

	assign(Build, "build")
	assign(Lint, "lint")
	assign(Typecheck, "typecheck", "type-check")
	assign(Test, "test")
}

// addGoPlan fills in Go's own standard toolchain commands — these are
// the language's own build/vet/test commands, not an invented
// convention, so running them (unlike guessing a script name) doesn't
// need the project to have opted in explicitly the way Node/Makefile
// detection does.
func addGoPlan(plan map[Category]commandSpec, dir string) {
	assign := func(cat Category, spec commandSpec) {
		if _, exists := plan[cat]; exists {
			return
		}
		plan[cat] = spec
	}

	assign(Build, commandSpec{argv: []string{"go", "build", "./..."}, workDir: dir, stack: "Go"})

	if _, err := exec.LookPath("golangci-lint"); err == nil {
		assign(Lint, commandSpec{argv: []string{"golangci-lint", "run", "./..."}, workDir: dir, stack: "Go (golangci-lint)"})
	} else {
		assign(Lint, commandSpec{argv: []string{"go", "vet", "./..."}, workDir: dir, stack: "Go (go vet — golangci-lint not found on PATH)"})
	}

	// Go type-checks as part of compiling — there is no separate
	// typecheck step to run a second time.
	assign(Typecheck, commandSpec{mirrors: Build, stack: "Go"})

	assign(Test, commandSpec{
		argv: []string{"go", "test", "./..."}, workDir: dir, stack: "Go",
		interpret: interpretGoTest,
	})
}

// interpretGoTest catches the one case `go test`'s own exit code can't
// distinguish from a real pass: it exits 0 both when every test
// genuinely passed AND when there are no test files anywhere in the
// module to run at all. Reporting PASS in the second case would be
// exactly the overclaim this whole tool exists to refuse — "tests
// passed" when zero tests actually executed.
func interpretGoTest(err error, output string) (Verdict, string, bool) {
	if err != nil {
		return "", "", false // a real failure — the default exit-code handling already reports this correctly
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "ok") {
			return "", "", false // at least one package actually ran and passed
		}
	}
	return NoProof, "`go test ./...` ran clean, but no package in the module had any test files to run — not proof anything actually passed", true
}

func pythonBin() string {
	if p, err := exec.LookPath("python3"); err == nil {
		return p
	}
	return "python"
}

func hasBuildSystem(dir string) bool {
	raw, err := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if err != nil {
		return false
	}
	return strings.Contains(string(raw), "[build-system]")
}

// hasModule checks whether a Python module is importable in whatever
// environment `py` resolves to, without ever touching the network — a
// plain `import`, nothing installed or fetched.
func hasModule(py, mod string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, py, "-c", "import "+mod).Run() == nil
}

// hasPytestSignal reports whether dir actually looks like it has a test
// suite at all — running pytest with zero real signal it applies isn't
// safe to assume clean (pytest itself exits nonzero when it collects no
// tests, which would misreport as a real FAIL).
func hasPytestSignal(dir string) bool {
	if fileExists(filepath.Join(dir, "tests")) || fileExists(filepath.Join(dir, "conftest.py")) || fileExists(filepath.Join(dir, "pytest.ini")) {
		return true
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "test_*.py")); len(matches) > 0 {
		return true
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*_test.py"))
	return len(matches) > 0
}

// addPythonPlan fills in whatever Python tooling is actually on PATH and
// actually applicable — never assumes a tool is meant to be used just
// because a manifest exists, and never invents a build step most Python
// applications (as opposed to packaged libraries) don't have at all.
func addPythonPlan(plan map[Category]commandSpec, dir string) {
	assign := func(cat Category, spec commandSpec) {
		if _, exists := plan[cat]; exists {
			return
		}
		plan[cat] = spec
	}

	py := pythonBin()

	if hasBuildSystem(dir) {
		if hasModule(py, "build") {
			assign(Build, commandSpec{argv: []string{py, "-m", "build", "--no-isolation"}, workDir: dir, stack: "Python"})
		} else {
			assign(Build, commandSpec{stack: "Python", unrunnable: "pyproject.toml declares a [build-system], but Python's `build` module isn't installed — no way to safely verify"})
		}
	}
	// else: leave Build undecided — most Python applications genuinely
	// have no build step, and inventing one would be exactly the kind of
	// guess this tool refuses to make. It still gets an honest NO PROOF
	// row explaining why, same as any other undetected category.

	if p, err := exec.LookPath("ruff"); err == nil {
		assign(Lint, commandSpec{argv: []string{p, "check", "."}, workDir: dir, stack: "Python (ruff)"})
	} else if p, err := exec.LookPath("flake8"); err == nil {
		assign(Lint, commandSpec{argv: []string{p, "."}, workDir: dir, stack: "Python (flake8)"})
	}

	if p, err := exec.LookPath("mypy"); err == nil {
		assign(Typecheck, commandSpec{argv: []string{p, "."}, workDir: dir, stack: "Python (mypy)"})
	} else if p, err := exec.LookPath("pyright"); err == nil {
		assign(Typecheck, commandSpec{argv: []string{p}, workDir: dir, stack: "Python (pyright)"})
	}

	if hasPytestSignal(dir) {
		if p, err := exec.LookPath("pytest"); err == nil {
			assign(Test, commandSpec{argv: []string{p, "-q"}, workDir: dir, stack: "Python (pytest)"})
		}
	}
}
