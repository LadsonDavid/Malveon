// Package incompleteness implements the one genuinely code-only signal
// for "does this code admit it's unfinished": TODO/FIXME/HACK/XXX/"not
// implemented" markers left in files changed this session. Confirmed via
// research (real tools like SonarQube already treat these as a
// legitimate static quality signal) that this is the correct scope —
// there is no general way to detect "complete"/"confident-worthy" from
// code structure alone, only specific, named markers like these.
//
// Deliberately one-directional: a marker's presence is real, structural
// proof the code documents its own gap. A marker's absence proves
// nothing — most finished code has none either — so this can never
// stand in for confidence-check (internal/checks/confidence), which
// tests an actual claim against reality. This is a different, narrower
// question: not "did the agent claim it works," but "does the code
// itself admit it doesn't."
package incompleteness

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/beta-test/internal/gitutil"
	"github.com/LadsonDavid/beta-test/internal/session"
)

type Finding struct {
	File   string
	Line   int
	Marker string
	Text   string
}

type Report struct {
	Available bool
	Reason    string
	Findings  []Finding
}

var sourceExts = map[string]bool{
	".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".py": true, ".go": true,
}

var markerPattern = regexp.MustCompile(`(?i)\b(TODO|FIXME|HACK|XXX)\b|not\s+implemented`)

// Run scans every source file changed since session start for
// incompleteness markers. Requires a session to have been started, so
// it stays scoped to what this session actually touched instead of
// flagging pre-existing TODOs unrelated to the current task.
func Run(root string) Report {
	st, ok := session.Load(root)
	if !ok {
		return Report{Available: false, Reason: "no session start recorded — run `malveon session start` before the agent begins its task"}
	}
	changed, err := gitutil.ChangedFilesSince(root, st.StartRef)
	if err != nil {
		return Report{Available: false, Reason: err.Error()}
	}

	var findings []Finding
	for rel := range changed {
		if !sourceExts[strings.ToLower(filepath.Ext(rel))] {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if readErr != nil {
			continue // deleted or moved since — nothing to scan
		}
		for i, line := range strings.Split(string(raw), "\n") {
			m := markerPattern.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			marker := strings.ToUpper(m[1])
			if marker == "" {
				marker = "NOT IMPLEMENTED"
			}
			findings = append(findings, Finding{
				File: rel, Line: i + 1, Marker: marker, Text: strings.TrimSpace(line),
			})
		}
	}

	return Report{Available: true, Findings: findings}
}
