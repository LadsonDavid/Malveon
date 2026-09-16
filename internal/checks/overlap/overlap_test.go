package overlap

import (
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

	findings := Run(g)
	if len(findings) != 2 {
		t.Fatalf("expected exactly 2 overlap findings, got %d: %+v", len(findings), findings)
	}

	for _, f := range findings {
		if len(f.Nodes) != 2 {
			t.Errorf("finding %+v: expected 2 colliding nodes, got %d", f, len(f.Nodes))
		}
	}
}
