package features

import (
	"os"
	"path/filepath"
	"testing"
)

func load(t *testing.T, name, body string) []Feature {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

// Users of a roadmap-style plan ("what we will build") were told FAILED for every line not built yet.
// A line counts as claimed done only when it says so: [x], a done marker, or a done status cell.
// A plain bullet, a numbered line or a to-do row claims nothing.
func TestOnlyALineThatSaysDoneIsClaimedDone(t *testing.T) {
	fs := load(t, "PLAN.md", "# Plan\n- `GET /api/a` list\n- [x] `GET /api/b` list\n- [ ] `GET /api/c` list\n- **5.4 Events page** ✅\n1. `GET /api/d` list\n* `GET /api/e` list ✔️\n")
	want := map[string]bool{"/api/a": true, "/api/b": false, "/api/c": true, "5.4": false, "/api/d": true, "/api/e": false}
	got := map[string]bool{}
	for _, f := range fs {
		for k := range want {
			if contains(f.Name, k) {
				got[k] = f.Unticked
			}
		}
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: Unticked = %v, want %v", k, got[k], w)
		}
	}
}

func TestTableRowsAreClaimedOnlyWithADoneCell(t *testing.T) {
	fs := load(t, "PLAN.md", "| # | Task | Status |\n|---|---|---|\n| 1 | `GET /api/a` list | Done |\n| 2 | `GET /api/b` list | |\n| 3 | `GET /api/c` list | Todo |\n")
	got := map[string]bool{}
	for _, f := range fs {
		for _, k := range []string{"/api/a", "/api/b", "/api/c"} {
			if contains(f.Name, k) {
				got[k] = f.Unticked
			}
		}
	}
	if got["/api/a"] || !got["/api/b"] || !got["/api/c"] {
		t.Errorf("table rows: %v", got)
	}
}

// JSON and text plans are lists of things to build now; they keep blocking.
func TestJSONPlansStayDueNow(t *testing.T) {
	fs := load(t, "features.json", `[{"name":"refund button"}]`)
	if len(fs) != 1 || fs[0].Unticked {
		t.Fatalf("%+v", fs)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
