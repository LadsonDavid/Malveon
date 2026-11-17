package wiring

import (
	"github.com/LadsonDavid/Malveon/pkg/graph"
	"strings"
	"testing"
)

func TestContentRemovalDoesNotDemandFileDeletion(t *testing.T) {
	g := &graph.Graph{Files: []string{"page.tsx"}}
	for _, claim := range []string{
		"Remove the paragraph containing `Example hint` in `page.tsx`, preserving the list.",
		"Delete the label from `page.tsx`.",
		"`page.tsx` — remove the paragraph.",
		"Remove `page.tsx` from the navigation menu.",
	} {
		r := verdictOf(t, g, claim)
		if r.Verdict != Skipped || !r.Unchecked {
			t.Fatalf("content removal is not file deletion: %s: %+v", claim, r)
		}
		if strings.Contains(r.Reason, "is still there") {
			t.Fatalf("false file removal accusation: %s", r.Reason)
		}
	}
	for _, claim := range []string{"Remove `page.tsx`", "Delete the file `page.tsx`.", "`page.tsx` — delete (stale)", "Remove `gone.tsx` and `page.tsx`"} {
		r := verdictOf(t, g, claim)
		if r.Verdict != NotBuilt {
			t.Fatalf("explicit file removal must still catch retained file: %s: %+v", claim, r)
		}
	}
	if r := verdictOf(t, g, "Remove `gone.tsx`"); r.Verdict != Working {
		t.Fatalf("absent file deletion: %+v", r)
	}
	if r := verdictOf(t, g, "Remove `gone.tsx` (keep `page.tsx`)"); r.Verdict != Working {
		t.Fatalf("context file must not become deletion target: %+v", r)
	}
	g.Nodes = []graph.Node{{Kind: graph.NetworkCall, File: "client.tsx", Line: 1, Path: "/api/missing", Method: "DELETE", Confidence: graph.Extracted}, {Kind: graph.RouteHandler, File: "server.ts", Line: 1, Path: "/api/health", Method: "GET", Confidence: graph.Extracted}}
	if r := verdictOf(t, g, "Delete records via `DELETE /api/missing`"); r.Verdict != Broken {
		t.Fatalf("DELETE API checks must remain active: %+v", r)
	}
}
