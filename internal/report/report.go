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
	"github.com/LadsonDavid/beta-test/internal/checks/heropatterns"
	"github.com/LadsonDavid/beta-test/internal/checks/incompleteness"
	"github.com/LadsonDavid/beta-test/internal/checks/overlap"
	"github.com/LadsonDavid/beta-test/internal/checks/planauthority"
	"github.com/LadsonDavid/beta-test/internal/checks/uioverlap"
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
	fmt.Fprintln(w, "CONTRACT CHECK — does the wired call agree with the route on HTTP method and request fields")
	fmt.Fprintln(w, "(shape-level only — a MATCH here is not a claim that the business result is correct;")
	fmt.Fprintln(w, " body-field agreement is JS/TS-only, and only checked when both sides are a literal object)")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	match, mismatch, notTested := 0, 0, 0
	for _, r := range results {
		fmt.Fprintf(w, "[method: %s] %s (%s)\n", r.Verdict, r.FeatureName, r.FeatureID)
		fmt.Fprintf(w, "  reason: %s\n", r.Reason)
		for _, e := range r.Evidence {
			fmt.Fprintf(w, "  evidence: %s\n", e)
		}
		if r.BodyVerdict != "" {
			fmt.Fprintf(w, "  [fields: %s] %s\n", r.BodyVerdict, r.BodyReason)
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
	fmt.Fprintf(w, "%d MATCH, %d MISMATCH, %d NOT TESTED (method agreement)\n\n", match, mismatch, notTested)
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

func WriteHeroPatterns(w io.Writer, res heropatterns.Report) {
	fmt.Fprintln(w, "HERO-ACT CHECK — known bug patterns introduced and fixed this session, from captured history")
	fmt.Fprintln(w, "(no self-report, no commits — requires `malveon watch` to have been running)")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if !res.Available {
		fmt.Fprintf(w, "SKIPPED: %s\n\n", res.Reason)
		return
	}
	if len(res.Findings) == 0 {
		fmt.Fprintln(w, "nothing flagged — no known bug pattern appeared and then disappeared this session")
		fmt.Fprintln(w)
		return
	}
	for _, f := range res.Findings {
		fmt.Fprintf(w, "[SELF-INTRODUCED, FOUND & FIXED SAME SESSION] %s — %s\n", f.File, f.Pattern)
		fmt.Fprintf(w, "  reason: %s\n", f.Reason)
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%d SELF-INTRODUCED\n\n", len(res.Findings))
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
		for i, n := range f.Nodes {
			fmt.Fprintf(w, "  evidence: %s:%d\n", n.File, n.Line)
			if i < len(f.Notes) && f.Notes[i] != "" {
				fmt.Fprintf(w, "    note: %s\n", f.Notes[i])
			}
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%d OVERLAP\n\n", len(findings))
}

func WriteUIOverlap(w io.Writer, findings []uioverlap.Finding) {
	fmt.Fprintln(w, "FRONTEND OVERLAP RISK — positioned elements with no positioning context in the file")
	fmt.Fprintln(w, "(structural risk only — this is not a claim that two elements actually overlap on screen;")
	fmt.Fprintln(w, " confirming that needs a real render, which this check deliberately doesn't do)")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if len(findings) == 0 {
		fmt.Fprintln(w, "nothing flagged — no positioned elements without a positioning context found")
		fmt.Fprintln(w)
		return
	}
	for _, f := range findings {
		fmt.Fprintf(w, "[STRUCTURAL RISK] %s:%d\n", f.File, f.Line)
		fmt.Fprintf(w, "  classes: %s\n", f.Classes)
		fmt.Fprintf(w, "  reason: %s\n", f.Reason)
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%d STRUCTURAL RISK\n\n", len(findings))
}

func WriteIncompleteness(w io.Writer, res incompleteness.Report) {
	fmt.Fprintln(w, "INCOMPLETENESS CHECK — TODO/FIXME/HACK/XXX markers left in code changed this session")
	fmt.Fprintln(w, "(code-only signal: presence is real proof the code admits a gap; absence proves nothing —")
	fmt.Fprintln(w, " this can never substitute for the confidence check above, which tests an actual claim)")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if !res.Available {
		fmt.Fprintf(w, "SKIPPED: %s\n\n", res.Reason)
		return
	}
	if len(res.Findings) == 0 {
		fmt.Fprintln(w, "nothing flagged — no incompleteness marker found in files changed this session")
		fmt.Fprintln(w)
		return
	}
	for _, f := range res.Findings {
		fmt.Fprintf(w, "[%s] %s:%d\n", f.Marker, f.File, f.Line)
		fmt.Fprintf(w, "  %s\n", f.Text)
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%d FLAGGED\n\n", len(res.Findings))
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
