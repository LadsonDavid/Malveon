package features

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadJSON(t *testing.T) {
	p := writeTemp(t, "features.json", `[{"id":"refund-button","name":"refund button"}]`)
	fs, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(fs) != 1 || fs[0].ID != "refund-button" || fs[0].Name != "refund button" {
		t.Fatalf("unexpected result: %+v", fs)
	}
}

func TestLoadMarkdownChecklist(t *testing.T) {
	p := writeTemp(t, "plan.md", `# Sprint plan

- [ ] Refund button
- [x] Cancel order
* Profile update
1. Update settings

Some prose that isn't a checklist item.
`)
	fs, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	wantNames := []string{"Refund button", "Cancel order", "Profile update", "Update settings"}
	if len(fs) != len(wantNames) {
		t.Fatalf("got %d features, want %d: %+v", len(fs), len(wantNames), fs)
	}
	for i, name := range wantNames {
		if fs[i].Name != name {
			t.Errorf("feature %d: got name %q, want %q", i, fs[i].Name, name)
		}
		if fs[i].ID == "" {
			t.Errorf("feature %d: expected a generated ID, got empty", i)
		}
	}
}

func TestLoadPlainText(t *testing.T) {
	p := writeTemp(t, "plan.txt", "refund button\n# a comment, skipped\n\ncancel order\n")
	fs, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(fs) != 2 {
		t.Fatalf("got %d features, want 2: %+v", len(fs), fs)
	}
	if fs[0].Name != "refund button" || fs[1].Name != "cancel order" {
		t.Errorf("unexpected names: %+v", fs)
	}
}

func TestDetect(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"PLAN.md", "features.json", "README.md", "notes.txt", "server.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	candidates, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	want := map[string]bool{
		filepath.Join(dir, "PLAN.md"):       true,
		filepath.Join(dir, "features.json"): true,
	}
	if len(candidates) != len(want) {
		t.Fatalf("got %d candidates, want %d: %v", len(candidates), len(want), candidates)
	}
	for _, c := range candidates {
		if !want[c] {
			t.Errorf("unexpected candidate: %s (README.md, notes.txt, server.go should never match)", c)
		}
	}
}

func TestDetectNoMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "server.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidates, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates, got %v", candidates)
	}
}

func writeIn(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectByContentFindsUnnamedFiles(t *testing.T) {
	dir := t.TempDir()

	// No name hint at all, but the content genuinely looks like a plan.
	writeIn(t, dir, "sprint3.json", `[{"id":"a","name":"refund button"},{"id":"b","name":"cancel order"}]`)
	writeIn(t, dir, "notes.md", "# Notes\n\n- [ ] Refund button\n- [x] Cancel order\n")

	// Should NOT be picked up: valid JSON, but not shaped like our
	// feature list (no "name" field), and a markdown file with fewer
	// than 2 real checklist lines.
	writeIn(t, dir, "config.json", `{"port": 8080, "debug": true}`)
	writeIn(t, dir, "CHANGELOG.md", "# Changelog\n\n## v1.0\n\nInitial release.\n")

	candidates, err := DetectByContent(dir)
	if err != nil {
		t.Fatalf("DetectByContent: %v", err)
	}

	want := map[string]bool{
		filepath.Join(dir, "sprint3.json"): true,
		filepath.Join(dir, "notes.md"):     true,
	}
	if len(candidates) != len(want) {
		t.Fatalf("got %d candidates, want %d: %v", len(candidates), len(want), candidates)
	}
	for _, c := range candidates {
		if !want[c] {
			t.Errorf("unexpected candidate: %s (config.json, CHANGELOG.md should never match)", c)
		}
	}
}

func TestLoadDuplicateNamesGetDistinctIDs(t *testing.T) {
	p := writeTemp(t, "plan.txt", "Refund Button\nrefund button\n")
	fs, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(fs) != 2 {
		t.Fatalf("got %d features, want 2: %+v", len(fs), fs)
	}
	if fs[0].ID == fs[1].ID {
		t.Errorf("expected distinct IDs for two features that slug the same, got %q twice", fs[0].ID)
	}
}

// TestDetectRecursesIntoSubdirectories covers the real bug a reviewer
// hit: a plan file kept in docs/PLAN.md (a completely normal place to
// keep one) was invisible to the old top-level-only scan, so a much
// weaker content-based match at the root won by default instead.
func TestDetectRecursesIntoSubdirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeIn(t, dir, filepath.Join("docs", "PLAN.md"), "- [ ] Refund button\n- [x] Cancel order\n")
	writeIn(t, dir, "README.md", "# Getting started\n\n- [ ] Clone the repo\n- [ ] Run npm install\n")

	candidates, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	want := filepath.Join(dir, "docs", "PLAN.md")
	if len(candidates) != 1 || candidates[0] != want {
		t.Fatalf("got %v, want exactly [%s] — name-based detection should find docs/PLAN.md and never match README.md by name", candidates, want)
	}
}

