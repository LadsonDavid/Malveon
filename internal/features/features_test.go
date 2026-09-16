package features

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadJSON(t *testing.T) {
	p := writeTemp(t, "features.json", `[{"id":"refund-button","name":"refund button"}]`)
	fs, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(fs) != 1 || fs[0].ID != "refund-button" || fs[0].Name != "refund button" {
		t.Fatalf("unexpected result: %+v", fs)
	}
}

func TestLoadMarkdownChecklist(t *testing.T) {
	p := writeTemp(t, "plan.md", `# Sprint plan

- [ ] Refund button
- [x] Cancel order
* Profile update
1. Update settings

Some prose that isn't a checklist item.
`)
	fs, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	wantNames := []string{"Refund button", "Cancel order", "Profile update", "Update settings"}
	if len(fs) != len(wantNames) {
		t.Fatalf("got %d features, want %d: %+v", len(fs), len(wantNames), fs)
	}
	for i, name := range wantNames {
		if fs[i].Name != name {
			t.Errorf("feature %d: got name %q, want %q", i, fs[i].Name, name)
		}
		if fs[i].ID == "" {
			t.Errorf("feature %d: expected a generated ID, got empty", i)
		}
	}
}

func TestLoadPlainText(t *testing.T) {
	p := writeTemp(t, "plan.txt", "refund button\n# a comment, skipped\n\ncancel order\n")
	fs, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(fs) != 2 {
		t.Fatalf("got %d features, want 2: %+v", len(fs), fs)
	}
	if fs[0].Name != "refund button" || fs[1].Name != "cancel order" {
		t.Errorf("unexpected names: %+v", fs)
	}
}

func TestDetect(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"PLAN.md", "features.json", "README.md", "notes.txt", "server.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	candidates, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	want := map[string]bool{
		filepath.Join(dir, "PLAN.md"):        true,
		filepath.Join(dir, "features.json"):  true,
	}
	if len(candidates) != len(want) {
		t.Fatalf("got %d candidates, want %d: %v", len(candidates), len(want), candidates)
	}
	for _, c := range candidates {
		if !want[c] {
			t.Errorf("unexpected candidate: %s (README.md, notes.txt, server.go should never match)", c)
		}
	}
}

func TestDetectNoMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "server.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidates, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates, got %v", candidates)
	}
}

func writeIn(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectByContentFindsUnnamedFiles(t *testing.T) {
	dir := t.TempDir()

	// No name hint at all, but the content genuinely looks like a plan.
	writeIn(t, dir, "sprint3.json", `[{"id":"a","name":"refund button"},{"id":"b","name":"cancel order"}]`)
	writeIn(t, dir, "notes.md", "# Notes\n\n- [ ] Refund button\n- [x] Cancel order\n")

	// Should NOT be picked up: valid JSON, but not shaped like our
	// feature list (no "name" field), and a markdown file with fewer
	// than 2 real checklist lines.
	writeIn(t, dir, "config.json", `{"port": 8080, "debug": true}`)
	writeIn(t, dir, "CHANGELOG.md", "# Changelog\n\n## v1.0\n\nInitial release.\n")

	candidates, err := DetectByContent(dir)
	if err != nil {
		t.Fatalf("DetectByContent: %v", err)
	}

	want := map[string]bool{
		filepath.Join(dir, "sprint3.json"): true,
		filepath.Join(dir, "notes.md"):     true,
	}
	if len(candidates) != len(want) {
		t.Fatalf("got %d candidates, want %d: %v", len(candidates), len(want), candidates)
	}
	for _, c := range candidates {
		if !want[c] {
			t.Errorf("unexpected candidate: %s (config.json, CHANGELOG.md should never match)", c)
		}
	}
}

func TestLoadDuplicateNamesGetDistinctIDs(t *testing.T) {
	p := writeTemp(t, "plan.txt", "Refund Button\nrefund button\n")
	fs, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(fs) != 2 {
		t.Fatalf("got %d features, want 2: %+v", len(fs), fs)
	}
	if fs[0].ID == fs[1].ID {
		t.Errorf("expected distinct IDs for two features that slug the same, got %q twice", fs[0].ID)
	}
}

// TestDetectRecursesIntoSubdirectories covers the real bug a reviewer
// hit: a plan file kept in docs/PLAN.md (a completely normal place to
// keep one) was invisible to the old top-level-only scan, so a much
// weaker content-based match at the root won by default instead.
func TestDetectRecursesIntoSubdirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeIn(t, dir, filepath.Join("docs", "PLAN.md"), "- [ ] Refund button\n- [x] Cancel order\n")
	writeIn(t, dir, "README.md", "# Getting started\n\n- [ ] Clone the repo\n- [ ] Run npm install\n")

	candidates, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	want := filepath.Join(dir, "docs", "PLAN.md")
	if len(candidates) != 1 || candidates[0] != want {
		t.Fatalf("got %v, want exactly [%s] — name-based detection should find docs/PLAN.md and never match README.md by name", candidates, want)
	}
}

// TestDetectSkipsNoiseDirectories proves recursion doesn't wander into
// node_modules/.git/.next and accidentally treat something in there as
// a plan candidate.
func TestDetectSkipsNoiseDirectories(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"node_modules", ".git", ".next"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
		writeIn(t, dir, filepath.Join(sub, "PLAN.md"), "- [ ] should never be found\n")
	}

	candidates, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates (all matches sit inside skipped dirs), got %v", candidates)
	}
}
