// Package report renders check results for the terminal.
//
// Layout rules (redesigned 2026-09-17 after a real user's complaint that a
// long codebase's output was unreadable — a flat list of 43 near-identical
// blocks, in plan order, with the real problems buried among them):
//
//   - An overview prints first: one line per check, so the whole result is
//     visible before scrolling into any detail (Krug's "big picture at a
//     glance" — a reader should never have to read the whole report just to
//     know whether anything is wrong).
//   - Within a check, findings are grouped by what they mean, not left in
//     plan order: proven-broken gets full detail (rare, needs a human);
//     proven-fine is compacted to one line each (common, just needs
//     confirming); unresolved/"NO PROOF" is grouped by its reason instead
//     of repeating the same explanatory sentence after every single item —
//     in a real 43-feature run, 36 of them shared one of four reasons, so
//     this alone collapses ~150 lines into about a dozen.
//   - Findings that repeat per-file (frontend overlap risk, incompleteness
//     markers) are grouped under the file once instead of repeating the
//     file path and reason sentence for every line in it.
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

// CheckSummary is one line of the overview: which check, and its result
// in one glance (e.g. "7 PASS · 0 FAIL · 36 NO PROOF", or "clean").
type CheckSummary struct {
	Label string
	Line  string
}

// WriteOverview prints one line per check before any per-check detail —
// the reader should be able to tell whether anything needs attention
// without reading past this block.
func WriteOverview(w io.Writer, summaries []CheckSummary) {
	fmt.Fprintln(w, "OVERVIEW")
	fmt.Fprintln(w, strings.Repeat("=", 78))
	for _, s := range summaries {
		fmt.Fprintf(w, "  %-13s %s\n", s.Label, s.Line)
	}
	fmt.Fprintln(w)
}

// groupByKey buckets items by a string key, preserving first-seen order
// of the keys — used everywhere below to turn "same reason, N times" into
// one header plus a compact list instead of N repeated explanations.
func groupByKey[T any](items []T, key func(T) string) (order []string, groups map[string][]T) {
	groups = make(map[string][]T)
	for _, it := range items {
		k := key(it)
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], it)
	}
	return order, groups
}

func WriteWiring(w io.Writer, results []wiring.Result) CheckSummary {
	fmt.Fprintln(w, "WIRING — does a frontend action actually reach a real backend route?")
	fmt.Fprintln(w, strings.Repeat("-", 78))

	var fails, passes, noProof []wiring.Result
	for _, r := range results {
		switch r.Verdict {
		case wiring.Fail:
			fails = append(fails, r)
		case wiring.Pass:
			passes = append(passes, r)
		default:
			noProof = append(noProof, r)
		}
	}

	if len(fails) > 0 {
		fmt.Fprintf(w, "FAIL (%d) — proven broken, look at these first\n", len(fails))
		for _, r := range fails {
			fmt.Fprintf(w, "  %s\n", r.FeatureName)
			fmt.Fprintf(w, "    %s\n", r.Reason)
			for _, e := range r.Evidence {
				fmt.Fprintf(w, "    evidence: %s\n", e)
			}
		}
		fmt.Fprintln(w)
	}

	if len(passes) > 0 {
		fmt.Fprintf(w, "PASS (%d) — real evidence found\n", len(passes))
		for _, r := range passes {
			fmt.Fprintf(w, "  %s — %s\n", r.FeatureName, r.Reason)
			if len(r.Evidence) == 2 {
				fmt.Fprintf(w, "    %s -> %s\n", r.Evidence[0], r.Evidence[1])
			}
		}
		fmt.Fprintln(w)
	}

	if len(noProof) > 0 {
		fmt.Fprintf(w, "NO PROOF (%d) — nothing solid either way, grouped by why\n", len(noProof))
		order, groups := groupByKey(noProof, func(r wiring.Result) string { return r.Reason })
		for _, reason := range order {
			items := groups[reason]
			fmt.Fprintf(w, "  %s (%d)\n", reason, len(items))
			for _, r := range items {
				fmt.Fprintf(w, "    - %s\n", r.FeatureName)
			}
		}
		fmt.Fprintln(w)
	}

	return CheckSummary{
		Label: "WIRING",
		Line:  fmt.Sprintf("%d PASS · %d FAIL · %d NO PROOF", len(passes), len(fails), len(noProof)),
	}
}

