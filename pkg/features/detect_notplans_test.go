package features

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const checklist = "# Things\n- [ ] one thing\n- [ ] another thing\n"

// A project's own housekeeping files are never "the plan". Run on 15 real projects, `malveon check`
// stopped at "found more than one possible plan file (README.md, CHANGELOG.md, CODE_OF_CONDUCT.md,
// .github/ISSUE_TEMPLATE/feature_request.md, test fixtures)" and checked nothing.
func TestDetectIgnoresFilesThatAreNotPlans(t *testing.T) {
	root := writeTree(t, map[string]string{
		"README.md":          checklist,
		"CHANGELOG.md":       checklist,
		"CODE_OF_CONDUCT.md": checklist,
		"CONTRIBUTING.md":    checklist,
		".github/ISSUE_TEMPLATE/feature_request.md": checklist,
		".github/PULL_REQUEST_TEMPLATE.md":          checklist,
		"e2e/fixtures/plan.md":                      checklist,
		"testdata/features.json":                    `[{"name":"x"}]`,
		"src/__fixtures__/todo.md":                  checklist,
		"docs/PLAN.md":                              checklist,
	})
	byName, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	var rel []string
	for _, p := range byName {
		r, _ := filepath.Rel(root, p)
		rel = append(rel, filepath.ToSlash(r))
	}
	if want := []string{"docs/PLAN.md"}; !reflect.DeepEqual(rel, want) {
		t.Errorf("Detect = %v, want only the real plan %v", rel, want)
	}
}

func TestDetectByContentIgnoresFilesThatAreNotPlans(t *testing.T) {
	root := writeTree(t, map[string]string{
		"README.md":                        checklist,
		"CHANGELOG.md":                     checklist,
		".github/PULL_REQUEST_TEMPLATE.md": checklist,
		"e2e/fixtures/anything.md":         checklist,
	})
	got, err := DetectByContent(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("DetectByContent = %v, want nothing: none of these is a plan", got)
	}
	// ...but a checklist the user named something else still counts.
	root2 := writeTree(t, map[string]string{"README.md": checklist, "launch-tasks.md": checklist})
	got2, _ := DetectByContent(root2)
	if len(got2) != 1 || filepath.Base(got2[0]) != "launch-tasks.md" {
		t.Errorf("DetectByContent = %v, want only launch-tasks.md", got2)
	}
}
