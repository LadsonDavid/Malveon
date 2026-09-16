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

func TestFindNodesMatching(t *testing.T) {
	g := &Graph{Nodes: []Node{
		{Kind: RouteHandler, Path: "/refund", Words: []string{"server", "refund"}},
		{Kind: RouteHandler, Path: "/cancel-order-info", Words: []string{"server", "cancel", "order", "info"}},
		{Kind: NetworkCall, Path: "/refund", Words: []string{"app", "refund"}},
	}}

	matches := g.FindNodesMatching(RouteHandler, []string{"refund", "button"})
	if len(matches) != 1 || matches[0].Path != "/refund" {
		t.Fatalf("expected exactly the /refund route, got %+v", matches)
	}

	none := g.FindNodesMatching(RouteHandler, []string{"totally", "unrelated"})
	if len(none) != 0 {
		t.Fatalf("expected no matches, got %+v", none)
	}
}

// TestFindNodesMatchingExcludesCommonWords covers the real matching bug
// a reviewer found: "Restore /admin/circles" matched an unrelated
// "/api/admin/announcements" route because both merely shared the word
// "admin" — a word so common across the app's own routes that it no
// longer identifies anything. A large synthetic graph reproduces that
// shape: many "admin"-tagged nodes plus one real, specific match.
func TestFindNodesMatchingExcludesCommonWords(t *testing.T) {
	var nodes []Node
	for i := 0; i < 20; i++ {
		nodes = append(nodes, Node{Kind: RouteHandler, Path: "/admin/x", Words: []string{"admin", "x"}})
	}
	nodes = append(nodes, Node{Kind: RouteHandler, Path: "/admin/circles", Words: []string{"admin", "circles"}})
	g := &Graph{Nodes: nodes}

	// "Restore /admin/circles" only shares "admin" with the 20 generic
	// nodes — that word must not be enough to match them.
	byAdminAlone := g.FindNodesMatching(RouteHandler, []string{"restore", "admin", "squad", "management"})
	if len(byAdminAlone) != 0 {
		t.Fatalf("expected 0 matches via the over-common word \"admin\" alone, got %d: %+v", len(byAdminAlone), byAdminAlone)
	}

	// The real, specific word "circles" must still work.
	byCircles := g.FindNodesMatching(RouteHandler, []string{"circles", "accountability"})
	if len(byCircles) != 1 || byCircles[0].Path != "/admin/circles" {
		t.Fatalf("expected exactly the /admin/circles route via the real word \"circles\", got %+v", byCircles)
	}
}

// TestFindNodesMatchingExcludesStopWords covers the second half of the
// real matching bug: "All 6 Settings tabs..." matched an unrelated
// /api/notifications/read-all route because "all" is both a generic
// English word and a URL path fragment — too rare in this graph to trip
// the frequency filter, but still not a real signal.
func TestFindNodesMatchingExcludesStopWords(t *testing.T) {
	g := &Graph{Nodes: []Node{
		{Kind: RouteHandler, Path: "/api/notifications/read-all", Words: []string{"api", "notifications", "read", "all"}},
		{Kind: RouteHandler, Path: "/circles", Words: []string{"circles"}},
	}}
	matches := g.FindNodesMatching(RouteHandler, []string{"all", "6", "settings", "tabs", "persist", "data"})
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches via the stopword \"all\" alone, got %d: %+v", len(matches), matches)
	}
}

// TestFindNodesMatchingSmallGraphUnaffected proves the min-count floor
// protects a small graph (a young project, a test fixture): a word
// shared by every node there is normal, not noise, and must still match.
func TestFindNodesMatchingSmallGraphUnaffected(t *testing.T) {
	g := &Graph{Nodes: []Node{
		{Kind: RouteHandler, Path: "/admin/settings", Words: []string{"admin", "settings"}},
		{Kind: NetworkCall, Path: "/admin/settings", Words: []string{"admin", "settings"}},
	}}
	matches := g.FindNodesMatching(NetworkCall, []string{"admin", "settings"})
	if len(matches) != 1 {
		t.Fatalf("expected the shared word \"admin\" to still match in a small graph, got %d: %+v", len(matches), matches)
	}
}
