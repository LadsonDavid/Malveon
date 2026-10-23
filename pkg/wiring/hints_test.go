package wiring

import (
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// A plain-language plan line ("Members can bookmark a post") names nothing
// malveon can look up, so it stays unchecked (SKIPPED) — that never changes.
// What used to be missing is any help: the line told the writer to "add the
// URL or file" without saying which URL or file was closest. Found
// 2026-09-30, running plain lines against a real Next.js app (a real Next.js app).
func TestUncheckedLineShowsClosestThingsInTheCode(t *testing.T) {
	g := &graph.Graph{Files: []string{
		"src/app/api/bookmarks/route.ts", "src/app/api/bookmarks/[id]/route.ts",
		"src/app/api/feed/route.ts", "src/components/BookmarkButton.tsx",
	}, Nodes: []graph.Node{
		route("/api/bookmarks", "POST", "src/app/api/bookmarks/route.ts"),
		route("/api/bookmarks/:id", "DELETE", "src/app/api/bookmarks/[id]/route.ts"),
		route("/api/feed", "GET", "src/app/api/feed/route.ts"),
		call("/api/bookmarks", "POST", "src/components/BookmarkButton.tsx"),
	}}

	r := verdictOf(t, g, "Members can bookmark a post")
	if r.Verdict != Skipped || !r.Unchecked {
		t.Fatalf("got %s (%s): a plain line must stay unchecked, hints never change the verdict", r.Verdict, r.Reason)
	}
	if len(r.Closest) == 0 {
		t.Fatal("no closest-in-code hints for a line about bookmarks")
	}
	if !strings.Contains(r.Closest[0], "POST /api/bookmarks") {
		t.Errorf("first hint = %q, want the bookmarks route", r.Closest[0])
	}
	for _, h := range r.Closest {
		if strings.Contains(h, "/api/feed") {
			t.Errorf("hint %q shares no word with the line", h)
		}
	}
	// The same URL is listed once, not once per route and once per call.
	seen := map[string]int{}
	for _, h := range r.Closest {
		seen[strings.SplitN(h, " (", 2)[0]]++
	}
	for h, n := range seen {
		if n > 1 {
			t.Errorf("%s listed %d times", h, n)
		}
	}
}

func TestNoHintsWhenNothingIsClose(t *testing.T) {
	g := &graph.Graph{Nodes: []graph.Node{route("/api/feed", "GET", "f.ts")}}
	if r := verdictOf(t, g, "Zero 'Coming Soon' placeholder pages remain in the Admin Hub."); len(r.Closest) != 0 {
		t.Errorf("got hints %v for a line that matches nothing", r.Closest)
	}
}

// Lines with a real verdict never carry hints.
func TestNamedLinesHaveNoHints(t *testing.T) {
	g := &graph.Graph{Nodes: []graph.Node{
		route("/api/jobs", "POST", "api/jobs/route.ts"),
		call("/api/jobs", "POST", "PostJobModal.tsx"),
	}}
	if r := verdictOf(t, g, "Posting a job calls `POST /api/jobs`"); len(r.Closest) != 0 {
		t.Errorf("a line with a verdict got hints: %v", r.Closest)
	}
}

func TestStem(t *testing.T) {
	for in, want := range map[string]string{
		"bookmarks": "bookmark", "bookmark": "bookmark", "categories": "category",
		"addresses": "address", "boxes": "box", "status": "status", "class": "class",
		"jobs": "job", "ids": "ids",
	} {
		if got := stem(in); got != want {
			t.Errorf("stem(%q) = %q, want %q", in, got, want)
		}
	}
}

// The route /api/reports/:id and a call to /api/reports/:param are one URL.
func TestHintsFoldPlaceholderSpellings(t *testing.T) {
	g := &graph.Graph{Files: []string{"a/reports/[id]/route.ts", "m.tsx"}, Nodes: []graph.Node{
		route("/api/reports/:id", "PATCH", "a/reports/[id]/route.ts"),
		call("/api/reports/:param", "PATCH", "m.tsx"),
	}}
	r := verdictOf(t, g, "Members can report a post")
	if len(r.Closest) != 1 || !strings.Contains(r.Closest[0], "a/reports/[id]/route.ts") {
		t.Fatalf("want one hint pointing at the route file, got %v", r.Closest)
	}
}
