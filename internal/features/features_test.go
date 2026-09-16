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
