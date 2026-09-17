// Package uioverlap implements the frontend visual-overlap risk check —
// distinct from internal/checks/overlap, which is about backend route
// collisions. This one is about CSS positioning, and it is deliberately
// NOT a claim that two elements actually overlap on screen: that fact
// depends on real rendered geometry (content size, viewport), which
// cannot be known without a browser. See CLAUDE.md for the full
// reasoning behind why this stays structural-risk-only in v1.
//
// v1 scope, "Tailwind first" (confirmed 2026-09-16): scan JSX/TSX/HTML
// for elements using Tailwind's `absolute` utility class, and flag a
// file where NONE of its elements anywhere establish a positioning
// context (`relative`/`sticky`/`fixed`). An absolutely-positioned
// element with no positioning context anywhere in the same file will
// position against the page itself instead of its intended container —
// a well-known, common real bug, not a guess.
//
// `fixed` is deliberately NOT something this check flags (fixed 2026-09-17,
// a real false positive found by reading a real project's Modal.tsx and
// admin/layout.tsx): a `fixed` element always positions against the
// viewport — it never needs an ancestor to anchor to, the same way
// `absolute` does. It IS still counted as valid positioning context
// *for a nested absolute descendant*, since CSS gives any non-static
// positioned element (relative/sticky/fixed, same as absolute) its own
// containing block — a `<div className="fixed inset-0">` wrapping
// `<div className="absolute inset-0">` is a standard, correct modal
// pattern, not a bug.
//
// Stated limitation: file-scoped, not ancestor-precise. This does not
// trace the actual JSX parent chain (that needs real tag-tree parsing,
// a larger future addition) — it only knows whether *any* positioning
// context exists anywhere in the file. A false negative is possible if
// the file has an unrelated `relative` element that isn't actually this
// element's ancestor; that's a known trade-off for staying simple and
// correct about what it does check, not a hidden gap.
// Plain-CSS files (.css/.scss/.less) get the same file-scoped heuristic,
// applied to the actual `position:` declaration instead of a utility
// class name: a file with position: absolute anywhere and no
// position: relative/sticky/fixed anywhere is flagged. Context detection
// deliberately scans the whole file as flat text rather than trying to
// pair each declaration with its enclosing selector via brace-matching —
// real CSS/SCSS nesting (a parent selector's own `relative` inside a
// block that also contains a nested `absolute` child) would make a
// brace-matched scan miss the parent's declaration and produce a false
// positive; a flat "does this value appear anywhere in the file" search
// can't undercount a real context declaration that way. The cost is the
// same one already accepted for Tailwind: file-scoped, not ancestor-
// precise, real future work if that ever needs tightening.
package uioverlap

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/beta-test/internal/skipdirs"
)

type Finding struct {
	File    string
	Line    int
	Classes string
	Reason  string
}

var classAttrPattern = regexp.MustCompile(`(?:class|className)\s*=\s*"([^"]*)"`)

// positioningContextClasses are classes that establish a valid CSS
// containing block for a nested absolutely-positioned descendant.
// Includes "fixed": per the CSS spec, any non-static position
// (relative/sticky/fixed, same as absolute) gives an element its own
// containing block — a "fixed inset-0" wrapper around an "absolute
// inset-0" child is a standard, correct pattern, not a bug. Deliberately
// excludes "absolute" itself: this check is file-scoped, not
// ancestor-precise (see package doc comment), so counting absolute as
// its own context would let one absolute element anywhere in the file
// silence every other absolute element's real missing-context bug.
var positioningContextClasses = map[string]bool{
	"relative": true, "sticky": true, "fixed": true,
}

// escapingClasses are classes that need a positioning-context ancestor
// to avoid escaping their intended container. Deliberately excludes
// "fixed": a fixed element always positions against the viewport, so it
// never needs one (the transform/filter/perspective containing-block
// edge case is out of scope for this file-scoped heuristic — confirmed
// as a real false positive 2026-09-17 by reading a real project's
// Modal.tsx and admin/layout.tsx, both flagging standard "fixed inset-0"
// patterns that were never actually broken).
var escapingClasses = map[string]bool{
	"absolute": true,
}

var cssPositionDeclPattern = regexp.MustCompile(`(?i)position\s*:\s*(absolute|fixed|relative|sticky)\b`)

// Run scans every JSX/TSX/HTML file under root and returns one finding
// per file that has an absolute element but no positioning context
// anywhere in the same file.
func Run(root string) ([]Finding, error) {
	var findings []Finding

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipdirs.Names[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		var scan func(file, src string) []Finding
		switch ext {
		case ".jsx", ".tsx", ".html", ".vue":
			scan = scanFile
		case ".css", ".scss", ".less":
			scan = scanCSSFile
		default:
			return nil
		}

		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		src := string(raw)
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}

		findings = append(findings, scan(rel, src)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return findings, nil
}

func scanFile(file, src string) []Finding {
	matches := classAttrPattern.FindAllStringSubmatchIndex(src, -1)
	if len(matches) == 0 {
		return nil
	}

	hasContext := false
	type escapee struct {
		line, start, end int
		classes          string
	}
	var candidates []escapee

	for _, m := range matches {
		classAttr := src[m[2]:m[3]]
		classes := strings.Fields(classAttr)
		isContext, isEscaping := false, false
		for _, c := range classes {
			if positioningContextClasses[c] {
				isContext = true
			}
			if escapingClasses[c] {
				isEscaping = true
			}
		}
		if isContext {
			hasContext = true
		}
		if isEscaping {
			candidates = append(candidates, escapee{line: lineOf(src, m[0]), classes: classAttr})
		}
	}

	if hasContext || len(candidates) == 0 {
		return nil
	}

	findings := make([]Finding, 0, len(candidates))
	for _, c := range candidates {
		findings = append(findings, Finding{
			File:    file,
			Line:    c.line,
			Classes: c.classes,
			Reason:  "absolutely-positioned element with no positioning context (relative/sticky/fixed) found anywhere in this file — likely escapes its intended container",
		})
	}
	return findings
}

// scanCSSFile flags every position: absolute declaration in a plain
// CSS/SCSS/LESS file that has no position: relative/sticky/fixed
// declaration anywhere in the same file. position: fixed is never
// flagged (always resolves against the viewport) but does count as
// valid context for a nested absolute — see the package doc comment for
// both that reasoning and why context detection deliberately doesn't
// try to pair a declaration with its enclosing selector.
func scanCSSFile(file, src string) []Finding {
	matches := cssPositionDeclPattern.FindAllStringSubmatchIndex(src, -1)
	if len(matches) == 0 {
		return nil
	}

	hasContext := false
	var candidateOffsets []int
	for _, m := range matches {
		value := strings.ToLower(src[m[2]:m[3]])
		if value == "relative" || value == "sticky" || value == "fixed" {
			hasContext = true
			continue
		}
		candidateOffsets = append(candidateOffsets, m[0])
	}

	if hasContext || len(candidateOffsets) == 0 {
		return nil
	}

	findings := make([]Finding, 0, len(candidateOffsets))
	for _, offset := range candidateOffsets {
		findings = append(findings, Finding{
			File:    file,
			Line:    lineOf(src, offset),
			Classes: "position: absolute",
			Reason:  "plain CSS declares position: absolute with no position: relative/sticky/fixed found anywhere in this file — likely escapes its intended container",
		})
	}
	return findings
}

func lineOf(src string, byteOffset int) int {
	return strings.Count(src[:byteOffset], "\n") + 1
}
