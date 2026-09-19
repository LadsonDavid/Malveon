// Command malveon reads a plan and checks whether the code backs it up —
// see CLAUDE.md in the repo root for the full spec.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/LadsonDavid/beta-test/internal/checks/commands"
	"github.com/LadsonDavid/beta-test/internal/checks/confidence"
	"github.com/LadsonDavid/beta-test/internal/checks/contract"
	"github.com/LadsonDavid/beta-test/internal/checks/heropatterns"
	"github.com/LadsonDavid/beta-test/internal/checks/incompleteness"
	"github.com/LadsonDavid/beta-test/internal/checks/overlap"
	"github.com/LadsonDavid/beta-test/internal/checks/planauthority"
	"github.com/LadsonDavid/beta-test/internal/checks/uioverlap"
	"github.com/LadsonDavid/beta-test/internal/checks/wiring"
	"github.com/LadsonDavid/beta-test/internal/extractor"
	"github.com/LadsonDavid/beta-test/internal/features"
	"github.com/LadsonDavid/beta-test/internal/gate"
	"github.com/LadsonDavid/beta-test/internal/report"
	"github.com/LadsonDavid/beta-test/internal/session"
	"github.com/LadsonDavid/beta-test/internal/telemetry"
	"github.com/LadsonDavid/beta-test/internal/watch"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "check":
		runCheck(os.Args[2:])
	case "session":
		runSession(os.Args[2:])
	case "watch":
		runWatch(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  malveon session start [--root <path>]")
	fmt.Fprintln(os.Stderr, "  malveon watch [--root <path>]")
	fmt.Fprintln(os.Stderr, "    Run this in the background before the agent's task begins, so the")
	fmt.Fprintln(os.Stderr, "    hero-act check has real history to check — no self-report needed. Ctrl-C to stop.")
	fmt.Fprintln(os.Stderr, "  malveon check [--features <path>] [--root <path>] [--claimed-summary <path>]")
	fmt.Fprintln(os.Stderr, "                [--exec] [--exec-timeout <duration>] [--no-gate] [--focused-tests]")
	fmt.Fprintln(os.Stderr, "                [--no-telemetry]")
	fmt.Fprintln(os.Stderr, "    --features can be omitted: malveon looks for a plan file automatically,")
	fmt.Fprintln(os.Stderr, "    and asks which one to use if more than one looks right.")
	fmt.Fprintln(os.Stderr, "    --claimed-summary can be omitted too: the confidence check reads commit")
	fmt.Fprintln(os.Stderr, "    messages since session start automatically. Pass it to use something else instead.")
	fmt.Fprintln(os.Stderr, "    --exec opts into also running the project's own real build/lint/typecheck/test")
	fmt.Fprintln(os.Stderr, "    commands (whatever it's defined — Makefile, package.json, Go, or Python tooling);")
	fmt.Fprintln(os.Stderr, "    off by default since it executes real subprocesses and can take a while. malveon")
	fmt.Fprintln(os.Stderr, "    always exits non-zero if a check found something broken/unresolved/unavailable —")
	fmt.Fprintln(os.Stderr, "    so a git pre-commit hook can block on it — with --exec that includes real command")
	fmt.Fprintln(os.Stderr, "    results too. --no-gate keeps the report but always exits 0.")
	fmt.Fprintln(os.Stderr, "    --focused-tests additionally runs tests scoped to this session's changes (Jest/Vitest's")
	fmt.Fprintln(os.Stderr, "    own --changed support, pytest-picked if installed, or Go at package granularity) —")
	fmt.Fprintln(os.Stderr, "    off by default, additive evidence only, never a substitute for the full test result.")
	fmt.Fprintln(os.Stderr, "    check sends one anonymous usage event (OS, arch, clean/blocked — nothing about your")
	fmt.Fprintln(os.Stderr, "    project or code); --no-telemetry or DO_NOT_TRACK=1 turns it off.")
}

