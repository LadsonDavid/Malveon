// Command malveon reads a plan and checks whether the code backs it up —
// see CLAUDE.md in the repo root for the full spec.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LadsonDavid/beta-test/internal/checks/confidence"
	"github.com/LadsonDavid/beta-test/internal/checks/contract"
	"github.com/LadsonDavid/beta-test/internal/checks/heroact"
	"github.com/LadsonDavid/beta-test/internal/checks/overlap"
	"github.com/LadsonDavid/beta-test/internal/checks/planauthority"
	"github.com/LadsonDavid/beta-test/internal/checks/wiring"
	"github.com/LadsonDavid/beta-test/internal/extractor"
	"github.com/LadsonDavid/beta-test/internal/features"
	"github.com/LadsonDavid/beta-test/internal/report"
	"github.com/LadsonDavid/beta-test/internal/session"
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
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  malveon session start [--root <path>]")
	fmt.Fprintln(os.Stderr, "  malveon check --features <path> [--root <path>] [--bugs-reported <path>] [--claimed-summary <path>]")
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

func runCheck(args []string) {
	fset := flag.NewFlagSet("check", flag.ExitOnError)
	featuresPath := fset.String("features", "", "path to the features file: .json, .md (checklist), or plain text (required)")
	root := fset.String("root", ".", "repo root to scan")
	bugsReported := fset.String("bugs-reported", "", "path to a plain-text file of self-reported bugs (for the hero-act check)")
	claimedSummary := fset.String("claimed-summary", "", "path to a plain-text file of the agent's own claims about what it built (for the confidence check)")
	fset.Parse(args)

	if *featuresPath == "" {
		fmt.Fprintln(os.Stderr, "error: --features is required")
		usage()
		os.Exit(2)
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

	report.WriteWiring(os.Stdout, wiringResults)
	report.WriteContract(os.Stdout, contract.Run(g, fs))
	report.WriteOverlap(os.Stdout, overlap.Run(g))
	report.WritePlanAuthority(os.Stdout, planauthority.Run(g, fs, *root))

	if *bugsReported != "" {
		report.WriteHeroAct(os.Stdout, heroact.Run(*root, *bugsReported))
	} else {
		report.WriteHeroAct(os.Stdout, heroact.Result{Available: false, Reason: "no --bugs-reported file given"})
	}

	report.WriteConfidence(os.Stdout, confidence.Run(fs, wiringResults, *claimedSummary))
}
