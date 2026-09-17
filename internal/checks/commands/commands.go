// Package commands implements the one deliberate real-execution check in
// malveon: does the project's own build/lint/typecheck/test command
// actually pass right now. Every other check in this tool is static
// graph or text analysis; this is the one exception, added 2026-09-17
// after a direct audit against Feeling_Sun_6436's (the first real
// tester's) own stated model of verification — "code changed → checks
// passed → verified on the real host or device." Until this check
// existed, malveon's own idea of "checks passed" was only ever a static
// graph guess; it never actually ran the tester's own build/lint/test
// commands and captured their real result. See CLAUDE.md section 3.2.11
// for why this doesn't contradict the "static-only, no live execution"
// rule in section 2: build/lint/typecheck/test are the project's own
// already-defined, non-interactive commands — nothing is started as a
// server, nothing is rendered in a browser, no deployed app is touched.
// That third state (verified live, on a real host/device) stays exactly
// as deferred as it always was.
//
// Only commands the project already defines are ever run: a Makefile
// target, a package.json script, Go's own standard toolchain commands,
// or a Python tool already on PATH. Nothing here invents a command the
// project didn't already ask for (no guessed tsc invocation, no
// auto-installed linter) — an undetectable category is honestly
// NO PROOF, the same rule as every other check in this tool.
package commands

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Category string

const (
	Build     Category = "build"
	Lint      Category = "lint"
	Typecheck Category = "typecheck"
	Test      Category = "test"

	// FocusedTest is the opt-in, additive category from RunFocused (see
	// focused.go) — never part of Run/categoryOrder above. It's evidence
	// on top of Test's full-suite result, never a substitute for it.
	FocusedTest Category = "focused test"
)

// categoryOrder fixes the run order: Build first because Typecheck (for a
// Go stack, where type-checking is inherent to compilation, not a
// separate step) reuses Build's already-captured result rather than
// compiling the same package twice.
var categoryOrder = []Category{Build, Lint, Typecheck, Test}

type Verdict string

// NoProof deliberately reuses the same "NO PROOF" label every other
// check uses for "we looked and found nothing solid" — this check's
// undetectable-category case is exactly that same situation, just for a
// real command instead of a graph path.
const (
	Pass    Verdict = "PASS"
	Fail    Verdict = "FAIL"
	NoProof Verdict = "NO PROOF"
)

// Result is one category's outcome for one detected toolchain directory
// (a repo can have more than one — e.g. a Node frontend and a Go backend
// in separate directories — so Dir/Stack identify which).
type Result struct {
	Category Category
	Stack    string // "Makefile", "Node (npm)", "Go", "Python", ...
	Dir      string // relative to root; "." for the project root
	Command  string // the real command that ran; "" if none was found
	Verdict  Verdict
	Reason   string
	Duration time.Duration
	Output   string // tail of captured combined stdout+stderr, truncated
}

const (
	// DefaultTimeout is the hard per-command cap (the Timeout pattern —
	// see CLAUDE.md 3.2.11) applied when the caller doesn't override it.
	// One fixed cap rather than a per-category tuned one, deliberately —
	// see the package doc's stated-limitation note in CLAUDE.md.
	DefaultTimeout = 3 * time.Minute
	outputCap      = 4000 // characters kept from the tail of a command's output
)

// Run detects every toolchain directory in the project (walking the whole
// tree the same way the extractor does, skipping the same noise
// directories) and, for each of build/lint/typecheck/test, runs whatever
// real command that toolchain defines. Each command is isolated with its
// own hard timeout and its own subprocess, so one hanging or failing
// command can never stop the others from running — the Bulkhead pattern:
// a build timing out must never cost the tester their lint/test results
// too.
func Run(root string, timeout time.Duration) []Result {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	dirs := detectToolchainDirs(root)
	var results []Result
	for _, dir := range dirs {
		plan := planFor(dir)
		built := map[Category]Result{}
		for _, cat := range categoryOrder {
			spec, ok := plan[cat]
			switch {
			case !ok:
				results = append(results, Result{
					Category: cat, Stack: dir.stackLabel(), Dir: dir.rel,
					Verdict: NoProof, Reason: noCommandReason(dir, cat),
				})
			case spec.mirrors != "":
				// Go's typecheck: not a separate command — reuse the
				// already-executed build result rather than compiling
				// the same package twice for no new information.
				if b, ok := built[Build]; ok {
					r := b
					r.Category = cat
					r.Reason = "Go type-checks as part of compilation — no separate step; mirrors the build result above"
					results = append(results, r)
				} else {
					results = append(results, Result{
						Category: cat, Stack: spec.stack, Dir: dir.rel,
						Verdict: NoProof, Reason: "build result unavailable to mirror for type-checking",
					})
				}
			case spec.unrunnable != "":
				results = append(results, Result{
					Category: cat, Stack: spec.stack, Dir: dir.rel,
					Verdict: NoProof, Reason: spec.unrunnable,
				})
			default:
				r := execute(dir.rel, cat, spec, timeout)
				built[cat] = r
				results = append(results, r)
			}
		}
	}
	return results
}

// execute runs one real command to completion or until timeout, whichever
// comes first — the Timeout pattern. A command that times out is neither
// proven to pass nor proven to fail: it's cut off before finishing, so
// the honest verdict is NO PROOF, not a guess in either direction.
func execute(dir string, cat Category, spec commandSpec, timeout time.Duration) Result {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, spec.argv[0], spec.argv[1:]...)
	cmd.Dir = spec.workDir
	setNewProcessGroup(cmd)
	cmd.Cancel = func() error { return killProcessTree(cmd) }
	cmd.WaitDelay = 5 * time.Second // give a killed process tree a moment to exit before Wait gives up

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	out := tail(buf.String(), outputCap)
	command := strings.Join(spec.argv, " ")

	if ctx.Err() == context.DeadlineExceeded {
		return Result{
			Category: cat, Stack: spec.stack, Dir: dir, Command: command,
			Verdict:  NoProof,
			Reason:   fmt.Sprintf("timed out after %s and was killed — not proven to pass or fail, just cut off", timeout),
			Duration: elapsed, Output: out,
		}
	}

	if spec.interpret != nil {
		if v, reason, ok := spec.interpret(err, buf.String()); ok {
			return Result{
				Category: cat, Stack: spec.stack, Dir: dir, Command: command,
				Verdict: v, Reason: reason, Duration: elapsed, Output: out,
			}
		}
	}

	if err != nil {
		return Result{
			Category: cat, Stack: spec.stack, Dir: dir, Command: command,
			Verdict:  Fail,
			Reason:   fmt.Sprintf("`%s` exited with an error: %v", command, err),
			Duration: elapsed, Output: out,
		}
	}
	return Result{
		Category: cat, Stack: spec.stack, Dir: dir, Command: command,
		Verdict:  Pass,
		Reason:   fmt.Sprintf("`%s` ran and exited clean", command),
		Duration: elapsed, Output: out,
	}
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…(truncated)…\n" + s[len(s)-n:]
}
