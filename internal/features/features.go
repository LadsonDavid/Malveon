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
	"sort"
	"strings"

	"github.com/LadsonDavid/beta-test/internal/skipdirs"
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

var (
	candidateNamePattern = regexp.MustCompile(`(?i)(plan|feature|checklist|todo)`)
	candidateExts        = map[string]bool{".json": true, ".md": true, ".markdown": true, ".txt": true}
)

// Detect recurses through root (deliberately skipping skipdirs.Names, so it
// never wanders into node_modules or a build output dir and calls
// something a plan file by accident) for files that look like a plan:
// name containing "plan"/"feature"/"checklist"/"todo" with a supported
// extension. Returns sorted candidate paths, never a guess about which
// one is "the" plan — that decision is always left to the caller.
//
// Was top-level-only until 2026-09-16: a real test run against a project
// keeping its plan in docs/PLAN.md found nothing here, fell through to
// DetectByContent, and that fallback's much weaker signal (a README.md
// that happened to have checklist-looking lines) won by default — the
// exact kind of confidently-wrong result this whole tool exists to
// catch, now happening to the tool's own plan detection. Recursing here
// is the real fix: a deliberately-named file should be found wherever it
// actually lives, not just at the root.
func Detect(root string) ([]string, error) {
	var candidates []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skipdirs.Names[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !candidateExts[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		if candidateNamePattern.MatchString(name) {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("looking for a plan file in %s: %w", root, err)
	}
	sort.Strings(candidates)
	return candidates, nil
}

// DetectByContent is the fallback when Detect's name match finds nothing
// — the plan could be named anything. Recurses the same way Detect does
// (see skipdirs.Names), scanning every .json/.md/.markdown file (not .txt — a
// bare text file with no name hint has no reliable content signal
// either; asking the user is the honest move there, not guessing from "a
// file with some lines in it") and checking whether its *content*
// actually looks like a plan, not just its extension:
//   - .json: parses as an array of objects, each with a non-empty
//     "name" field — the real shape this tool expects, not just "is
//     it valid JSON."
//   - .md/.markdown: contains at least 2 real checklist-syntax lines
//     (see checklistItemPattern) — a strong, specific signal, not just
//     "the file has some bullet points somewhere."
//
// Same rule as everywhere else: this only ever proposes candidates for
// the caller to confirm or choose between, never picks one silently —
// callers must treat every result from this function (unlike Detect's)
// as needing confirmation even when there's only one, since a content
// match is a guess, not a deliberate name.
func DetectByContent(root string) ([]string, error) {
	var candidates []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skipdirs.Names[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".json" && ext != ".md" && ext != ".markdown" {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if contentLooksLikePlan(ext, raw) {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("looking for a plan file in %s: %w", root, err)
	}
	sort.Strings(candidates)
	return candidates, nil
}

func contentLooksLikePlan(ext string, raw []byte) bool {
	if ext == ".json" {
		var items []map[string]any
		if json.Unmarshal(raw, &items) != nil || len(items) == 0 {
			return false
		}
		for _, item := range items {
			name, ok := item["name"].(string)
			if !ok || strings.TrimSpace(name) == "" {
				return false
			}
		}
		return true
	}
	// .md / .markdown
	return len(parseChecklistLines(string(raw))) >= 2
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
