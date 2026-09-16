// Package heropatterns implements automatic, zero-self-report hero-act
// detection: using the snapshot history captured by internal/watch
// (never git commits, never anything the agent said), it checks whether
// an earlier snapshot of a changed file contained a known, named bug
// pattern that the current version no longer has. That's real,
// structural evidence the pattern was introduced and fixed within this
// same session — no commit message trusted, no self-report needed.
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

type Finding struct {
	File    string
	Pattern string
	Reason  string
}

type Report struct {
	Available bool
	Reason    string
	Findings  []Finding
}

// Run checks captured watch history (see internal/watch) for files
// where a known bad pattern appeared in an earlier snapshot and is
// absent from the current, on-disk version.
func Run(root string) Report {
	st, ok := watch.LoadStatus(root)
	if !ok {
		return Report{Available: false, Reason: "`malveon watch` was never run for this session — no captured history to check known bug patterns against"}
	}
	usable, reason := watch.Usable(st)
	if !usable {
		return Report{Available: false, Reason: reason}
	}

	history, err := watch.LoadHistory(root)
	if err != nil {
		return Report{Available: false, Reason: err.Error()}
	}
	if len(history) == 0 {
		return Report{Available: true} // watcher ran, nothing changed yet — a valid empty state
	}

	fileGens := map[string][]watch.Generation{}
	for _, gen := range history {
		for _, f := range gen.Files {
			fileGens[f] = append(fileGens[f], gen)
		}
	}

	var findings []Finding
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
				continue // pattern is still present — not fixed, nothing to report
			}
			if gen, found := earliestWithPattern(root, gens, file, p); found {
				findings = append(findings, Finding{
					File: file, Pattern: p.name,
					Reason: fmt.Sprintf("%s was present in a captured snapshot from this session (gen %d) and is gone from the current version — introduced and fixed within this session, found automatically from captured history, no self-report involved", p.name, gen.Seq),
				})
			}
		}
	}

	return Report{Available: true, Findings: findings}
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
