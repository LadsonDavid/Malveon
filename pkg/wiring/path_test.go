package wiring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
)

// flowProject is a small Next.js + Supabase app, one route per rule the
// end-to-end path has to get right.
var flowProject = map[string]string{
	"src/app/jobs/page.tsx": `import PostJobModal from '@/components/PostJobModal'
export default function Page() { return <PostJobModal /> }`,
	"src/components/PostJobModal.tsx": `export default function PostJobModal() {
  const save = () => fetch('/api/jobs', { method: 'POST' })
  const draft = () => fetch('/api/drafts', { method: 'POST' })
  const note = () => fetch('/api/notes', { method: 'POST' })
  const mail = () => fetch('/api/mail', { method: 'POST' })
  const ghost = () => fetch('/api/ghost', { method: 'POST' })
  const out = () => fetch('/api/sessions', { method: 'DELETE' })
  const item = () => fetch('/api/items', { method: 'POST' })
  return null
}`,
	"src/components/Orphan.tsx": `export function Orphan() { fetch('/api/orphan', { method: 'POST' }) }`,
	"src/app/api/jobs/route.ts": `export async function POST(request: Request) {
  const { data } = await supabase
    .from('jobs')
    .insert({ title: 'x' })
  return Response.json(data)
}`,
	"src/app/api/drafts/route.ts": `export async function POST(request: Request) {
  const { data } = await supabase.from('drafts').select('*')
  return Response.json(data)
}`,
	"src/app/api/notes/route.ts": `import { createNote } from '@/lib/notes'
export async function POST(request: Request) {
  return Response.json(await createNote())
}`,
	"src/lib/notes.ts": `export async function createNote() {
  return supabase.from('notes').insert({ body: 'x' })
}`,
	"src/app/api/mail/route.ts": `import { Resend } from 'resend'
export async function POST(request: Request) {
  const { data } = await supabase.from('profiles').select('email')
  await new Resend('k').emails.send({})
  return Response.json({ ok: true })
}`,
	"src/app/api/ghost/route.ts": `import { saveGhost } from '@/lib/ghost'
export async function POST(request: Request) {
  return Response.json(await saveGhost())
}`,
	"src/app/api/sessions/route.ts": `export async function DELETE() {
  const { data } = await supabase.auth.getUser()
  await supabase.auth.signOut({ scope: 'others' })
  return Response.json({ ok: true })
}`,
	"src/app/api/items/route.ts": `export async function GET() {
  await supabase.from('items').insert({})
  return Response.json({})
}
export async function POST(request: Request) {
  const { data } = await supabase.from('items').select('*')
  return Response.json(data)
}`,
	"src/app/api/orphan/route.ts": `export async function POST(request: Request) {
  await supabase.from('orphans').insert({})
  return Response.json({})
}`,
}

func flowGraph(t *testing.T) func(line string) Result {
	t.Helper()
	root := t.TempDir()
	for rel, src := range flowProject {
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
	return func(line string) Result {
		return Run(g, []features.Feature{{ID: "x", Name: line}})[0]
	}
}

func TestFlowPath(t *testing.T) {
	check := flowGraph(t)

	cases := []struct {
		line      string
		want      Verdict
		pathHas   []string
		reasonHas string
	}{
		// Screen → Action → Server → State, all four hops found.
		{"Save a job via POST /api/jobs", Working,
			[]string{"page /jobs", "POST /api/jobs (", "route POST /api/jobs", "inserts into table `jobs`"}, ""},
		// The route only reads, and the line expects a save: proven HALF BUILT.
		{"Save a draft via POST /api/drafts", HalfBuilt,
			[]string{"changes no data: only reads table `drafts`"}, "never changes any data"},
		// Same route, but the line doesn't expect a change.
		{"Search drafts via POST /api/drafts", Working, []string{"only reads table `drafts`"}, ""},
		// The write is in a helper the route imports.
		{"Save a note via POST /api/notes", Working, []string{"inserts into table `notes`"}, ""},
		// A service malveon doesn't read: never proven unchanged.
		{"Submit the form via POST /api/mail", Working, []string{"uses `Resend`"}, ""},
		// Code malveon couldn't find: never proven unchanged.
		{"Save a ghost via POST /api/ghost", Working, []string{"couldn't find (@/lib/ghost)"}, ""},
		// signOut ends sessions: a real change.
		{"Sign out other devices via DELETE /api/sessions", Working, []string{"changes auth (`auth.signOut`)"}, ""},
		// GET's insert in the same file never counts for POST.
		{"Save an item via POST /api/items", HalfBuilt, []string{"only reads table `items`"}, ""},
		// No page imports the component: shown, not blocking.
		{"Save an orphan via POST /api/orphan", Working, []string{"no page found that shows Orphan.tsx"}, ""},
	}
	for _, c := range cases {
		t.Run(c.line, func(t *testing.T) {
			r := check(c.line)
			path := strings.Join(r.Path, " → ")
			if r.Verdict != c.want {
				t.Fatalf("verdict %s, want %s (%s) path: %s", r.Verdict, c.want, r.Reason, path)
			}
			for _, want := range c.pathHas {
				if !strings.Contains(path, want) {
					t.Errorf("path %q missing %q", path, want)
				}
			}
			if !strings.Contains(r.Reason, c.reasonHas) {
				t.Errorf("reason %q missing %q", r.Reason, c.reasonHas)
			}
		})
	}
}

// Without any pages malveon can read, "no page shows it" would be an
// unchecked claim: the Screen hop says it wasn't traced instead.
func TestScreenNotTracedWithoutPages(t *testing.T) {
	g, err := extractor.Extract("../../testdata/fixture")
	if err != nil {
		t.Fatal(err)
	}
	r := Run(g, []features.Feature{{ID: "x", Name: "refund button"}})[0]
	if r.Verdict != Working || len(r.Path) == 0 || !strings.HasPrefix(r.Path[0], "page not traced") {
		t.Fatalf("verdict %s, path %v — want a CONNECTED line whose Screen hop says it wasn't traced", r.Verdict, r.Path)
	}
}

// "Files exist" is not a feature: a line whose only fact is a file needs
// something in the app to use it — or says plainly it couldn't tell.
func TestFileExistingIsNotAFeature(t *testing.T) {
	root := t.TempDir()
	for rel, src := range map[string]string{
		"src/app/settings/page.tsx": `import Used from '@/components/Used'
export default function P() { return <Used /> }`,
		"src/components/Used.tsx":   `export default function Used() { return null }`,
		"src/components/Unused.tsx": `export default function Unused() { return null }`,
		"scripts/helpers.py":        `def helper(): pass`,
		"vite.config.ts":            `export default {}`,
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(src), 0o644)
	}
	g, err := extractor.Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		line, reasonHas string
		want            Verdict
	}{
		{"Create `Unused.tsx` for the settings screen", "nothing in the app uses it: no file imports Unused.tsx", HalfBuilt},
		{"Create `Used.tsx` for the settings screen", "imported by", Working},
		{"Add `helpers.py` for the build", "isn't traced", Working},
		{"Add `vite.config.ts`", "may be started by a command or a tool", Working},
	} {
		r := Run(g, []features.Feature{{ID: "x", Name: c.line}})[0]
		if r.Verdict != c.want || !strings.Contains(r.Reason, c.reasonHas) {
			t.Errorf("%q: got %s (%s), want %s with %q", c.line, r.Verdict, r.Reason, c.want, c.reasonHas)
		}
	}
}
