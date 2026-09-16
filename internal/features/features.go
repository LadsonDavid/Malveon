// Package features loads whatever plan/checklist file the user already
// has, in whatever common format they already keep it in. The external
// contract never changes regardless of format: Load takes a path, returns
// a flat []Feature. Every format-specific parsing detail (JSON shape,
// checklist syntax, line splitting) is hidden inside this package — no
// caller anywhere in the tool knows or cares how many formats exist.
package features

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Feature struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Words returns the feature's name split into lowercase tokens for loose
// matching against graph node words.
func (f Feature) Words() []string {
	return strings.Fields(strings.ToLower(strings.NewReplacer("-", " ", "_", " ").Replace(f.Name)))
}

// Load reads path and returns its features, dispatching on file extension:
// .json is the original {id,name} array; .md/.markdown reads a checklist
// (- [ ] item / - item / 1. item); anything else (.txt or no extension)
// is treated as one feature name per line. IDs are generated from the
// name when a format doesn't supply one, de-duplicated if two lines slug
// to the same value.
func Load(path string) ([]Feature, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading features file: %w", err)
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return parseJSON(raw)
	case ".md", ".markdown":
		return withGeneratedIDs(parseChecklistLines(string(raw))), nil
	default:
		return withGeneratedIDs(parsePlainTextLines(string(raw))), nil
	}
}

func parseJSON(raw []byte) ([]Feature, error) {
	var fs []Feature
	if err := json.Unmarshal(raw, &fs); err != nil {
		return nil, fmt.Errorf("parsing features file: %w", err)
	}
	return fs, nil
}

var checklistItemPattern = regexp.MustCompile(`^\s*(?:[-*]\s*\[[ xX]\]|[-*]|\d+[.)])\s+(.+)$`)

// parseChecklistLines extracts item text from Markdown checklist/list
// syntax: "- [ ] item", "- [x] item", "- item", "* item", "1. item".
// Lines that don't match any of those forms (headings, prose, blank
// lines) are skipped rather than guessed at.
func parseChecklistLines(text string) []string {
	var names []string
	for _, line := range strings.Split(text, "\n") {
		if m := checklistItemPattern.FindStringSubmatch(line); m != nil {
			name := strings.TrimSpace(m[1])
			if name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// parsePlainTextLines treats every non-empty, non-comment line as one
// feature name — the fallback for .txt or any unrecognized extension.
func parsePlainTextLines(text string) []string {
	var names []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		names = append(names, line)
	}
	return names
}

func withGeneratedIDs(names []string) []Feature {
	seen := map[string]int{}
	fs := make([]Feature, 0, len(names))
	for _, name := range names {
		id := slugify(name)
		seen[id]++
		if n := seen[id]; n > 1 {
			id = fmt.Sprintf("%s-%d", id, n)
		}
		fs = append(fs, Feature{ID: id, Name: name})
	}
	return fs
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := slugNonAlnum.ReplaceAllString(strings.ToLower(name), "-")
	return strings.Trim(s, "-")
}
