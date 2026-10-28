package wiring

import (
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// TestFixturesEndToEnd runs the real extractor and the real wiring check
// over every language fixture. Each fixture is built to hold the same
// four facts: a connected call (refund), a call to a URL nothing serves
// (cancel-order calls /cancel-order; the server only has
// /cancel-order-info), a call with the wrong method (the screen sends PUT
// /settings, the server only accepts POST), and — except in the JS
// fixture — a server route nothing calls (/profile).
func TestFixturesEndToEnd(t *testing.T) {
	other := map[string]Verdict{
		"refund":          Working,
		"cancel-order":    Broken,
		"profile":         HalfBuilt,
		"update-settings": Broken,
	}
	cases := map[string]map[string]Verdict{
		"fixture": {
			"refund-button":   Working,
			"cancel-order":    Broken,
			"profile-update":  Working, // fetch(`/profile/${userId}`) resolves to /profile/:param
			"update-settings": Broken,
		},
		"fixture-go": other, "fixture-python": other, "fixture-rust": other, "fixture-java": other,
		"fixture-ruby": other, "fixture-php": other, "fixture-dotnet": other,
	}
	for dir, want := range cases {
		t.Run(dir, func(t *testing.T) {
			g, err := extractor.Extract("../../testdata/" + dir)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			fs, err := features.Load("../../testdata/" + dir + "/features.json")
			if err != nil {
				t.Fatalf("load features: %v", err)
			}
			for _, r := range Run(g, fs) {
				if w, ok := want[r.FeatureID]; ok && r.Verdict != w {
					t.Errorf("feature %q: got %s (%s), want %s", r.FeatureID, r.Verdict, r.Reason, w)
				}
				if r.Verdict == Working && len(r.Pairs) == 0 {
					t.Errorf("feature %q: CONNECTED must rest on a real call -> route connection", r.FeatureID)
				}
			}
		})
	}
}

func verdictOf(t *testing.T, g *graph.Graph, name string) Result {
	t.Helper()
	return Run(g, []features.Feature{{ID: "x", Name: name}})[0]
}

func route(path, method, file string) graph.Node {
	return graph.Node{Kind: graph.RouteHandler, Path: path, Method: method, File: file, Line: 1,
		Confidence: graph.Extracted, Words: graph.SplitWords(path)}
}

func call(path, method, file string) graph.Node {
	return graph.Node{Kind: graph.NetworkCall, Path: path, Method: method, File: file, Line: 5,
		Confidence: graph.Extracted, Words: graph.SplitWords(path)}
}

// A single shared word is never a match — the real false PASS a tester
// saw: a line about icon strokes ("...the two sets read as one...")
// "reached" /api/notifications/read-all through the word "read".
func TestOneSharedWordIsNotEvidence(t *testing.T) {
	g := &graph.Graph{Nodes: []graph.Node{
		route("/api/notifications/read-all", "PATCH", "route.ts"),
		call("/api/notifications/read-all", "PATCH", "page.tsx"),
	}}
	r := verdictOf(t, g, "Keep the two icon sets consistent so they read as one system")
	if r.Verdict == Working {
		t.Fatalf("got CONNECTED from one coincidental word: %s", r.Reason)
	}
}

func TestNamedURL(t *testing.T) {
	g := &graph.Graph{Files: []string{"api/orphan/route.ts"}, Nodes: []graph.Node{
		route("/api/jobs", "GET", "api/jobs/route.ts"),
		route("/api/jobs", "POST", "api/jobs/route.ts"),
		call("/api/jobs", "POST", "PostJobModal.tsx"),
		route("/api/orphan", "GET", "api/orphan/route.ts"),
		call("/api/ghost", "GET", "Ghost.tsx"),
	}}
	cases := []struct {
		line string
		want Verdict
		sub  string
	}{
		{"Wire the button to `POST /api/jobs`", Working, "POST /api/jobs reaches"},
		// The server never registered DELETE and no screen sends one: not built yet.
		{"Delete jobs via `DELETE /api/jobs`", NotBuilt, "no DELETE handler"},
		// A route nothing in the app calls is HALF BUILT unless the line says
		// something outside calls it (a tester's rule, founder 2026-09-25).
		{"Orphan API (`/api/orphan`)", HalfBuilt, "say so in the line"},
		{"Call `/api/orphan` from the dashboard", HalfBuilt, "no frontend call to it was found"},
		{"The orphan button posts to `/api/orphan`", HalfBuilt, "no frontend call to it was found"},
		// The founder's real prompt and the agent's real claim: neither names an
		// outside caller ("health" in the URL doesn't count, only the line's words).
		{"add a GET `/api/orphan` route that returns `{ ok: true }`", HalfBuilt, "no frontend call to it was found"},
		{"GET `/api/orphan` is served from api/orphan/route.ts and returns { ok: true }", HalfBuilt, "no frontend call to it was found"},
		{"add a GET `/api/orphan` health check that returns `{ ok: true }`", Working, "called from outside the app"},
		{"`/api/orphan` webhook called by Stripe", Working, "called from outside the app"},
		{"The webhook settings button posts to `/api/orphan`", HalfBuilt, "no frontend call to it was found"},
		{"Ghost API (`/api/ghost`)", Broken, "no server route handles it"},
		{"Search API (`/api/search`)", NotBuilt, "nothing in the code serves or calls /api/search"},
	}
	for _, tc := range cases {
		r := verdictOf(t, g, tc.line)
		if r.Verdict != tc.want || !strings.Contains(r.Reason, tc.sub) {
			t.Errorf("%q: got %s (%s), want %s containing %q", tc.line, r.Verdict, r.Reason, tc.want, tc.sub)
		}
	}
}

// A call to /posts/${id} must not count as a call to /posts/pending.
func TestPlaceholderCallDoesNotNameFixedSegment(t *testing.T) {
	g := &graph.Graph{Nodes: []graph.Node{
		route("/api/posts/:id", "DELETE", "api/posts/[id]/route.ts"),
		call("/api/posts/:param", "DELETE", "Admin.tsx"),
	}}
	r := verdictOf(t, g, "Pending queue (`/api/posts/pending`)")
	if r.Verdict == Working {
		t.Fatalf("a DELETE /api/posts/${id} call was taken as a call to /api/posts/pending: %s", r.Reason)
	}
}

func TestFilesAndRemoval(t *testing.T) {
	g := &graph.Graph{Files: []string{"src/components/Footer.tsx"}}
	if r := verdictOf(t, g, "Create `Header.tsx` for the top bar"); r.Verdict != NotBuilt {
		t.Errorf("missing file: got %s (%s)", r.Verdict, r.Reason)
	}
	if r := verdictOf(t, g, "`docs/OLD.md` — delete (stale)"); r.Verdict != Working {
		t.Errorf("removed file: got %s (%s), want CONNECTED (it's gone, as planned)", r.Verdict, r.Reason)
	}
	if r := verdictOf(t, g, "Remove `src/components/Footer.tsx`"); r.Verdict != NotBuilt {
		t.Errorf("file still there: got %s (%s), want NOT BUILT", r.Verdict, r.Reason)
	}
}

// A table the line names must be used by the component the line names
// (or the routes it calls) — not by something unrelated elsewhere. The
// real false CONNECTED: `NotificationsTab` "used" `profiles` because a
// chat page did.
func TestTableMustBeUsedByTheNamedComponent(t *testing.T) {
	g := &graph.Graph{
		Files: []string{"NotificationsTab.tsx", "chat/page.tsx"},
		Nodes: []graph.Node{
			{Kind: graph.DataCall, Method: "TABLE", Path: "profiles", File: "chat/page.tsx", Line: 3, Confidence: graph.Extracted},
		},
	}
	r := verdictOf(t, g, "`NotificationsTab`: Save preferences to `profiles`")
	if r.Verdict != HalfBuilt {
		t.Fatalf("got %s (%s), want HALF BUILT — the tab itself never touches profiles", r.Verdict, r.Reason)
	}

	g.Nodes = append(g.Nodes, graph.Node{Kind: graph.DataCall, Method: "TABLE", Path: "profiles", File: "NotificationsTab.tsx", Line: 9, Confidence: graph.Extracted})
	if r := verdictOf(t, g, "`NotificationsTab`: Save preferences to `profiles`"); r.Verdict != Working {
		t.Fatalf("got %s (%s), want CONNECTED once the tab writes profiles itself", r.Verdict, r.Reason)
	}
}

func TestCodeNames(t *testing.T) {
	g := &graph.Graph{
		Files: []string{"CircleTab.tsx"},
		Defs:  map[string][]graph.Loc{"sendEmail": {{File: "mail.ts", Line: 10, End: 20}}},
		Nodes: []graph.Node{
			{Kind: graph.DataCall, Method: "TABLE", Path: "outbox", File: "mail.ts", Line: 30, Confidence: graph.Extracted}, // outside sendEmail
		},
	}
	if r := verdictOf(t, g, "Wire `CircleTab` to real circle data"); r.Verdict != Skipped {
		t.Errorf("component with no calls on a 'wire it' line: got %s (%s), want HALF BUILT", r.Verdict, r.Reason)
	}
	if r := verdictOf(t, g, "Wire `sendEmail()` to the mail API"); r.Verdict != Skipped {
		t.Errorf("a call outside the function's own lines must not count: got %s (%s)", r.Verdict, r.Reason)
	}
	if r := verdictOf(t, g, "Add `ProfileHeader` to the page"); r.Verdict != NotBuilt {
		t.Errorf("unknown component: got %s (%s), want NOT BUILT", r.Verdict, r.Reason)
	}
}

// An ALL-CAPS label in backticks ("`INPUT`: ...") is a label, not a code
// name: "`INPUT` isn't anywhere in the code" was a false fact about a word
// an agent wrote to label a bullet (found 2026-09-30 on an agent-written
// plan). An all-caps word elsewhere (a status value) still counts.
func TestAllCapsLabelIsNotACodeName(t *testing.T) {
	g := &graph.Graph{Files: []string{"src/components/PostForm.tsx"}, Nodes: []graph.Node{route("/api/x", "GET", "r.ts")}}

	// Task metadata bullets are skipped outright, not checked.
	for _, line := range []string{
		"**Task ID**: `TASK-01`",
		"`INPUT`: Request payload `{ postId: string }`",
		"`OUTPUT`: the new row",
	} {
		if r := verdictOf(t, g, line); r.Verdict != Skipped {
			t.Errorf("%q: got %s (%s), want SKIPPED", line, r.Verdict, r.Reason)
		}
	}

	// A label line that names a real file is still checked — and the label
	// must not add a NOT BUILT of its own.
	r := verdictOf(t, g, "`INPUT`: `PostForm.tsx` sends the post id")
	if r.Verdict == NotBuilt || strings.Contains(r.Reason, "INPUT") || !strings.Contains(r.Reason, "PostForm") {
		t.Errorf("label + file: got %s (%s), want a verdict about PostForm.tsx only", r.Verdict, r.Reason)
	}
	// A label malveon doesn't know as metadata is dropped by anchorsIn alone.
	for _, a := range anchorsIn("`CONTEXT`: members see a banner") {
		t.Errorf("`CONTEXT` label became an anchor: %+v", a)
	}

	// A real all-caps constant that's absent is a true NOT BUILT.
	for _, line := range []string{"Set status to `WITHDRAWN`", "`WITHDRAWN` — the state after a member leaves"} {
		if r := verdictOf(t, g, line); r.Verdict != NotBuilt || !strings.Contains(r.Reason, "`WITHDRAWN` isn't anywhere in the code") {
			t.Errorf("%q: got %s (%s), want NOT BUILT naming WITHDRAWN", line, r.Verdict, r.Reason)
		}
	}
}

func TestSkippedAndUnchecked(t *testing.T) {
	g := &graph.Graph{Nodes: []graph.Node{route("/api/x", "GET", "r.ts")}}
	if r := verdictOf(t, g, "**Job Board (`/jobs`)**:"); r.Verdict != Skipped {
		t.Errorf("heading: got %s", r.Verdict)
	}
	if r := verdictOf(t, g, "Zero 'Coming Soon' placeholder pages remain in the Admin Hub."); r.Verdict != Skipped || !r.Unchecked {
		t.Errorf("vague line: got %s unchecked=%v (%s), want SKIPPED and unchecked", r.Verdict, r.Unchecked, r.Reason)
	}
}

func TestDecisiveAnchors(t *testing.T) {
	yes := []string{"Added `POST /api/refund`", "Created `src/refund.js`", "`fmtPrice` is used", "`profiles.notification_settings`", "calls GET /api/items", "the `Order_List` component"}
	no := []string{"Added the `lodash` dependency", "reads the `profiles` table", "nothing named here", "uses `vitest`"}
	for _, s := range yes {
		if !Decisive(s) {
			t.Errorf("%q names something that can be called missing", s)
		}
	}
	for _, s := range no {
		if Decisive(s) {
			t.Errorf("%q rests on a plain word: its absence proves nothing", s)
		}
	}
}

func TestAnchorsOfListsWhatALineNames(t *testing.T) {
	a := AnchorsOf("Added `POST /api/refund`, `src/refund.js` and `fmtPrice`, and read `lodash`")
	if len(a.URLs) != 1 || a.URLs[0].Path != "/api/refund" || a.URLs[0].Method != "POST" {
		t.Errorf("urls = %+v", a.URLs)
	}
	if len(a.Files) != 1 || a.Files[0] != "src/refund.js" {
		t.Errorf("files = %v", a.Files)
	}
	if len(a.Names) != 2 {
		t.Errorf("names = %v", a.Names)
	}
}

// A component that exists but makes no call is HALF BUILT only because the line promised
// a connection. The result says so, so a caller that reads prose (an agent's notes) and
// not a plan can tell that kind of HALF BUILT from "a route nothing calls".
func TestResultSaysWhenAThingExistsButMakesNoCall(t *testing.T) {
	g := &graph.Graph{Files: []string{"CircleTab.tsx"}, Nodes: []graph.Node{route("/api/orphan", "GET", "r.ts"), route("/api/x", "GET", "r.ts"), call("/api/x", "GET", "app.js")}}
	r := verdictOf(t, g, "Wire `CircleTab` to real circle data")
	if r.Verdict != Skipped || !r.ExistsButNoCall {
		t.Errorf("%s ExistsButNoCall=%v (%s)", r.Verdict, r.ExistsButNoCall, r.Reason)
	}
	r = verdictOf(t, g, "Call `/api/orphan` from the dashboard")
	if r.Verdict != HalfBuilt || r.ExistsButNoCall {
		t.Errorf("a route nothing calls is another kind of half built: %s ExistsButNoCall=%v", r.Verdict, r.ExistsButNoCall)
	}
}
