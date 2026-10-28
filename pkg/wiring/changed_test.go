package wiring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// A plan line that names only an existing file and says it will gain
// something ("`PostMenu.tsx` adds a report option") used to be CONNECTED on
// the file existing — and, for a page, on unrelated routes it calls. With the
// files changed since the session started, malveon can state the fact it
// does have: the file hasn't changed.

const (
	menuFile = "src/components/PostMenu.tsx"
	pageFile = "src/app/feed/[id]/page.tsx"
	tabFile  = "src/components/ReportsTab.tsx"
)

func changedGraph(t *testing.T) *graph.Graph {
	t.Helper()
	root := t.TempDir()
	for rel, src := range map[string]string{
		pageFile: `import PostMenu from '@/components/PostMenu'
import ReportsTab from '@/components/ReportsTab'
export default function Page() {
  fetch('/api/feed')
  return <div><PostMenu /><ReportsTab /></div>
}`,
		menuFile: `export default function PostMenu() { return <div>post</div> }`,
		tabFile:  `export default function ReportsTab() { return <div>reports</div> }`,
		"src/app/api/feed/route.ts": `export async function GET() {
  const { data } = await supabase.from('notifications').insert({})
  return Response.json(data)
}`,
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g, err := extractor.Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func runLine(g *graph.Graph, line string, changed map[string]bool) Result {
	return RunWithChanges(g, []features.Feature{{ID: "x", Name: line}}, changed)[0]
}

func TestUnchangedFileCannotHaveGainedWhatTheLineSays(t *testing.T) {
	g := changedGraph(t)
	none := map[string]bool{} // git worked; nothing changed since the session started

	for _, c := range []struct{ name, line string }{
		{"a component", "`" + menuFile + "` adds a report option to the post menu for non-authors"},
		{"a page that calls unrelated routes", "`" + pageFile + "` adds the report button to the post detail view"},
		{"other change verbs", "`" + menuFile + "` renders a report badge"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := runLine(g, c.line, none)
			if r.Verdict != Skipped || !r.Unchecked {
				t.Fatalf("verdict %s unchecked=%v (%s), want SKIPPED and unchecked — the file hasn't changed since the session started", r.Verdict, r.Unchecked, r.Reason)
			}
			for _, want := range []string{"exists", "hasn't changed since this session started", "says it"} {
				if !strings.Contains(r.Reason, want) {
					t.Errorf("reason %q missing %q", r.Reason, want)
				}
			}
			if len(r.Path) != 0 {
				t.Errorf("a NOT BUILT line shows no flow, got %v", r.Path)
			}
		})
	}
}

func TestChangedFileIsConnectedButOnlyTheChangeIsSeen(t *testing.T) {
	g := changedGraph(t)

	card := runLine(g, "`"+menuFile+"` adds a report option to the post menu", map[string]bool{menuFile: true})
	if card.Verdict != Skipped {
		t.Fatalf("verdict %s (%s), want CONNECTED", card.Verdict, card.Reason)
	}
	for _, want := range []string{"changed since this session started", "not that", "in backticks"} {
		if !strings.Contains(card.Reason, want) {
			t.Errorf("reason %q missing %q", card.Reason, want)
		}
	}

	// The page calls /api/feed, whose route inserts into notifications. That
	// flow has nothing to do with "adds the report button": it isn't shown as
	// the line's path, and the contract check doesn't get it as a connection.
	page := runLine(g, "`"+pageFile+"` adds the report button to the post detail view", map[string]bool{pageFile: true})
	if page.Verdict != Skipped {
		t.Fatalf("verdict %s (%s), want CONNECTED", page.Verdict, page.Reason)
	}
	if len(page.Path) != 0 || len(page.Pairs) != 0 {
		t.Errorf("unrelated calls must not be shown as this line's flow: path %v, %d pairs", page.Path, len(page.Pairs))
	}
	if !strings.Contains(page.Reason, "not that") {
		t.Errorf("reason %q should say it sees the change, not the addition", page.Reason)
	}
}

// One of several files unchanged: the line is about all of them.
func TestOneUnchangedFileAmongSeveralIsNamed(t *testing.T) {
	g := changedGraph(t)
	r := runLine(g, "`"+menuFile+"` and `"+tabFile+"` add a report entry", map[string]bool{menuFile: true})
	if r.Verdict != Skipped || !r.Unchecked || !strings.Contains(r.Reason, "ReportsTab.tsx") || strings.Contains(r.Reason, "PostMenu.tsx` exists, but it hasn't") {
		t.Fatalf("got %s (%s), want SKIPPED (unchecked) naming only ReportsTab.tsx", r.Verdict, r.Reason)
	}
}

// Lines this must not touch.
func TestChangedSetLeavesOtherLinesAlone(t *testing.T) {
	g := changedGraph(t)
	none := map[string]bool{}

	for _, c := range []struct {
		name, line string
		want       Verdict
		reasonHas  string
	}{
		// Creating a file: existence is the claim, whatever else changed.
		{"a create line", "Create `" + tabFile + "` for the moderation screen", Skipped, "imported by"},
		// The file is the object of "add", before any file mention: it's a new
		// file being added, not a file that gains something.
		{"an add-the-file line", "Add `" + tabFile + "` for the moderation screen", Skipped, "imported by"},
		// A URL in the line: the URL lookup decides, as always.
		{"a line naming a URL", "`" + pageFile + "` adds a call to `GET /api/feed`", Working, "reaches its server route"},
		// ...and a file that makes no such call is not connected to it, whatever changed.
		{"a file that never calls the URL", "`" + menuFile + "` adds a call to `GET /api/feed`", HalfBuilt, "nothing connects it"},
		// A file that isn't there is missing whether or not anything changed.
		{"a missing file", "`src/components/Missing.tsx` adds a report badge", NotBuilt, "doesn't exist"},
		// A removal line checks the file is gone.
		{"a removal line", "Delete `" + menuFile + "`", NotBuilt, "still there"},
		// No promise of change: only that the thing exists.
		{"no change verb", "`" + menuFile + "` is the post menu", Skipped, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := runLine(g, c.line, none)
			if r.Verdict != c.want || !strings.Contains(r.Reason, c.reasonHas) {
				t.Fatalf("got %s (%s), want %s containing %q", r.Verdict, r.Reason, c.want, c.reasonHas)
			}
			if strings.Contains(r.Reason, "since this session started") {
				t.Errorf("reason %q talks about the session for a line this doesn't apply to", r.Reason)
			}
		})
	}
}

