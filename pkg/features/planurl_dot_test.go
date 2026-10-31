package features

import "testing"

// open-source routes the plan reader used to drop: `/.well-known/...` (starts with a dot) and a
// document served on a route when the line names the method (`GET /swagger.json`).
func TestPlanURLsKeepWellKnownAndMethodedFilenames(t *testing.T) {
	cases := []struct{ line, want, method string }{
		{"`GET /.well-known/openid-configuration` serves item 1", "/.well-known/openid-configuration", "GET"},
		{"`GET /swagger.json` serves item 2", "/swagger.json", "GET"},
	}
	for _, c := range cases {
		us := PlanURLs(c.line)
		if len(us) != 1 || us[0].Path != c.want || us[0].Method != c.method {
			t.Errorf("%q: got %+v, want %s %s", c.line, us, c.method, c.want)
		}
	}
	// a file name with no method is still a file
	if us := PlanURLs("move `src/app/(landing)/layout.tsx` to /app"); len(us) != 1 || us[0].Path != "/app" {
		t.Errorf("layout.tsx must stay a file, got %+v", us)
	}
}
