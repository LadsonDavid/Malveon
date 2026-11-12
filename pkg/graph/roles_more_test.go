package graph

import "testing"

// Found by running malveon over 145 projects: benchmark solutions and *.fixture.* files are
// test material, not production code.
func TestMoreFixtureShapes(t *testing.T) {
	for _, f := range []string{
		"bench/harness/tasks/04-stack-trace-bug/solution/src/report/summary.ts",
		"packages/hub/test/agent-launch-postgres.fixture.mjs",
		"app/fixtures/data.js",
		"x/solutions/a.ts",
	} {
		if got := SourceRole(f); got != "fixture" {
			t.Errorf("%s: want fixture, got %s", f, got)
		}
	}
	for _, f := range []string{"src/solutionPage.ts", "src/fixturesLoader.ts", "src/app/solution.ts"} {
		if got := SourceRole(f); got != "production" {
			t.Errorf("%s: want production, got %s", f, got)
		}
	}
}