// No session, not a git repo, or git failing: malveon knows nothing about
// what changed, so it never invents "unchanged" — behavior is exactly today's.
func TestNoChangedSetBehavesAsBefore(t *testing.T) {
	g := changedGraph(t)
	for _, line := range []string{
		"`" + menuFile + "` adds a report option to the post menu for non-authors",
		"`" + pageFile + "` adds the report button to the post detail view",
	} {
		old := Run(g, []features.Feature{{ID: "x", Name: line}})[0]
		got := runLine(g, line, nil)
		if got.Verdict != old.Verdict || got.Reason != old.Reason || strings.Join(got.Path, "|") != strings.Join(old.Path, "|") {
			t.Errorf("%q: Run gave %s (%s), RunWithChanges(nil) gave %s (%s)", line, old.Verdict, old.Reason, got.Verdict, got.Reason)
		}
	}
}

// Bug 12 (sweep 2026-10-12): an unchanged file is not proof a plan line is
// unbuilt — the plan may describe work finished before the session, and
// malveon can't tell which. So a plan-document line says so (SKIPPED, unchecked:
// name the new thing) instead of claiming NOT BUILT. A prompt is typed
// during the session by definition, so an untouched file there really is not done.
func TestUnchangedFileIsOnlyNotBuiltForPrompts(t *testing.T) {
	g := changedGraph(t)
	line := "`" + menuFile + "` adds a report option to the post menu for non-authors"
	none := map[string]bool{}

	plan := RunWithChanges(g, []features.Feature{{ID: "x", Name: line}}, none)[0]
	if plan.Verdict != Skipped || !plan.Unchecked {
		t.Fatalf("plan line: %s (%s), want SKIPPED and unchecked", plan.Verdict, plan.Reason)
	}
	for _, want := range []string{"hasn't changed since this session started", "can't tell", "in backticks"} {
		if !strings.Contains(plan.Reason, want) {
			t.Errorf("reason %q should say %q", plan.Reason, want)
		}
	}

	prompt := RunWithChanges(g, []features.Feature{{ID: "p", Name: line, InSession: true}}, none)[0]
	if prompt.Verdict != NotBuilt {
		t.Fatalf("prompt line: %s (%s), want NOT BUILT", prompt.Verdict, prompt.Reason)
	}
}
