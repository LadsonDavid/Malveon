// Package report renders check results as a plain terminal table — no
// fixed format beyond "one row per feature/finding, verdict, reason,
// evidence."
package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/LadsonDavid/beta-test/internal/checks/confidence"
	"github.com/LadsonDavid/beta-test/internal/checks/contract"
	"github.com/LadsonDavid/beta-test/internal/checks/heroact"
	"github.com/LadsonDavid/beta-test/internal/checks/overlap"
	"github.com/LadsonDavid/beta-test/internal/checks/planauthority"
	"github.com/LadsonDavid/beta-test/internal/checks/wiring"
)

func WriteWiring(w io.Writer, results []wiring.Result) {
	fmt.Fprintln(w, "WIRING CHECK — does a frontend action actually reach a real backend route")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	pass, fail, notTested := 0, 0, 0
	for _, r := range results {
		fmt.Fprintf(w, "[%s] %s (%s)\n", r.Verdict, r.FeatureName, r.FeatureID)
		fmt.Fprintf(w, "  reason: %s\n", r.Reason)
		for _, e := range r.Evidence {
			fmt.Fprintf(w, "  evidence: %s\n", e)
		}
		fmt.Fprintln(w)
		switch r.Verdict {
		case wiring.Pass:
			pass++
		case wiring.Fail:
			fail++
		case wiring.NotTested:
			notTested++
		}
	}
	fmt.Fprintf(w, "%d PASS, %d FAIL, %d NOT TESTED\n\n", pass, fail, notTested)
}

func WriteContract(w io.Writer, results []contract.Result) {
	fmt.Fprintln(w, "CONTRACT CHECK — does the wired call agree with the route on HTTP method")
	fmt.Fprintln(w, "(shape-level only — a MATCH here is not a claim that the business result is correct)")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	match, mismatch, notTested := 0, 0, 0
	for _, r := range results {
		fmt.Fprintf(w, "[%s] %s (%s)\n", r.Verdict, r.FeatureName, r.FeatureID)
		fmt.Fprintf(w, "  reason: %s\n", r.Reason)
		for _, e := range r.Evidence {
			fmt.Fprintf(w, "  evidence: %s\n", e)
		}
		fmt.Fprintln(w)
		switch r.Verdict {
		case contract.Match:
			match++
		case contract.Mismatch:
			mismatch++
		case contract.NotTested:
			notTested++
		}
	}
	fmt.Fprintf(w, "%d MATCH, %d MISMATCH, %d NOT TESTED\n\n", match, mismatch, notTested)
}

func WritePlanAuthority(w io.Writer, res planauthority.Result) {
	fmt.Fprintln(w, "NOT-IN-PLAN CHECK — code changed this session with no matching plan entry")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if !res.Available {
		fmt.Fprintf(w, "SKIPPED: %s\n\n", res.Reason)
		return
	}
	if len(res.Findings) == 0 {
		fmt.Fprintln(w, "nothing flagged — every route/call changed this session matches a plan entry")
		fmt.Fprintln(w)
		return
	}
	for _, f := range res.Findings {
		fmt.Fprintf(w, "[NOT IN PLAN] %s %s\n", f.Kind, f.Path)
		if f.Method != "" {
			fmt.Fprintf(w, "  method: %s\n", f.Method)
		}
		fmt.Fprintf(w, "  evidence: %s:%d\n", f.File, f.Line)
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%d NOT IN PLAN\n\n", len(res.Findings))
}

func WriteHeroAct(w io.Writer, res heroact.Result) {
	fmt.Fprintln(w, "HERO-ACT CHECK — did the agent \"find\" a bug it introduced itself this session")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if !res.Available {
		fmt.Fprintf(w, "SKIPPED: %s\n\n", res.Reason)
		return
	}
	if len(res.Findings) == 0 {
		fmt.Fprintln(w, "no self-reported bugs to check (empty or missing --bugs-reported input)")
		fmt.Fprintln(w)
		return
	}
	selfIntro, preExisting, notResolved := 0, 0, 0
	for _, f := range res.Findings {
		fmt.Fprintf(w, "[%s]\n", f.Verdict)
		fmt.Fprintf(w, "  reported: %s\n", f.ReportLine)
		fmt.Fprintf(w, "  reason: %s\n", f.Reason)
		fmt.Fprintln(w)
		switch f.Verdict {
		case heroact.SelfIntroduced:
			selfIntro++
		case heroact.PreExisting:
			preExisting++
		case heroact.NotResolved:
			notResolved++
		}
	}
	fmt.Fprintf(w, "%d SELF-INTRODUCED, %d PRE-EXISTING, %d NOT RESOLVED\n\n", selfIntro, preExisting, notResolved)
}

func WriteOverlap(w io.Writer, findings []overlap.Finding) {
	fmt.Fprintln(w, "OVERLAP CHECK — two or more route registrations claiming the same method+path")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if len(findings) == 0 {
		fmt.Fprintln(w, "nothing flagged — no colliding route registrations found")
		fmt.Fprintln(w)
		return
	}
	for _, f := range findings {
		fmt.Fprintf(w, "[OVERLAP] %s %s — registered %d times\n", f.Method, f.Path, len(f.Nodes))
		for _, n := range f.Nodes {
			fmt.Fprintf(w, "  evidence: %s:%d\n", n.File, n.Line)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%d OVERLAP\n\n", len(findings))
}

func WriteConfidence(w io.Writer, report confidence.Report) {
	fmt.Fprintln(w, "CONFIDENCE CHECK — does the agent's own claim match what was actually verified")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if !report.Available {
		fmt.Fprintf(w, "SKIPPED: %s\n\n", report.Reason)
		return
	}
	confirmed, mismatch, notClaimed := 0, 0, 0
	for _, r := range report.Results {
		fmt.Fprintf(w, "[%s] %s (%s)\n", r.Verdict, r.FeatureName, r.FeatureID)
		if r.ClaimLine != "" {
			fmt.Fprintf(w, "  claimed: %s\n", r.ClaimLine)
		}
		fmt.Fprintf(w, "  reason: %s\n", r.Reason)
		fmt.Fprintln(w)
		switch r.Verdict {
		case confidence.Confirmed:
			confirmed++
		case confidence.Mismatch:
			mismatch++
		case confidence.NotClaimed:
			notClaimed++
		}
	}
	fmt.Fprintf(w, "%d CONFIRMED, %d CONFIDENCE MISMATCH, %d NOT TESTED\n\n", confirmed, mismatch, notClaimed)
}