// TestDetectSkipsNoiseDirectories proves recursion doesn't wander into
// node_modules/.git/.next and accidentally treat something in there as
// a plan candidate.
func TestDetectSkipsNoiseDirectories(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"node_modules", ".git", ".next"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
		writeIn(t, dir, filepath.Join(sub, "PLAN.md"), "- [ ] should never be found\n")
	}

	candidates, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates (all matches sit inside skipped dirs), got %v", candidates)
	}
}

// TestLoadAllMergesMultipleDocs proves --features can be checked against
// more than one plan doc at once: the results merge into one flat list.
func TestLoadAllMergesMultipleDocs(t *testing.T) {
	a := writeTemp(t, "a.json", `[{"id":"refund-button","name":"refund button"}]`)
	b := writeTemp(t, "b.json", `[{"id":"cancel-order","name":"cancel order"}]`)

	fs, warnings, err := LoadAll([]string{a, b})
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no duplicate warnings, got %v", warnings)
	}
	if len(fs) != 2 || fs[0].ID != "refund-button" || fs[1].ID != "cancel-order" {
		t.Fatalf("unexpected merged result: %+v", fs)
	}
}

// TestLoadAllReportsDuplicateIDsInsteadOfSilentlyDropping proves a
// collision between two docs is never silently dropped — same "never
// guess, never go quiet" rule as everywhere else in this package.
func TestLoadAllReportsDuplicateIDsInsteadOfSilentlyDropping(t *testing.T) {
	a := writeTemp(t, "a.json", `[{"id":"refund-button","name":"refund button"}]`)
	b := writeTemp(t, "b.json", `[{"id":"refund-button","name":"refund button but reworded"}]`)

	fs, warnings, err := LoadAll([]string{a, b})
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("expected the first doc's feature to win, got %+v", fs)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "refund-button") {
		t.Fatalf("expected one warning naming the duplicate id, got %v", warnings)
	}
}

// The tests below cover Classify and PlanURLs (2026-09-23): which plan
// lines are code tasks at all, and which URLs a line names.

func TestClassify_OrdinaryShortFeatureNamesAreTasks(t *testing.T) {
	// Real names from testdata/fixture's features.json — genuine features
	// with no connectivity vocabulary. Skipping one would hide a real result.
	for _, name := range []string{"refund button", "cancel order", "profile", "update settings"} {
		if skip, why := Classify(Feature{Name: name}); skip {
			t.Errorf("%q skipped as %q, want it checked", name, why)
		}
	}
}

func TestClassify_SkipsWhatIsNotACodeTask(t *testing.T) {
	cases := map[string]string{
		"**Job Board (`/jobs`)**:":                                                   "heading",
		"`npm run build` succeeds cleanly with zero errors.":                         "build, test",
		"Open the landing page in the browser at desktop and mobile":                 "manual step",
		"`lucide-react` stays as the app's icon library":                             "note or decision",
		"Force stroke={1.5} explicitly on every Tabler icon so it matches Heroicons": "design",
		"`src/app/(landing)/layout.tsx` — font loading + wrapper div":                "design",
		// Icon names include "Database" — still an icon list, not a database task.
		"**Tabler Icons** — all feature icons: Rocket, Settings, Palette, Database, Bot": "design",
	}
	for name, want := range cases {
		skip, why := Classify(Feature{Name: name})
		if !skip || !strings.Contains(why, want) {
			t.Errorf("%q: skip=%v why=%q, want skipped as %q", name, skip, why, want)
		}
	}
}