func runSession(args []string) {
	if len(args) == 0 || args[0] != "start" {
		usage()
		os.Exit(2)
	}
	fset := flag.NewFlagSet("session start", flag.ExitOnError)
	root := fset.String("root", ".", "repo root")
	fset.Parse(args[1:])

	st, err := session.Start(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("session started at %s\n", st.StartRef)
}

func runWatch(args []string) {
	fset := flag.NewFlagSet("watch", flag.ExitOnError)
	root := fset.String("root", ".", "repo root to watch")
	fset.Parse(args)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("watching %s — Ctrl-C to stop\n", *root)
	if err := watch.Run(ctx, watch.Options{Root: *root}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("stopped")
}

func runCheck(args []string) {
	fset := flag.NewFlagSet("check", flag.ExitOnError)
	featuresPath := fset.String("features", "", "path to the features file: .json, .md (checklist), or plain text (required)")
	root := fset.String("root", ".", "repo root to scan")
	claimedSummary := fset.String("claimed-summary", "", "optional: path to a plain-text file of the agent's own claims (overrides the automatic commit-message scan for the confidence check)")
	runExec := fset.Bool("exec", false, "also run the project's own real build/lint/typecheck/test commands (opt-in — executes real subprocesses and can take a while; off by default)")
	execTimeout := fset.Duration("exec-timeout", commands.DefaultTimeout, "hard per-command timeout for the build/lint/typecheck/test check")
	noGate := fset.Bool("no-gate", false, "still print the full report, but always exit 0 regardless of what was found")
	focusedTests := fset.Bool("focused-tests", false, "additionally run tests scoped to what changed this session (Jest/Vitest --changed, pytest-picked, or Go package-level) — additive only, never replaces the full test result; requires a session start")
	noTelemetry := fset.Bool("no-telemetry", false, "don't send the anonymous check_run usage event (also respects MALVEON_NO_TELEMETRY and DO_NOT_TRACK env vars)")
	fset.Parse(args)

	if *featuresPath == "" {
		candidates, err := features.Detect(*root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fromContentScan := false
		if len(candidates) == 0 {
			// Nothing matched by name — the plan could be called
			// anything. Fall back to checking file content before
			// giving up and asking the user to type a path. A content
			// match is a guess, not a deliberate name, so it always
			// needs confirming — see resolveFeaturesPath.
			candidates, err = features.DetectByContent(*root)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fromContentScan = true
		}
		resolved, err := resolveFeaturesPath(candidates, fromContentScan, os.Stdin, os.Stderr, isInteractive())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			usage()
			os.Exit(2)
		}
		*featuresPath = resolved
	}

	fs, err := features.Load(*featuresPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	g, err := extractor.Extract(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error scanning %s: %v\n", *root, err)
		os.Exit(1)
	}

	// Every check's result is held in a named variable, not just inside a
	// render closure, so the exit-code gate (internal/gate) can inspect
	// the exact same values the report renders — the printed report and
	// the exit code can never disagree about what a run actually found.
	wiringResults := wiring.Run(g, fs)
	contractResults := contract.Run(g, fs)
	overlapFindings := overlap.Run(g, *root)
	planResult := planauthority.Run(g, fs, *root)
	heroReport := heropatterns.Run(*root)
	incompletenessReport := incompleteness.Run(*root)
	confidenceReport := confidence.Run(fs, wiringResults, *root, *claimedSummary)

	uiFindings, uiErr := uioverlap.Run(*root)
	if uiErr != nil {
		fmt.Fprintf(os.Stderr, "error scanning for frontend overlap risk: %v\n", uiErr)
		os.Exit(1)
	}

	// The one check that executes real commands instead of reading the
	// code graph — see CLAUDE.md 3.2.11. Opt-in via --exec; off by
	// default, and the gate below never treats not-requesting-it as a
	// failure, same as any other deliberate opt-out.
	//
	// The overview prints only after every section is computed (see
	// below), and a real build/lint/typecheck/test run can take minutes —
	// without a status line here, the terminal would sit completely silent
	// that whole time and look indistinguishable from a hang. This prints
	// to stderr, not stdout, so it never lands in a redirected/piped report.
	var commandResults []commands.Result
	if *runExec {
		fmt.Fprintf(os.Stderr, "running the project's own build/lint/typecheck/test commands (up to %s per command, can take a few minutes)...\n", *execTimeout)
		commandResults = commands.Run(*root, *execTimeout)
	}

	// Opt-in only (--focused-tests) — additive evidence on top of the
	// full test result above, never a substitute for it. See CLAUDE.md
	// 3.2.11's focused-test note for why this stays opt-in rather than
	// the default.
	var focusedReport commands.FocusedReport
	if *focusedTests {
		fmt.Fprintln(os.Stderr, "running focused tests scoped to this session's changes...")
		focusedReport = commands.RunFocused(*root, *execTimeout)
	}

	// Each section is rendered into its own buffer so the overview (which
	// needs every section's summary) can print first, before any of the
	// detail it summarizes — see internal/report's package doc comment.
	var sections []*bytes.Buffer
	var summaries []report.CheckSummary
	render := func(write func(io.Writer) report.CheckSummary) {
		buf := &bytes.Buffer{}
		summaries = append(summaries, write(buf))
		sections = append(sections, buf)
	}

	render(func(w io.Writer) report.CheckSummary { return report.WriteWiring(w, wiringResults) })
	render(func(w io.Writer) report.CheckSummary { return report.WriteContract(w, contractResults) })
	render(func(w io.Writer) report.CheckSummary { return report.WriteOverlap(w, overlapFindings) })
	render(func(w io.Writer) report.CheckSummary { return report.WriteUIOverlap(w, uiFindings) })
	render(func(w io.Writer) report.CheckSummary { return report.WritePlanAuthority(w, planResult) })
	render(func(w io.Writer) report.CheckSummary { return report.WriteHeroPatterns(w, heroReport) })
	render(func(w io.Writer) report.CheckSummary { return report.WriteIncompleteness(w, incompletenessReport) })
	render(func(w io.Writer) report.CheckSummary { return report.WriteConfidence(w, confidenceReport) })
	render(func(w io.Writer) report.CheckSummary { return report.WriteCommands(w, commandResults, !*runExec) })
	if *focusedTests {
		render(func(w io.Writer) report.CheckSummary { return report.WriteFocusedTests(w, focusedReport) })
	}

	report.WriteOverview(os.Stdout, summaries)
	for _, s := range sections {
		os.Stdout.Write(s.Bytes())
	}

	decision := gate.Evaluate(gate.Input{
		Wiring:          wiringResults,
		Contract:        contractResults,
		Overlap:         overlapFindings,
		PlanAuthority:   planResult,
		HeroPatterns:    heroReport,
		Confidence:      confidenceReport,
		Commands:        commandResults,
		CommandsNotRequested: !*runExec,

		FocusedTests:          focusedReport,
		FocusedTestsRequested: *focusedTests,
	})

	// One anonymous check_run event, best-effort, before the report's
	// own exit paths below — see internal/telemetry's package doc for
	// exactly what this does and doesn't send, and the opt-outs.
	if !telemetry.Disabled(*noTelemetry) {
		telemetry.SendCheckRun(decision.Blocked)
	}

	if !decision.Blocked {
		fmt.Println("GATE: clean — nothing here blocks a commit")
		return
	}

	fmt.Println("GATE: blocked — a commit gated on this run should not proceed")
	for _, r := range decision.Reasons {
		fmt.Printf("  - %s\n", r)
	}
	if *noGate {
		fmt.Println("(--no-gate given: exiting 0 anyway)")
		return
	}
	os.Exit(1)
}