func WriteContract(w io.Writer, results []contract.Result) CheckSummary {
	fmt.Fprintln(w, "CONTRACT — does the wired call agree with the route on HTTP method and request fields?")
	fmt.Fprintln(w, "(shape-level only — a MATCH here is not a claim that the business result is correct;")
	fmt.Fprintln(w, " body-field agreement is JS/TS-only, and only checked when both sides are a literal object)")
	fmt.Fprintln(w, strings.Repeat("-", 78))

	isBad := func(r contract.Result) bool {
		return r.Verdict == contract.Mismatch || r.BodyVerdict == contract.Mismatch
	}

	var bad, good, noProof []contract.Result
	match, mismatch, notTested := 0, 0, 0
	for _, r := range results {
		switch r.Verdict {
		case contract.Match:
			match++
		case contract.Mismatch:
			mismatch++
		default:
			notTested++
		}
		switch {
		case isBad(r):
			bad = append(bad, r)
		case r.Verdict == contract.Match:
			good = append(good, r)
		default:
			noProof = append(noProof, r)
		}
	}

	if len(bad) > 0 {
		fmt.Fprintf(w, "MISMATCH (%d) — method or request fields disagree, look at these first\n", len(bad))
		for _, r := range bad {
			fmt.Fprintf(w, "  %s\n", r.FeatureName)
			fmt.Fprintf(w, "    method: %s — %s\n", r.Verdict, r.Reason)
			for _, e := range r.Evidence {
				fmt.Fprintf(w, "    evidence: %s\n", e)
			}
			if r.BodyVerdict != "" {
				fmt.Fprintf(w, "    fields: %s — %s\n", r.BodyVerdict, r.BodyReason)
			}
		}
		fmt.Fprintln(w)
	}

	if len(good) > 0 {
		fmt.Fprintf(w, "MATCH (%d) — method and fields agree\n", len(good))
		for _, r := range good {
			line := fmt.Sprintf("  %s — %s", r.FeatureName, r.Reason)
			if r.BodyVerdict == contract.Match {
				line += " · fields MATCH"
			}
			fmt.Fprintln(w, line)
			if len(r.Evidence) == 2 {
				fmt.Fprintf(w, "    %s -> %s\n", r.Evidence[0], r.Evidence[1])
			}
		}
		fmt.Fprintln(w)
	}

	if len(noProof) > 0 {
		fmt.Fprintf(w, "NO PROOF (%d) — nothing to check the contract of, grouped by why\n", len(noProof))
		order, groups := groupByKey(noProof, func(r contract.Result) string { return r.Reason })
		for _, reason := range order {
			items := groups[reason]
			fmt.Fprintf(w, "  %s (%d)\n", reason, len(items))
			for _, r := range items {
				fmt.Fprintf(w, "    - %s\n", r.FeatureName)
			}
		}
		fmt.Fprintln(w)
	}

	return CheckSummary{
		Label: "CONTRACT",
		Line:  fmt.Sprintf("%d MATCH · %d MISMATCH · %d NO PROOF", match, mismatch, notTested),
	}
}

func WritePlanAuthority(w io.Writer, res planauthority.Result) CheckSummary {
	fmt.Fprintln(w, "NOT-IN-PLAN — code changed this session with no matching plan entry")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if !res.Available {
		fmt.Fprintf(w, "SKIPPED: %s\n\n", res.Reason)
		return CheckSummary{Label: "NOT-IN-PLAN", Line: "SKIPPED"}
	}
	if len(res.Findings) == 0 {
		fmt.Fprintln(w, "nothing flagged — every route/call changed this session matches a plan entry")
		fmt.Fprintln(w)
		return CheckSummary{Label: "NOT-IN-PLAN", Line: "clean"}
	}
	for _, f := range res.Findings {
		fmt.Fprintf(w, "  %s %s\n", f.Kind, f.Path)
		if f.Method != "" {
			fmt.Fprintf(w, "    method: %s\n", f.Method)
		}
		fmt.Fprintf(w, "    evidence: %s:%d\n", f.File, f.Line)
	}
	fmt.Fprintln(w)
	return CheckSummary{Label: "NOT-IN-PLAN", Line: fmt.Sprintf("%d flagged", len(res.Findings))}
}

