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
