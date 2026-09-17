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
	"github.com/LadsonDavid/beta-test/internal/report"
	"github.com/LadsonDavid/beta-test/internal/session"
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
	fmt.Fprintln(os.Stderr, "    --features can be omitted: malveon looks for a plan file automatically,")
	fmt.Fprintln(os.Stderr, "    and asks which one to use if more than one looks right.")
	fmt.Fprintln(os.Stderr, "    --claimed-summary can be omitted too: the confidence check reads commit")
	fmt.Fprintln(os.Stderr, "    messages since session start automatically. Pass it to use something else instead.")
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

	wiringResults := wiring.Run(g, fs)

	uiFindings, uiErr := uioverlap.Run(*root)
	if uiErr != nil {
		fmt.Fprintf(os.Stderr, "error scanning for frontend overlap risk: %v\n", uiErr)
		os.Exit(1)
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
	render(func(w io.Writer) report.CheckSummary { return report.WriteContract(w, contract.Run(g, fs)) })
	render(func(w io.Writer) report.CheckSummary { return report.WriteOverlap(w, overlap.Run(g, *root)) })
	render(func(w io.Writer) report.CheckSummary { return report.WriteUIOverlap(w, uiFindings) })
	render(func(w io.Writer) report.CheckSummary { return report.WritePlanAuthority(w, planauthority.Run(g, fs, *root)) })
	render(func(w io.Writer) report.CheckSummary { return report.WriteHeroPatterns(w, heropatterns.Run(*root)) })
	render(func(w io.Writer) report.CheckSummary { return report.WriteIncompleteness(w, incompleteness.Run(*root)) })
	render(func(w io.Writer) report.CheckSummary {
		return report.WriteConfidence(w, confidence.Run(fs, wiringResults, *root, *claimedSummary))
	})

	report.WriteOverview(os.Stdout, summaries)
	for _, s := range sections {
		os.Stdout.Write(s.Bytes())
	}
}