func WriteHeroPatterns(w io.Writer, res heropatterns.Report) CheckSummary {
	fmt.Fprintln(w, "HERO-ACT — known bug patterns this session's own code introduced, no self-report")
	fmt.Fprintln(w, "(git baseline always runs once a session started; `malveon watch` adds detection for a")
	fmt.Fprintln(w, " pattern that appeared and disappeared entirely within the session)")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if !res.Available {
		fmt.Fprintf(w, "SKIPPED: %s\n\n", res.Reason)
		return CheckSummary{Label: "HERO-ACT", Line: "SKIPPED"}
	}
	if !res.WatchAvailable {
		fmt.Fprintf(w, "note: %s — \"introduced and fixed within the session\" detection didn't run this time\n\n", res.WatchReason)
	}
	if len(res.Findings) == 0 {
		fmt.Fprintln(w, "nothing flagged — no known bug pattern was introduced this session")
		fmt.Fprintln(w)
		return CheckSummary{Label: "HERO-ACT", Line: "clean"}
	}
	for _, f := range res.Findings {
		fmt.Fprintf(w, "  [%s] %s — %s\n", f.Status, f.File, f.Pattern)
		fmt.Fprintf(w, "    %s\n", f.Reason)
	}
	fmt.Fprintln(w)
	return CheckSummary{Label: "HERO-ACT", Line: fmt.Sprintf("%d found", len(res.Findings))}
}

func WriteOverlap(w io.Writer, findings []overlap.Finding) CheckSummary {
	fmt.Fprintln(w, "OVERLAP — two or more route registrations claiming the same method+path")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if len(findings) == 0 {
		fmt.Fprintln(w, "nothing flagged — no colliding route registrations found")
		fmt.Fprintln(w)
		return CheckSummary{Label: "OVERLAP", Line: "clean"}
	}
	for _, f := range findings {
		fmt.Fprintf(w, "  %s %s — registered %d times\n", f.Method, f.Path, len(f.Nodes))
		for i, n := range f.Nodes {
			fmt.Fprintf(w, "    %s:%d\n", n.File, n.Line)
			if i < len(f.Notes) && f.Notes[i] != "" {
				fmt.Fprintf(w, "      note: %s\n", f.Notes[i])
			}
		}
	}
	fmt.Fprintln(w)
	return CheckSummary{Label: "OVERLAP", Line: fmt.Sprintf("%d collision(s)", len(findings))}
}

// WriteUIOverlap groups findings by file (the same risk sentence used to
// repeat once per finding — a file with 4 flagged lines printed the same
// paragraph 4 times). Findings within a reason group are further grouped
// by file, since a file with several flagged elements is one place to go
// fix, not several unrelated ones.
func WriteUIOverlap(w io.Writer, findings []uioverlap.Finding) CheckSummary {
	fmt.Fprintln(w, "FRONTEND OVERLAP RISK — positioned elements with no positioning context in the file")
	fmt.Fprintln(w, "(structural risk only — this is not a claim that two elements actually overlap on screen;")
	fmt.Fprintln(w, " confirming that needs a real render, which this check deliberately doesn't do)")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if len(findings) == 0 {
		fmt.Fprintln(w, "nothing flagged — no positioned elements without a positioning context found")
		fmt.Fprintln(w)
		return CheckSummary{Label: "UI OVERLAP", Line: "clean"}
	}

	reasonOrder, reasonGroups := groupByKey(findings, func(f uioverlap.Finding) string { return f.Reason })
	files := map[string]bool{}
	for _, reason := range reasonOrder {
		items := reasonGroups[reason]
		fmt.Fprintf(w, "%s (%d)\n", reason, len(items))
		fileOrder, fileGroups := groupByKey(items, func(f uioverlap.Finding) string { return f.File })
		for _, file := range fileOrder {
			files[file] = true
			for _, f := range fileGroups[file] {
				fmt.Fprintf(w, "  %s:%d — %s\n", f.File, f.Line, f.Classes)
			}
		}
		fmt.Fprintln(w)
	}
	return CheckSummary{
		Label: "UI OVERLAP",
		Line:  fmt.Sprintf("%d risk across %d file(s)", len(findings), len(files)),
	}
}

