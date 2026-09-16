// Package uioverlap implements the frontend visual-overlap risk check —
// distinct from internal/checks/overlap, which is about backend route
// collisions. This one is about CSS positioning, and it is deliberately
// NOT a claim that two elements actually overlap on screen: that fact
// depends on real rendered geometry (content size, viewport), which
// cannot be known without a browser. See CLAUDE.md for the full
// reasoning behind why this stays structural-risk-only in v1.
//
// v1 scope, "Tailwind first" (confirmed 2026-09-16): scan JSX/TSX/HTML
// for elements using Tailwind's `absolute`/`fixed` utility classes, and
// flag a file where NONE of its elements anywhere establish a
// positioning context (`relative`/`absolute`/`fixed`/`sticky`). An
// absolutely/fixed-positioned element with no positioning context
// anywhere in the same file will position against the page itself
// instead of its intended container — a well-known, common real bug,
// not a guess.
//
// Stated limitation: file-scoped, not ancestor-precise. This does not
// trace the actual JSX parent chain (that needs real tag-tree parsing,
// a larger future addition) — it only knows whether *any* positioning
// context exists anywhere in the file. A false negative is possible if
// the file has an unrelated `relative` element that isn't actually this
// element's ancestor; that's a known trade-off for staying simple and
// correct about what it does check, not a hidden gap.
// Plain-CSS-file cascade resolution (the general case beyond Tailwind
// utility classes) is explicitly deferred, not built.
package uioverlap

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Finding struct {
	File    string
	Line    int
	Classes string
	Reason  string
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true, ".malveon": true,
}

var classAttrPattern = regexp.MustCompile(`(?:class|className)\s*=\s*"([^"]*)"`)

// positioningContextClasses are classes that count as a real, deliberate
// containment anchor. Deliberately excludes "absolute"/"fixed" — an
// escaping element sitting near another escaping element isn't a fix,
// it's the same problem twice.
var positioningContextClasses = map[string]bool{
	"relative": true, "sticky": true,
}
var escapingClasses = map[string]bool{
	"absolute": true, "fixed": true,
}

// Run scans every JSX/TSX/HTML file under root and returns one finding
// per file that has an absolute/fixed element but no positioning context
// anywhere in the same file.
func Run(root string) ([]Finding, error) {
	var findings []Finding

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jsx", ".tsx", ".html", ".vue":
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

		findings = append(findings, scanFile(rel, src)...)
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
			Reason:  "positioned element (absolute/fixed) with no positioning context (relative/absolute/fixed/sticky) found anywhere in this file — likely escapes its intended container",
		})
	}
	return findings
}

func lineOf(src string, byteOffset int) int {
	return strings.Count(src[:byteOffset], "\n") + 1
}
