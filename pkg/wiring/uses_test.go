package wiring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
)

// A line naming a place and a function ("PATCH /api/x sends `createNotification`")
// says the place uses the function. Found 2026-09-30 on a real Next.js app: the route
// only updated a table, the helper existed elsewhere, and the line read CONNECTED.
var usesProject = map[string]string{
	"src/lib/db.ts": `export async function createNotification(userId: string, text: string) {
  return supabase.from('notifications').insert({ user_id: userId, text })
}`,
	"src/lib/helpers.ts": `import { createNotification } from '@/lib/db'
export async function notifyReporter(id: string) { await createNotification(id, 'done') }`,
	"src/app/admin/page.tsx": `import BanPanel from '@/components/BanPanel'
import NotePanel from '@/components/NotePanel'
export default function Page() {
  const r = (id: string) => fetch(` + "`/api/admin/reports/${id}`" + `, { method: 'PATCH' })
  const a = (id: string) => fetch(` + "`/api/admin/appeals/${id}`" + `, { method: 'PATCH' })
  const m = (id: string) => fetch(` + "`/api/admin/mutes/${id}`" + `, { method: 'PATCH' })
  const g = (id: string) => fetch(` + "`/api/admin/ghosts/${id}`" + `, { method: 'PATCH' })
  return <><BanPanel /><NotePanel /></>
}`,
	"src/components/BanPanel.tsx": `export default function BanPanel() {
  const ban = (id: string) => fetch(` + "`/api/admin/bans/${id}`" + `, { method: 'PATCH' })
  return null
}`,
	"src/components/NotePanel.tsx": `export default function NotePanel() {
  const note = (id: string) => fetch(` + "`/api/admin/notes/${id}`" + `, { method: 'PATCH' })
  return null
}`,
	// Updates a table and never mentions the helper.
	"src/app/api/admin/reports/[id]/route.ts": `export async function PATCH(request: Request) {
  await supabase.from('post_reports').update({ status: 'DISMISSED' })
  return Response.json({ success: true })
}`,
	"src/app/api/admin/notes/[id]/route.ts": `export async function PATCH(request: Request) {
  await supabase.from('notes').update({ body: 'x' })
  return Response.json({ success: true })
}`,
	// Calls the helper itself.
	"src/app/api/admin/appeals/[id]/route.ts": `import { createNotification } from '@/lib/db'
export async function PATCH(request: Request) {
  await supabase.from('appeals').update({ status: 'OK' })
  await createNotification('u', 'your appeal was reviewed')
  return Response.json({ success: true })
}`,
	// Calls it through a file it imports.
	"src/app/api/admin/bans/[id]/route.ts": `import { notifyReporter } from '@/lib/helpers'
export async function PATCH(request: Request) {
  await notifyReporter('x')
  return Response.json({ success: true })
}`,
	// Loads it with a dynamic import.
	"src/app/api/admin/mutes/[id]/route.ts": `export async function PATCH(request: Request) {
  const { createNotification } = await import('@/lib/db')
  await createNotification('u', 'muted')
  return Response.json({ success: true })
}`,
	// Imports code malveon can't find: the route may use it there.
	"src/app/api/admin/ghosts/[id]/route.ts": `import { haunt } from '@/lib/haunting'
export async function PATCH(request: Request) {
  await haunt()
  return Response.json({ success: true })
}`,
}

func usesGraph(t *testing.T) func(line string) Result {
	t.Helper()
	root := t.TempDir()
	for rel, src := range usesProject {
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

func TestLineNamingAPlaceAndAFunctionNeedsThePlaceToUseIt(t *testing.T) {
	check := usesGraph(t)
	cases := []struct {
		name, line string
		want       Verdict
		reasonHas  string
	}{
		{"route never mentions the helper", "`PATCH /api/admin/reports/[id]` sends `createNotification` to the reporter", HalfBuilt,
			"`createNotification` exists (src"},
		{"route calls the helper itself", "`PATCH /api/admin/appeals/[id]` sends `createNotification` to the reporter", Working,
			"uses `createNotification`"},
		{"route calls it through an imported file", "`PATCH /api/admin/bans/[id]` sends `createNotification` to the reporter", Working,
			"helpers.ts"},
		{"route loads it with a dynamic import", "`PATCH /api/admin/mutes/[id]` sends `createNotification` to the reporter", Working,
			"uses `createNotification`"},
		{"route imports code malveon can't find", "`PATCH /api/admin/ghosts/[id]` sends `createNotification` to the reporter", Working,
			"can't follow"},
		{"component calls a route that uses it", "`BanPanel.tsx` sends `createNotification` to the reporter", Working, ""},
		{"component calls a route that doesn't", "`NotePanel.tsx` sends `createNotification` to the reporter", HalfBuilt, "never uses it"},
		// The name is the whole subject, or its own file is the place: nothing to check.
		{"only the helper is named", "Add the `createNotification` helper", Working, ""},
		{"the place is the helper's own file", "`src/lib/db.ts` exports `createNotification`", Working, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := check(c.line)
			if r.Verdict != c.want {
				t.Fatalf("verdict %s, want %s: %s", r.Verdict, c.want, r.Reason)
			}
			if !strings.Contains(r.Reason, c.reasonHas) {
				t.Errorf("reason %q missing %q", r.Reason, c.reasonHas)
			}
			if c.want == Working && strings.Contains(r.Reason, "never uses") {
				t.Errorf("a line kept CONNECTED still says it never uses the name: %q", r.Reason)
			}
		})
	}
}

// The HALF BUILT reason names the function, where it is, and the place that doesn't use it.
func TestUnusedNameReasonSaysWhoNeverUsesIt(t *testing.T) {
	r := usesGraph(t)("`PATCH /api/admin/reports/[id]` sends `createNotification` to the reporter")
	if r.Verdict != HalfBuilt {
		t.Fatalf("verdict %s: %s", r.Verdict, r.Reason)
	}
	for _, want := range []string{"`createNotification` exists", "db.ts:1", "route /api/admin/reports/:id never uses it"} {
		if !strings.Contains(r.Reason, want) {
			t.Errorf("reason %q missing %q", r.Reason, want)
		}
	}
}