// Task-metadata bullets from an agent-written plan (found 2026-09-30, a
// 454-line plan became 184 plan lines, most of them these) are not tasks.
func TestClassify_SkipsTaskMetadataLabels(t *testing.T) {
	for _, name := range []string{
		"**Task ID**: `TASK-01`",
		"**Agent**: `backend-specialist`",
		"**Skills**: `api-patterns`, `clean-code`",
		"**Priority**: P0 (Blocker for the launch)",
		"**Priority:** P0",
		"**Dependencies**: None",
		"Depends on: TASK-01",
		"Owner: alice",
		"`INPUT`: Request payload `{ postId: string }`",
		"`OUTPUT`: JSON with the new row",
		"**Status**: `WITHDRAWN`", // an all-caps value after a label is still a value
		"**Acceptance criteria**: the list loads",
	} {
		skip, why := Classify(Feature{Name: name})
		if !skip || !strings.Contains(why, "metadata") {
			t.Errorf("%q: skip=%v why=%q, want skipped as task metadata", name, skip, why)
		}
	}
}

// A label line that also names a URL, a file with an extension, or a
// code-shaped name stays checkable — the line is doing more than labelling.
func TestClassify_MetadataLabelNamingSomethingStaysChecked(t *testing.T) {
	for _, name := range []string{
		"**Output**: `POST /api/refunds` returns the new refund",
		"`INPUT`: `src/app/foo.tsx` posts the form",
		"Task: wire `PostJobModal` to the server",
		"**Owner**: `notifications_worker`",
	} {
		if skip, why := Classify(Feature{Name: name}); skip {
			t.Errorf("%q skipped as %q, want it checked", name, why)
		}
	}
	// A word that merely starts like a label is not one.
	for _, name := range []string{"Statuses tab shows every member", "Typeahead search box"} {
		if skip, why := Classify(Feature{Name: name}); skip && strings.Contains(why, "metadata") {
			t.Errorf("%q skipped as metadata (%q), it is a real line", name, why)
		}
	}
}

func TestClassify_ConnectionWordsKeepALineChecked(t *testing.T) {
	for _, name := range []string{
		"Resources Library (`/api/resources`): CRUD operations for tutorials",
		"Persist the user's chosen color theme to the database", // design and connection words tie: checked
	} {
		if skip, why := Classify(Feature{Name: name}); skip {
			t.Errorf("%q skipped as %q, want it checked", name, why)
		}
	}
}

func TestPlanURLs(t *testing.T) {
	got := PlanURLs("Wire the button into `POST /api/jobs` and `/circles/[id]`, see `src/app/global.css` and `src/app/(landing)/layout.tsx`")
	want := []PlanURL{{Path: "/api/jobs", Method: "POST"}, {Path: "/circles/:id"}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v — file paths must never be read as URLs", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("url %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

// Every common way to write a path placeholder is part of the URL (found
// 2026-09-29: `DELETE /api/users/<id>` was cut to /api/users and matched the
// unrelated GET list route, a false HALF BUILT saying the route existed).
func TestPlanURLsPlaceholderStyles(t *testing.T) {
	for line, want := range map[string]string{
		"`DELETE /api/users/<id>` removes a user":        "/api/users/:id",       // Flask, Django
		"`DELETE /api/users/<int:id>` removes a user":    "/api/users/:id",       // Flask converter
		"`DELETE /api/users/{id}` removes a user":        "/api/users/:id",       // FastAPI, Spring, .NET, Laravel, OpenAPI
		"`GET /api/users/{id:int}/posts` lists posts":    "/api/users/:id/posts", // ASP.NET constraint
		"`DELETE /api/users/:id` removes a user":         "/api/users/:id",       // Express
		"`DELETE /api/users/[id]` removes a user":        "/api/users/:id",       // Next.js
		"`GET /api/users/{userId}/orders/{orderId}` ...": "/api/users/:userId/orders/:orderId",
	} {
		got := PlanURLs(line)
		if len(got) != 1 || got[0].Path != want {
			t.Errorf("PlanURLs(%q) = %v, want path %s", line, got, want)
		}
	}
}

// Each plan line knows where it came from, so an editor can point at it.
func TestLoadRecordsSource(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "PLAN.md")
	os.WriteFile(md, []byte("# Plan\n\n- [ ] Refund button\nprose\n- [x] Cancel order\n"), 0o644)
	txt := filepath.Join(dir, "plan.txt")
	os.WriteFile(txt, []byte("# comment\n\nExport CSV\n"), 0o644)
	js := filepath.Join(dir, "features.json")
	os.WriteFile(js, []byte(`[{"id":"a","name":"A"}]`), 0o644)

	for path, want := range map[string][]string{
		md:  {md + ":3", md + ":5"},
		txt: {txt + ":3"},
		js:  {js},
	} {
		fs, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, f := range fs {
			got = append(got, f.Source)
		}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%s: sources = %v, want %v", filepath.Base(path), got, want)
		}
	}
}

