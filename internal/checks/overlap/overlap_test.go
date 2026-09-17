package overlap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LadsonDavid/beta-test/internal/graph"
)

func TestRun(t *testing.T) {
	g := &graph.Graph{Nodes: []graph.Node{
		// True collision: same method, identical literal path, two files.
		{Kind: graph.RouteHandler, File: "a.js", Line: 1, Method: "POST", Path: "/refund", Confidence: graph.Extracted},
		{Kind: graph.RouteHandler, File: "b.js", Line: 5, Method: "POST", Path: "/refund", Confidence: graph.Extracted},

		// True collision via wildcard equivalence: a literal and a
		// parameterized route claiming overlapping URL space.
		{Kind: graph.RouteHandler, File: "c.js", Line: 2, Method: "GET", Path: "/profile/:id", Confidence: graph.Extracted},
		{Kind: graph.RouteHandler, File: "d.js", Line: 9, Method: "GET", Path: "/profile/123", Confidence: graph.Extracted},

		// NOT a collision: same path, different method.
		{Kind: graph.RouteHandler, File: "e.js", Line: 3, Method: "GET", Path: "/settings", Confidence: graph.Extracted},
		{Kind: graph.RouteHandler, File: "f.js", Line: 4, Method: "POST", Path: "/settings", Confidence: graph.Extracted},

		// NOT a collision: unresolved method, never guessed.
		{Kind: graph.RouteHandler, File: "g.js", Line: 1, Method: "", Path: "/mystery", Confidence: graph.Extracted},
		{Kind: graph.RouteHandler, File: "h.js", Line: 1, Method: "", Path: "/mystery", Confidence: graph.Extracted},

		// NOT a collision: dynamic path, never compared.
		{Kind: graph.RouteHandler, File: "i.js", Line: 1, Method: "GET", Path: "/dyn", Confidence: graph.Inferred},
		{Kind: graph.RouteHandler, File: "j.js", Line: 1, Method: "GET", Path: "/dyn", Confidence: graph.Inferred},
	}}

	findings := Run(g, t.TempDir())
	if len(findings) != 2 {
		t.Fatalf("expected exactly 2 overlap findings, got %d: %+v", len(findings), findings)
	}

	for _, f := range findings {
		if len(f.Nodes) != 2 {
			t.Errorf("finding %+v: expected 2 colliding nodes, got %d", f, len(f.Nodes))
		}
		for _, note := range f.Notes {
			if note != "" {
				t.Errorf("finding %+v: none of these nodes have an EnclosingFunc, expected no unreachable notes, got %q", f, note)
			}
		}
	}
}

// TestRunFlagsPossiblyUnreachable covers the real gap a reviewer asked
// about: a route registered inside a function that's never called from
// anywhere else in the codebase should be distinguishable from a route
// that's definitely live, even though both still collide on method+path.
func TestRunFlagsPossiblyUnreachable(t *testing.T) {
	dir := t.TempDir()
	// setupLegacyRoutes is never referenced anywhere else in this tree —
	// dead code, but its registration still collides on method+path.
	writeFile(t, dir, "legacy.js", `
const app = require("express")();
function setupLegacyRoutes() {
  app.post("/refund", function (req, res) {});
}
`)
	writeFile(t, dir, "live.js", `
const app = require("express")();
app.post("/refund", function (req, res) {});
`)

	g := &graph.Graph{Nodes: []graph.Node{
		{Kind: graph.RouteHandler, File: "legacy.js", Line: 4, Method: "POST", Path: "/refund", Confidence: graph.Extracted, EnclosingFunc: "setupLegacyRoutes"},
		{Kind: graph.RouteHandler, File: "live.js", Line: 3, Method: "POST", Path: "/refund", Confidence: graph.Extracted},
	}}

	findings := Run(g, dir)
	if len(findings) != 1 {
		t.Fatalf("expected 1 overlap finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if len(f.Notes) != 2 {
		t.Fatalf("expected 2 notes (one per node), got %d", len(f.Notes))
	}
	if f.Notes[0] == "" {
		t.Errorf("expected legacy.js node (inside never-referenced setupLegacyRoutes) to be flagged possibly unreachable, got no note")
	}
	if f.Notes[1] != "" {
		t.Errorf("expected live.js node (top-level, no enclosing func) to have no note, got %q", f.Notes[1])
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunSuppressesNextJSStaticDynamicPair covers the real false
// positive a reviewer found: /api/circles/mine (static) and
// /api/circles/[id] (dynamic) in two separate route.ts files never
// actually collide in Next.js App Router — static always wins,
// deterministically, regardless of file order.
func TestRunSuppressesNextJSStaticDynamicPair(t *testing.T) {
	dir := t.TempDir()
	g := &graph.Graph{Nodes: []graph.Node{
		{Kind: graph.RouteHandler, File: "src/app/api/circles/[id]/route.ts", Line: 3, Method: "GET", Path: "/api/circles/:id", Confidence: graph.Extracted},
		{Kind: graph.RouteHandler, File: "src/app/api/circles/mine/route.ts", Line: 3, Method: "GET", Path: "/api/circles/mine", Confidence: graph.Extracted},
	}}
	findings := Run(g, dir)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings (Next.js static/dynamic split is never ambiguous), got %d: %+v", len(findings), findings)
	}
}

// TestRunSuppressesSameFileStaticFirst covers the one ordering that's
// safe under every researched router, order-dependent or not: a static
// route registered before a dynamic one in the same file.
func TestRunSuppressesSameFileStaticFirst(t *testing.T) {
	dir := t.TempDir()
	g := &graph.Graph{Nodes: []graph.Node{
		{Kind: graph.RouteHandler, File: "routes.js", Line: 5, Method: "GET", Path: "/users/me", Confidence: graph.Extracted},
		{Kind: graph.RouteHandler, File: "routes.js", Line: 12, Method: "GET", Path: "/users/:id", Confidence: graph.Extracted},
	}}
	findings := Run(g, dir)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings (static registered before dynamic, safe under every researched router), got %d: %+v", len(findings), findings)
	}
}

// TestRunKeepsFlaggingSameFileDynamicFirst covers the one ordering that
// IS a real, confirmed bug under order-dependent routers (Express,
// Flask, FastAPI, gorilla/mux): a dynamic route registered before the
// static one it would otherwise swallow.
func TestRunKeepsFlaggingSameFileDynamicFirst(t *testing.T) {
	dir := t.TempDir()
	g := &graph.Graph{Nodes: []graph.Node{
		{Kind: graph.RouteHandler, File: "routes.js", Line: 5, Method: "GET", Path: "/users/:id", Confidence: graph.Extracted},
		{Kind: graph.RouteHandler, File: "routes.js", Line: 12, Method: "GET", Path: "/users/me", Confidence: graph.Extracted},
	}}
	findings := Run(g, dir)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (dynamic route registered first genuinely swallows the static one under order-dependent routers), got %d: %+v", len(findings), findings)
	}
}
