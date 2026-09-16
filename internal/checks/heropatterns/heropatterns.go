// Package heropatterns implements automatic, zero-self-report hero-act
// detection: does this session's own code contain a known, named bug
// pattern that wasn't there before the session started. Two independent
// signals answer that, at different cost and different availability —
// same underlying concern, not two different checks (see CLAUDE.md
// 3.2.4 and the /software evolutionary-architectures routing that
// confirmed this shape: both protect "no known bad pattern introduced
// this session," just via a cheap always-on comparison and a richer
// optional one):
//
//   - Git baseline (always available once `malveon session start` ran):
//     compares the file's content at the session-start commit against
//     the current on-disk version. Catches a pattern that's present
//     right now and wasn't there at session start — a live, unfixed bug
//     the agent introduced. Needs no watcher at all.
//   - Watch history (only available if `malveon watch` was running):
//     compares intermediate snapshots against the current version.
//     Catches a pattern that appeared AND disappeared entirely within
//     the session — invisible to a single before/after diff, which is
//     exactly why `malveon watch` exists (see internal/watch).
//
// Deliberately narrow: a small, named catalog of well-known, low-noise
// bug signatures, not a general "was this a real bug" judgment (which
// is not resolvable from static snapshots alone — see CLAUDE.md's note
// on why the fully general version needs live test execution instead).
package heropatterns

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/beta-test/internal/gitutil"
	"github.com/LadsonDavid/beta-test/internal/session"
	"github.com/LadsonDavid/beta-test/internal/watch"
)

type patternDef struct {
	name string
	exts map[string]bool
	re   *regexp.Regexp
}

var catalog = []patternDef{
	{
		name: "assignment-in-condition",
		exts: map[string]bool{".js": true, ".jsx": true, ".ts": true, ".tsx": true},
		// if (x = 5) — a single '=' inside an if-condition, not ==/===/!=/<=/>=.
		re: regexp.MustCompile(`if\s*\([^()=!<>]*[^=!<>]=[^=][^()]*\)`),
	},
	{
		name: "empty-error-handling",
		exts: map[string]bool{".go": true},
		// if err != nil {} — the error is checked but silently swallowed.
		re: regexp.MustCompile(`if\s+\w*[Ee]rr\w*\s*!=\s*nil\s*\{\s*\}`),
	},
	{
		name: "bare-except",
		exts: map[string]bool{".py": true},
		// except: with no exception type — catches everything, including
		// KeyboardInterrupt/SystemExit.
		re: regexp.MustCompile(`(?m)^\s*except\s*:`),
	},
}

type Status string

const (
	StillPresent     Status = "INTRODUCED THIS SESSION — STILL PRESENT"
	FixedSameSession Status = "INTRODUCED & FIXED SAME SESSION"
)

type Finding struct {
	File    string
	Pattern string
	Status  Status
	Reason  string
}

type Report struct {
	Available bool
	Reason    string
	Findings  []Finding

	// WatchAvailable/WatchReason describe the richer, optional signal
	// specifically (see package doc) — Available can be true from the
	// git baseline alone while WatchAvailable is false, meaning
	// "introduced and fixed within the session" detection didn't run
	// even though "introduced and still present" did.
	WatchAvailable bool
	WatchReason    string
}

// Run checks both available signals (see package doc) and merges their
// findings. It's available if at least one signal has something to work
// with — a session start for the git baseline, or usable `malveon watch`
// history for the fixed-in-session signal.
func Run(root string) Report {
	stillPresent, gitOK := stillPresentFindings(root)
	fixed, watchOK, watchReason := fixedSameSessionFindings(root)

	if !gitOK && !watchOK {
		return Report{Available: false, Reason: "no session started and " + watchReason + " — run `malveon session start` (and optionally `malveon watch`) before the agent begins its task"}
	}

	findings := append(stillPresent, fixed...)
	return Report{Available: true, Findings: findings, WatchAvailable: watchOK, WatchReason: watchReason}
}

// stillPresentFindings compares each session-changed file's content at
// session start (via git) against its current version — the cheap,
// always-available half. ok is false only if no session was started at
// all, never on a per-file resolution failure.
func stillPresentFindings(root string) (findings []Finding, ok bool) {
	st, started := session.Load(root)
	if !started {
		return nil, false
	}
	changed, err := gitutil.ChangedFilesSince(root, st.StartRef)
	if err != nil {
		return nil, true
	}

	for file := range changed {
		applicable := patternsFor(file)
		if len(applicable) == 0 {
			continue
		}
		currentRaw, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if readErr != nil {
			continue // deleted or moved since - nothing to check
		}
		current := string(currentRaw)

		baseline, existed, err := gitutil.FileAtRef(root, st.StartRef, file)
		if err != nil {
			continue
		}

		for _, p := range applicable {
			if !p.re.MatchString(current) {
				continue // not present now - nothing to flag
			}
			if existed && p.re.MatchString(baseline) {
				continue // already there before the session - pre-existing, not introduced
			}
			findings = append(findings, Finding{
				File: file, Pattern: p.name, Status: StillPresent,
				Reason: fmt.Sprintf("%s is present now and wasn't in the file's state at session start — introduced this session, still here, found from git alone (no watcher needed)", p.name),
			})
		}
	}
	return findings, true
}

// fixedSameSessionFindings is the original watch-history-based signal:
// a pattern present in an earlier captured snapshot and absent from the
// current version. ok is false if `malveon watch` was never run or its
// history isn't trustworthy (stale heartbeat); reason explains why.
func fixedSameSessionFindings(root string) (findings []Finding, ok bool, reason string) {
	st, statusOK := watch.LoadStatus(root)
	if !statusOK {
		return nil, false, "`malveon watch` was never run for this session"
	}
	usable, why := watch.Usable(st)
	if !usable {
		return nil, false, why
	}

	history, err := watch.LoadHistory(root)
	if err != nil {
		return nil, false, err.Error()
	}
	if len(history) == 0 {
		return nil, true, "" // watcher ran, nothing changed yet - a valid empty state
	}

	fileGens := map[string][]watch.Generation{}
	for _, gen := range history {
		for _, f := range gen.Files {
			fileGens[f] = append(fileGens[f], gen)
		}
	}

	for file, gens := range fileGens {
		applicable := patternsFor(file)
		if len(applicable) == 0 {
			continue
		}
		currentRaw, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if readErr != nil {
			continue // file was deleted or moved since — nothing to compare against
		}
		current := string(currentRaw)

		for _, p := range applicable {
			if p.re.MatchString(current) {
				continue // pattern is still present — that's stillPresentFindings' job, not this one
			}
			if gen, found := earliestWithPattern(root, gens, file, p); found {
				findings = append(findings, Finding{
					File: file, Pattern: p.name, Status: FixedSameSession,
					Reason: fmt.Sprintf("%s was present in a captured snapshot from this session (gen %d) and is gone from the current version — introduced and fixed within this session, found automatically from captured history, no self-report involved", p.name, gen.Seq),
				})
			}
		}
	}
	return findings, true, ""
}

func patternsFor(file string) []patternDef {
	ext := strings.ToLower(filepath.Ext(file))
	var out []patternDef
	for _, p := range catalog {
		if p.exts[ext] {
			out = append(out, p)
		}
	}
	return out
}

func earliestWithPattern(root string, gens []watch.Generation, file string, p patternDef) (watch.Generation, bool) {
	for _, gen := range gens {
		content, ok := watch.SnapshotContent(root, gen, file)
		if ok && p.re.MatchString(content) {
			return gen, true
		}
	}
	return watch.Generation{}, false
}