// Only the line's own words name an outside caller — never its URL.
func TestExpectsOutsideCaller(t *testing.T) {
	cases := map[string]bool{
		"add a GET `/api/health` route that returns `{ ok: true }`": false,
		"add a GET /api/health route that returns { ok: true }":     false,
		"add a `/api/health` health check":                          true,
		"Stripe webhook at `/api/stripe`":                           true,
		"nightly cron job hits `/api/digest`":                       true,
		"public API for partners (`/api/v1/jobs`)":                  true,
		"`/api/pay` called by Stripe":                               true,
		"the webhook settings button posts to `/api/hooks`":         false, // asks for an in-app caller
	}
	for line, want := range cases {
		if got := ExpectsOutsideCaller(Feature{Name: line}); got != want {
			t.Errorf("%q: got %v, want %v", line, got, want)
		}
	}
}

// A "- [ ]" line is future work the plan hasn't claimed yet; "- [x]" and a
// plain bullet are due now (found 2026-09-29: with a pre-commit hook, every
// unticked to-do blocked every commit until the whole plan was built).
func TestLoadRecordsUnticked(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "PLAN.md")
	os.WriteFile(md, []byte("- [ ] Export CSV\n- [x] Refund button\n- [X] Cancel order\n- Plain bullet\n1. Numbered\n* [ ] Star unticked\n"), 0o644)
	fs, err := Load(md)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"Export CSV": true, "Refund button": false, "Cancel order": false, "Plain bullet": false, "Numbered": false, "Star unticked": true}
	if len(fs) != len(want) {
		t.Fatalf("got %d lines, want %d: %+v", len(fs), len(want), fs)
	}
	for _, f := range fs {
		if f.Unticked != want[f.Name] {
			t.Errorf("%q: Unticked = %v, want %v", f.Name, f.Unticked, want[f.Name])
		}
	}
}

// A plan line "claims a connection" (the screen talks to the server or a
// database) by what it SAYS, not by the names it points at. Found 2026-09-30 on
// an agent-written plan for a real Next.js app: "`src/types/database.ts` adds report and
// notification types" counted as claiming a database call because the file is
// called database.ts, and "...renders the tabs based on query parameters"
// because of "query" — so a types file and a page that correctly make no
// call were marked HALF BUILT, and would have blocked once ticked.
func TestClaimsConnectionIgnoresFileNamesAndQueryStrings(t *testing.T) {
	for _, line := range []string{
		"`src/types/database.ts` adds report and notification types",
		"`src/app/(app)/settings/page.tsx` renders the Invites and Reports tabs based on query parameters",
		"`src/components/settings/index.ts` exports InvitesTab and ReportsTab",
		"Show the tab chosen by the query string",
	} {
		if ClaimsConnection(Feature{Name: line}) {
			t.Errorf("%q should not claim a server or database connection", line)
		}
	}
	for _, line := range []string{
		"Saving the profile writes to the database",
		"`ProfileTab` saves the settings",
		"`POST /api/reports` saves a report",
		"The list loads the members from the server",
	} {
		if !ClaimsConnection(Feature{Name: line}) {
			t.Errorf("%q says it talks to the server or database and should claim a connection", line)
		}
	}
}

