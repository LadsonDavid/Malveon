package features

import "testing"

func TestLooksLikeRoute(t *testing.T) {
	cases := []struct {
		text      string
		backtick  bool
		want      bool
		wantNamed string
	}{
		{"add /api/search for the jobs", false, true, "two segments"},
		{"make GET /refund return the amount", false, true, "with its method"},
		{"make the /pricing page show a banner", false, true, "called a page"},
		{"add a route /export that sends csv", false, true, "called a route"},
		{"make `/pricing` show a banner", true, true, "a user's backticks"},
		{"Run `/mcp` and confirm", false, false, "a command in an agent's prose"},
		{"Run `/mcp` and confirm", true, true, "backticks count for a user"},
		{"load /software then add search", false, false, "a skill"},
		{"load /software then add search", true, false, "a skill, even for a user"},
		{"Data is saved to `/tmp`", false, false, "a one-word path"},
	}
	for _, c := range cases {
		urls := PlanURLs(c.text)
		if len(urls) != 1 {
			t.Fatalf("%q: %d urls", c.text, len(urls))
		}
		if got := LooksLikeRoute(c.text, urls[0], c.backtick); got != c.want {
			t.Errorf("%s: %q (backtick %v) = %v, want %v", c.wantNamed, c.text, c.backtick, got, c.want)
		}
	}
}
