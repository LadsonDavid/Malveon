package graph

import "testing"

func TestPathsMatch(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"/refund", "/refund", true},
		{"/refund", "/refund-info", false},
		{"/profile/123", "/profile/:id", true},
		{"/profile/123", "/profile/{id}", true},
		{"/profile/123", "/profile/<id>", true},     // Flask/Django route placeholder
		{"/profile/123", "/profile/<int:id>", true}, // Flask converter
		{"/profile/<id>", "/profile/:id", true},
		{"/profile", "/profile/:id", false}, // different segment count
		{"/cancel-order", "/cancel-order-info", false},
	}
	for _, c := range cases {
		got := PathsMatch(c.a, c.b)
		if got != c.want {
			t.Errorf("PathsMatch(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// TestContentWordsDropsOverCommonWords covers the real matching bug a
// reviewer found: "Restore /admin/circles" matched an unrelated
// "/api/admin/announcements" route because both merely shared "admin" —
// a word so common across the app's own routes it identifies nothing.
func TestContentWordsDropsOverCommonWords(t *testing.T) {
	var nodes []Node
	for i := 0; i < 20; i++ {
		nodes = append(nodes, Node{Kind: RouteHandler, Path: "/admin/x", Words: []string{"admin", "x"}})
	}
	g := &Graph{Nodes: nodes}
	got := g.ContentWords([]string{"Restore", "admin", "circles", "all"})
	if len(got) != 2 || got[0] != "restore" || got[1] != "circles" {
		t.Fatalf("got %v, want [restore circles] — \"admin\" is over-common here, \"all\" is a stopword", got)
	}
}

// A small graph (a young project, a fixture) never has "over-common"
// words: a word every node shares there is normal, not noise.
func TestContentWordsSmallGraphUnaffected(t *testing.T) {
	g := &Graph{Nodes: []Node{
		{Kind: RouteHandler, Path: "/admin/settings", Words: []string{"admin", "settings"}},
		{Kind: NetworkCall, Path: "/admin/settings", Words: []string{"admin", "settings"}},
	}}
	if got := g.ContentWords([]string{"admin", "settings"}); len(got) != 2 {
		t.Fatalf("got %v, want both words kept in a small graph", got)
	}
}

func TestLookups(t *testing.T) {
	g := &Graph{
		Files: []string{`src\components\Footer.tsx`, "docs/PLAN.md"},
		Nodes: []Node{
			{Kind: RouteHandler, Path: "/api/upload/sign", Confidence: Extracted},
			{Kind: RouteHandler, Path: "/api/jobs/:id", Confidence: Extracted},
			{Kind: NetworkCall, Path: "/api/jobs/:param", Confidence: Extracted},
			{Kind: NetworkCall, Path: "", Confidence: Inferred},
		},
	}
	if n := len(g.RoutesUnder("/api/upload")); n != 1 {
		t.Errorf("RoutesUnder(/api/upload) = %d routes, want 1", n)
	}
	if n := len(g.RoutesServing("/api/jobs/42")); n != 1 {
		t.Errorf("RoutesServing(/api/jobs/42) = %d routes, want 1", n)
	}
	if n := len(g.CallsTo("/api/jobs/:id")); n != 1 {
		t.Errorf("CallsTo(/api/jobs/:id) = %d calls, want 1 (the unresolvable call never counts)", n)
	}
	if got := g.FindFiles("src/components/Footer.tsx"); len(got) != 1 {
		t.Errorf("FindFiles by path = %v, want the Windows-separated file", got)
	}
	if got := g.FindFiles("Footer.tsx"); len(got) != 1 {
		t.Errorf("FindFiles by name = %v", got)
	}
	if got := g.FindFiles("ooter.tsx"); len(got) != 0 {
		t.Errorf("FindFiles must match whole names, got %v", got)
	}
}

func TestSplitWords(t *testing.T) {
	got := SplitWords("Wire `PostJobModal` into (`/api/jobs`):")
	want := []string{"wire", "post", "job", "modal", "into", "api", "jobs"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// MentionsWord: does any code (identifiers, split at camelCase/snake_case,
// or file names) mention w, singular or plural, case-insensitively.
func TestMentionsWord(t *testing.T) {
	g := &Graph{
		Files: []string{"src/components/OrderNotes.tsx"},
		Names: map[string]map[string]bool{"src/api.ts": {"RefundButton": true, "cancel_order": true}},
	}
	for w, want := range map[string]bool{
		"refund": true, "refunds": true, "button": true, "cancel": true, "order": true,
		"orders": true, "note": true, "notes": true, "checkout": false, "invoice": false,
	} {
		if got := g.MentionsWord(w); got != want {
			t.Errorf("MentionsWord(%q) = %v, want %v", w, got, want)
		}
	}
}

// A route's own wildcard is not "under" a literal plan segment: the route
// /api/notifications/:id/read is not nested below /api/notifications/read-all.
// A wildcard in the plan prefix still matches anything.
func TestRoutesUnderLiteralPrefixSegmentsMatchLiterally(t *testing.T) {
	g := &Graph{Nodes: []Node{
		{Kind: RouteHandler, Path: "/api/notifications/:id/read", Confidence: Extracted},
		{Kind: RouteHandler, Path: "/api/upload/sign", Confidence: Extracted},
	}}
	if n := len(g.RoutesUnder("/api/notifications/read-all")); n != 0 {
		t.Errorf("RoutesUnder(/api/notifications/read-all) = %d routes, want 0", n)
	}
	if n := len(g.RoutesUnder("/api/notifications/:id")); n != 1 {
		t.Errorf("RoutesUnder(/api/notifications/:id) = %d routes, want 1", n)
	}
	if n := len(g.RoutesUnder("/api/upload")); n != 1 {
		t.Errorf("RoutesUnder(/api/upload) = %d routes, want 1", n)
	}
}

func TestPathsMatchCatchAll(t *testing.T) {
	cases := []struct {
		call, route string
		want        bool
	}{
		{"/api/docs/guide/intro", "/api/docs/:...slug", true},
		{"/api/docs/guide", "/api/docs/:...slug", true},
		{"/api/docs", "/api/docs/:...slug", false}, // [...slug] needs at least one segment
		{"/api/docs", "/api/docs/:...slug?", true}, // [[...slug]] may match none
		{"/api/other/x", "/api/docs/:...slug", false},
		{"/api/docs/:param/x", "/api/docs/:...slug", true},
	}
	for _, c := range cases {
		if got := PathsMatch(c.call, c.route); got != c.want {
			t.Errorf("PathsMatch(%q, %q) = %v, want %v", c.call, c.route, got, c.want)
		}
	}
}