// Found 2026-09-30 on the agent-written a real app plan: "`src/components/feed/PostCard.tsx`
// adds a report option to the post menu" and "`src/app/(app)/settings/layout.tsx`
// adds Invites and My Reports to the settings navigation" were SKIPPED as "a
// design or styling change" because the file names contain "card" and
// "layout". Two real tasks vanished from the report. A word inside backticks
// names a file or code; it says nothing about what the line is.
func TestClassifyDoesNotReadFileNamesAsDesignWords(t *testing.T) {
	for _, line := range []string{
		"`src/components/feed/PostCard.tsx` adds a report option to the post menu for non-authors",
		"`src/app/(app)/settings/layout.tsx` adds Invites and My Reports to the settings navigation",
	} {
		if skip, why := Classify(Feature{Name: line}); skip {
			t.Errorf("%q was skipped (%s), it is a code task", line, why)
		}
	}
	// Real design lines still skip.
	for _, line := range []string{
		"Force `stroke={1.5}` on every Tabler icon",
		"Use a soft gradient and a bolder headline on the hero",
	} {
		if skip, _ := Classify(Feature{Name: line}); !skip {
			t.Errorf("%q is a design line and should be skipped", line)
		}
	}
}

// A line that opens with a "manual step" verb but is really a feature
// (found by audit: "Check-in endpoint `POST /api/checkin`" was skipped, and a
// skipped line never blocks) stays checked.
func TestClassify_ManualVerbDoesNotHideARealFeature(t *testing.T) {
	for _, name := range []string{
		"Check-in endpoint `POST /api/checkin` is served and called",
		"Review queue: `GET /api/reviews` lists pending reviews",
		"Test results page at /admin/results shows the last run",
		"Try-on modal loads the photo",
	} {
		if skip, why := Classify(Feature{Name: name}); skip {
			t.Errorf("%q skipped as %q, want it checked", name, why)
		}
	}
	// ...and a real manual step still is one.
	if skip, _ := Classify(Feature{Name: "Check the console for errors after loading"}); !skip {
		t.Error("a plain manual step must still be skipped")
	}
}

// Bug 9 (sweep 2026-10-12): "Set `DATABASE_URL` in `.env`" is a setup step, not
// code. It read NOT BUILT in every project, because `.env` is never committed
// and DATABASE_URL isn't a name in the code.
func TestClassify_SkipsEnvironmentSetupSteps(t *testing.T) {
	for _, name := range []string{
		"Set `DATABASE_URL` in `.env`",
		"Add `STRIPE_KEY` to `.env.local`",
		"Configure the `SUPABASE_URL` environment variable",
		"Define the `API_KEY` env var before deploying",
	} {
		if skip, why := Classify(Feature{Name: name}); !skip || !strings.Contains(why, "environment") {
			t.Errorf("%q: skip=%v why=%q, want skipped as an environment/setup step", name, skip, why)
		}
	}
	// Code that READS the variable is still a code task.
	for _, name := range []string{
		"`getConfig` reads `DATABASE_URL` from the environment variables",
		"`POST /api/login` checks the `JWT_SECRET` env var",
	} {
		if skip, why := Classify(Feature{Name: name}); skip {
			t.Errorf("%q skipped as %q, want it checked", name, why)
		}
	}
}

