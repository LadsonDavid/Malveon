package wiring

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
)

// Found 2026-09-30 on a real Next.js + Supabase app: "Members can bookmark a
// post" was REWRITE THIS LINE although GET/POST /api/bookmarks and a component
// calling it exist — the route's word "bookmarks" never equalled the line's
// "bookmark". Plurals now compare equal; the strength rules stay as they were.
func TestSingularLineMatchesPluralRoute(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"src/components/BookmarkPostButton.tsx":       "export function BookmarkPostButton() {\n  const save = () => fetch('/api/bookmarks', { method: 'POST' })\n  return null\n}\n",
		"src/app/api/bookmarks/route.ts":              "export async function POST(request: Request) { return Response.json({ ok: true }) }\n",
		"src/app/api/bookmark-folders/route.ts":       "export async function GET() { return Response.json([]) }\n",
		"src/components/NotifTab.tsx":                 "export function NotifTab() {\n  const all = () => fetch('/api/notifications/read-all', { method: 'POST' })\n  return null\n}\n",
		"src/app/api/notifications/read-all/route.ts": "export async function POST() { return Response.json({ ok: true }) }\n",
	}
	// A real app has enough /api/* routes that "api" is a common word and
	// doesn't count; give the fixture that many so it behaves the same.
	for i := 0; i < 20; i++ {
		files[fmt.Sprintf("src/app/api/%s/route.ts", strings.Repeat("x", i+1))] =
			"export async function GET() { return Response.json([]) }\n"
	}
	for rel, src := range files {
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
	check := func(line string) Result { return Run(g, []features.Feature{{ID: "x", Name: line}})[0] }

	r := check("Members can bookmark a post")
	if r.Verdict != Working {
		t.Errorf("singular line should reach the plural /api/bookmarks route\n  got %s: %s", r.Verdict, r.Reason)
	}
	if len(r.Pairs) == 0 || r.Pairs[0].Route.Path != "/api/bookmarks" {
		t.Errorf("want the /api/bookmarks connection, got %+v", r.Pairs)
	}

	// The strength rules are unchanged: every URL word must still be in the
	// line, and coincidences with one shared word still never match.
	for _, line := range []string{
		"the two sets read as one",           // "read" alone must not reach /api/notifications/read-all
		"Members can bookmark a post folder", // must not be treated as an exact plural match of anything new
	} {
		r := check(line)
		if r.Verdict == Working && len(r.Pairs) > 0 && r.Pairs[0].Route.Path == "/api/notifications/read-all" {
			t.Errorf("%q must not match /api/notifications/read-all: %s", line, r.Reason)
		}
	}
	if r := check("the two sets read as one"); r.Verdict == Working {
		t.Errorf("coincidence must not be CONNECTED: %s", r.Reason)
	}
}
