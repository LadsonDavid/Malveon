// Package planauthority implements the not-in-plan flag from CLAUDE.md
// section 3.2.3: does the code touched this session do something the
// plan never asked for. Requires a session start (see internal/session)
// — without one there's no meaningful "this session's diff" to check
// against, so this check reports itself unavailable rather than
// silently scanning the whole repo (which would flag every pre-existing
// route in the codebase, not just what changed).
package planauthority

import (
	"fmt"
	"path/filepath"

	"github.com/LadsonDavid/beta-test/internal/features"
	"github.com/LadsonDavid/beta-test/internal/gitutil"
	"github.com/LadsonDavid/beta-test/internal/graph"
	"github.com/LadsonDavid/beta-test/internal/session"
)

type Finding struct {
	File   string
	Line   int
	Kind   string // "route" or "call"
	Path   string
	Method string
}

type Result struct {
	Available bool
	Reason    string // why unavailable, only set when Available is false
	Findings  []Finding
}

func Run(g *graph.Graph, fs []features.Feature, root string) Result {
	st, ok := session.Load(root)
	if !ok {
		return Result{Available: false, Reason: "no session start recorded — run `malveon session start` before the agent begins its task"}
	}

	changed, err := gitutil.ChangedFilesSince(root, st.StartRef)
	if err != nil {
		return Result{Available: false, Reason: fmt.Sprintf("couldn't compute changed files: %v", err)}
	}

	var findings []Finding
	for _, n := range g.Nodes {
		if !changed[filepath.ToSlash(n.File)] {
			continue
		}
		if n.Confidence != graph.Extracted {
			continue // can't fairly judge an unresolvable path against the plan
		}
		if matchesAnyFeature(n, fs) {
			continue
		}
		findings = append(findings, Finding{
			File:   n.File,
			Line:   n.Line,
			Kind:   kindLabel(n.Kind),
			Path:   n.Path,
			Method: n.Method,
		})
	}

	return Result{Available: true, Findings: findings}
}

func matchesAnyFeature(n graph.Node, fs []features.Feature) bool {
	nodeWords := make(map[string]bool, len(n.Words))
	for _, w := range n.Words {
		nodeWords[w] = true
	}
	for _, f := range fs {
		for _, w := range f.Words() {
			if nodeWords[w] {
				return true
			}
		}
	}
	return false
}

func kindLabel(k graph.Kind) string {
	if k == graph.RouteHandler {
		return "route"
	}
	return "call"
}