// Bug 11: a plan written as a table lost every row without a word. Rows are
// lines; the header row and the |---| separator are not.
func TestLoadMarkdownTableRows(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "PLAN.md")
	os.WriteFile(md, []byte(strings.Join([]string{
		"# Plan",
		"",
		"| # | Task | Status |",
		"|---|------|--------|",
		"| 1 | `GET /api/items` returns the items | done |",
		"| 2 | Add `ItemForm.tsx` | todo |",
		"| 3 | `POST /api/items` saves an item | ✅ |",
		"",
		"- [x] A normal bullet",
	}, "\n")+"\n"), 0o644)
	fs, err := Load(md)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 4 {
		t.Fatalf("got %d lines, want 3 table rows + 1 bullet: %+v", len(fs), fs)
	}
	byText := map[string]Feature{}
	for _, f := range fs {
		byText[f.Name] = f
	}
	for _, f := range fs {
		if strings.Contains(f.Name, "Task") || strings.Contains(f.Name, "---") {
			t.Errorf("the header or separator row became a plan line: %q", f.Name)
		}
	}
	var items, form, post Feature
	for _, f := range fs {
		switch {
		case strings.Contains(f.Name, "GET /api/items"):
			items = f
		case strings.Contains(f.Name, "ItemForm.tsx"):
			form = f
		case strings.Contains(f.Name, "POST /api/items"):
			post = f
		}
	}
	if items.Name == "" || form.Name == "" || post.Name == "" {
		t.Fatalf("a table row is missing: %+v", fs)
	}
	if items.Unticked || post.Unticked {
		t.Errorf("done rows are due now, got unticked: %+v %+v", items, post)
	}
	if !form.Unticked {
		t.Errorf("a row whose status is todo isn't claimed as done yet: %+v", form)
	}
	if !strings.HasPrefix(items.Source, md+":") {
		t.Errorf("a row records its line: %q", items.Source)
	}
}

// A table of design values or summaries is not a list of tasks: a row that
// names no URL, file or code name is SKIPPED (with the reason), not a vague
// plan line that blocks. (Found on a real Next.js app: font and colour tables turned into
// ten REWRITE THIS LINE results the first time tables were read.)
func TestClassify_TableRowNamingNothingIsSkipped(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "PLAN.md")
	os.WriteFile(md, []byte(strings.Join([]string{
		"| Token | Light | Dark | Use |",
		"|---|---|---|---|",
		"| `--landing-bg` | `#FAFAF7` | `#14181B` | page background |",
		"| Body / UI | Geist | | paragraphs, buttons |",
		"| Settings | Real persistence | Mocked with `setTimeout` |",
		"| `GET /api/items` | returns the items | | |",
		"| Add `ItemForm.tsx` | | | todo |",
	}, "\n")+"\n"), 0o644)
	fs, err := Load(md)
	if err != nil {
		t.Fatal(err)
	}
	skipped, checked := 0, 0
	for _, f := range fs {
		skip, why := Classify(f)
		if skip {
			skipped++
			if !strings.Contains(why, "table") {
				t.Errorf("%q skipped as %q, want it to say it's a table row", f.Name, why)
			}
		} else {
			checked++
		}
	}
	if skipped != 3 || checked != 2 {
		t.Fatalf("skipped %d, checked %d, want 3 and 2: %+v", skipped, checked, fs)
	}
}

// Instruction and skill files that AI coding tools keep in their own folders
// (.agent/workflows/plan.md, .claude/..., .cursor/...) are for the tool, not the
// project's plan. They were offered as plan files and produced 100+ lines of
// REWRITE THIS LINE noise (found on a real Next.js app, 2026-10-12).
func TestDetectSkipsAIToolFolders(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	write("docs/PLAN.md", "- [ ] one\n- [ ] two\n")
	for _, d := range []string{".agent/workflows", ".agents/skills", ".claude/commands", ".cursor/rules", ".codex", ".gemini"} {
		write(d+"/plan.md", "- [ ] one\n- [ ] two\n")
		write(d+"/notes.md", "- [ ] one\n- [ ] two\n")
	}
	got, err := Detect(dir)
	if err != nil || len(got) != 1 || filepath.Base(got[0]) != "PLAN.md" {
		t.Fatalf("Detect = %v, %v; want only docs/PLAN.md", got, err)
	}
	// The content tier too: nothing here is named like a plan, the files only look like one.
	dir2 := t.TempDir()
	p := filepath.Join(dir2, ".agent", "skills")
	os.MkdirAll(p, 0o755)
	os.WriteFile(filepath.Join(p, "checklists.md"), []byte("- [ ] one\n- [ ] two\n"), 0o644)
	os.WriteFile(filepath.Join(dir2, "ROADMAP.md"), []byte("- [ ] a\n- [ ] b\n"), 0o644)
	got, err = DetectByContent(dir2)
	if err != nil || len(got) != 1 || filepath.Base(got[0]) != "ROADMAP.md" {
		t.Fatalf("DetectByContent = %v, %v; want only ROADMAP.md", got, err)
	}
}
