package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

func TestIsTestFile(t *testing.T) {
	yes := []string{
		"src/post.test.ts", "src/post.spec.tsx", "a/b/app.test.js", "web/Button.cy.ts",
		"src/__tests__/post.ts", "cypress/e2e/login.ts", "tests/e2e/flow.ts", "e2e/flow.ts",
		"server/api_test.go", "app/test_views.py", "app/views_test.py", "app/conftest.py",
		`src\components\Card.test.tsx`, "SRC/Post.TEST.ts",
	}
	for _, p := range yes {
		if !isTestFile(p) {
			t.Errorf("%s should be a test file", p)
		}
	}
	// A bare test/ or tests/ folder, or a word like "latest", is a real app path:
	// a quiz app can have a /tests page that really calls /api/tests.
	no := []string{
		"src/app/tests/page.tsx", "src/test/Page.tsx", "src/lib/latest.ts", "src/contest.ts",
		"src/post.ts", "server/api.go", "app/views.py", "src/testimonials/Card.tsx", "src/app/spec/page.tsx",
	}
	for _, p := range no {
		if isTestFile(p) {
			t.Errorf("%s is app code, not a test file", p)
		}
	}
}

// Every kind of call node gets the flag, whichever scanner made it.
func TestExtractMarksCallsInTestFilesOnly(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"src/screen.ts":      `export const a = () => fetch("/api/items")`,
		"src/screen.test.ts": `it("x", () => fetch("/api/items"))`,
		"src/page.spec.tsx":  `export const b = () => fetch("/api/other")`,
		"app/views.py":       "import requests\ndef a():\n    return requests.get('/api/items')\n",
		"app/test_views.py":  "import requests\ndef test_a():\n    return requests.get('/api/items')\n",
	}
	for rel, src := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g, err := Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind == graph.NetworkCall {
			got[filepath.ToSlash(n.File)] = n.InTest
		}
	}
	want := map[string]bool{
		"src/screen.ts": false, "src/screen.test.ts": true, "src/page.spec.tsx": true,
		"app/views.py": false, "app/test_views.py": true,
	}
	for file, inTest := range want {
		if v, ok := got[file]; !ok {
			t.Errorf("no call extracted from %s (got %v)", file, got)
		} else if v != inTest {
			t.Errorf("%s: InTest = %v, want %v", file, v, inTest)
		}
	}
	// And the graph's own questions follow it: only the screen's call is "a call to it".
	if n := len(g.CallsTo("/api/items")); n != 2 {
		t.Errorf("CallsTo(/api/items) = %d, want the 2 app calls (screen.ts, views.py)", n)
	}
	if n := len(g.TestCallsTo("/api/items")); n != 2 {
		t.Errorf("TestCallsTo(/api/items) = %d, want the 2 test calls", n)
	}
}