// WriteIncompleteness groups markers by file for the same reason
// WriteUIOverlap does — a file with several TODOs is one stop, not several.
func WriteIncompleteness(w io.Writer, res incompleteness.Report) CheckSummary {
	fmt.Fprintln(w, "INCOMPLETENESS — TODO/FIXME/HACK/XXX markers left in code changed this session")
	fmt.Fprintln(w, "(code-only signal: presence is real proof the code admits a gap; absence proves nothing —")
	fmt.Fprintln(w, " this can never substitute for the confidence check below, which tests an actual claim)")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if !res.Available {
		fmt.Fprintf(w, "SKIPPED: %s\n\n", res.Reason)
		return CheckSummary{Label: "INCOMPLETE", Line: "SKIPPED"}
	}
	if len(res.Findings) == 0 {
		fmt.Fprintln(w, "nothing flagged — no incompleteness marker found in files changed this session")
		fmt.Fprintln(w)
		return CheckSummary{Label: "INCOMPLETE", Line: "clean"}
	}
	fileOrder, fileGroups := groupByKey(res.Findings, func(f incompleteness.Finding) string { return f.File })
	for _, file := range fileOrder {
		items := fileGroups[file]
		fmt.Fprintf(w, "%s (%d)\n", file, len(items))
		for _, f := range items {
			fmt.Fprintf(w, "  %-6s line %d — %s\n", f.Marker, f.Line, f.Text)
		}
	}
	fmt.Fprintln(w)
	return CheckSummary{
		Label: "INCOMPLETE",
		Line:  fmt.Sprintf("%d flagged across %d file(s)", len(res.Findings), len(fileOrder)),
	}
}

func WriteConfidence(w io.Writer, report confidence.Report) CheckSummary {
	fmt.Fprintln(w, "CONFIDENCE — does the agent's own claim match what was actually verified?")
	fmt.Fprintln(w, strings.Repeat("-", 78))
	if !report.Available {
		fmt.Fprintf(w, "SKIPPED: %s\n\n", report.Reason)
		return CheckSummary{Label: "CONFIDENCE", Line: "SKIPPED"}
	}

	var mismatches, confirmed, notClaimed []confidence.Result
	for _, r := range report.Results {
		switch r.Verdict {
		case confidence.Mismatch:
			mismatches = append(mismatches, r)
		case confidence.Confirmed:
			confirmed = append(confirmed, r)
		default:
			notClaimed = append(notClaimed, r)
		}
	}

	if len(mismatches) > 0 {
		fmt.Fprintf(w, "CONFIDENCE MISMATCH (%d) — claimed done, but not what got verified\n", len(mismatches))
		for _, r := range mismatches {
			fmt.Fprintf(w, "  %s\n", r.FeatureName)
			fmt.Fprintf(w, "    claimed: %s\n", r.ClaimLine)
			fmt.Fprintf(w, "    reason: %s\n", r.Reason)
		}
		fmt.Fprintln(w)
	}

	if len(confirmed) > 0 {
		fmt.Fprintf(w, "CONFIRMED (%d) — claim matches what was verified\n", len(confirmed))
		for _, r := range confirmed {
			fmt.Fprintf(w, "  %s — %s\n", r.FeatureName, r.ClaimLine)
		}
		fmt.Fprintln(w)
	}

	if len(notClaimed) > 0 {
		fmt.Fprintf(w, "NOT CLAIMED (%d) — nothing said about these, nothing to check\n", len(notClaimed))
		for _, r := range notClaimed {
			fmt.Fprintf(w, "  - %s\n", r.FeatureName)
		}
		fmt.Fprintln(w)
	}

	return CheckSummary{
		Label: "CONFIDENCE",
		Line:  fmt.Sprintf("%d CONFIRMED · %d MISMATCH · %d NOT CLAIMED", len(confirmed), len(mismatches), len(notClaimed)),
	}
}
