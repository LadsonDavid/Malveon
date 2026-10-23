package wiring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
)

// The sweep (2026-10-12): a working project per stack, and plan lines whose true
// verdict is known. Every line here once got a wrong answer from a real run;
// each is a regression test for one bug, named in the comment above it.
// "~" stands for a backtick in fixtures and lines, so they fit in Go raw strings.

func bt(s string) string { return strings.ReplaceAll(s, "~", "`") }

func sweepProject(t *testing.T, files map[string]string) func(line string) Result {
	t.Helper()
	root := t.TempDir()
	for rel, src := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(bt(src)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g, err := extractor.Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	return func(line string) Result {
		return Run(g, []features.Feature{{ID: "x", Name: bt(line)}})[0]
	}
}

type sweepCase struct {
	line string
	want Verdict
	why  string // part of the reason the answer must give; "" = don't care
}

func runSweep(t *testing.T, check func(string) Result, cases []sweepCase) {
	t.Helper()
	for _, c := range cases {
		c := c
		t.Run(c.line, func(t *testing.T) {
			r := check(c.line)
			if r.Verdict != c.want {
				t.Errorf("verdict = %s, want %s\n  reason: %s", r.Verdict, c.want, r.Reason)
			}
			if c.why != "" && !strings.Contains(r.Reason, c.why) {
				t.Errorf("reason %q should mention %q", r.Reason, c.why)
			}
		})
	}
}

// nextApp is a Next.js + Supabase app where everything the lines below say works.
var nextApp = map[string]string{
	"src/lib/supabase.ts": `export const supabase: any = {}`,
	"src/lib/useAuth.ts":  `export function useAuth() { return 'me' }`,
	"src/lib/useToast.ts": `export function useToast() { return () => {} }`,
	"src/lib/items.ts":    `export function getItems() { return fetch('/api/items') }`,
	"src/lib/notify.ts": `import { supabase } from '@/lib/supabase'
export async function createNotification(userId: string, msg: string) {
  await supabase.from('notifications').insert({ user_id: userId, msg })
}`,
	"src/hooks/useItems.ts": `import { useEffect, useState } from 'react'
export function useItems() { const [j, setJ] = useState<any[]>([]); useEffect(() => { fetch('/api/items').then(r => r.json()).then(setJ) }, []); return j }`,
	"src/components/ItemList.tsx": `import { useItems } from '../hooks/useItems'
export default function ItemList() { const items = useItems(); return <ul>{items.map((i: any) => <li key={i.id}>{i.name}</li>)}</ul> }`,
	"src/components/ItemForm.tsx": `export default function ItemForm() {
  async function add(e: any) { await fetch('/api/items', { method: 'POST', body: JSON.stringify({ name: 'x' }) }) }
  return <form onSubmit={add}><button>Add</button></form>
}`,
	"src/components/ItemRow.tsx": `export default function ItemRow({ id }: { id: string }) {
  return <button onClick={() => fetch(~/api/items/${id}~, { method: 'DELETE' })}>Delete</button>
}`,
	"src/components/Rename.tsx": `export default function Rename() { return <button onClick={() => fetch('/api/items/1', { method: 'PUT' })}>Rename</button> }`,
	"src/components/Orphan.tsx": `export default function Orphan() { return <div>banner</div> }`,
	"src/components/orders/OrderModal.tsx": `import { useAuth } from '@/lib/useAuth'
export default function OrderModal({ open }: { open: boolean }) {
  const u = useAuth()
  async function save() { await fetch('/api/orders', { method: 'POST', body: JSON.stringify({ total: 1 }) }) }
  return open ? <button onClick={save}>Save {u}</button> : null
}`,
	"src/components/orders/OrderList.tsx":    `export default function OrderList({ orders }: { orders: any[] }) { return <ul>{orders.map(o => <li key={o.id}>{o.total}</li>)}</ul> }`,
	"src/components/orders/OrderFilters.tsx": `export default function OrderFilters() { return <div>filters</div> }`,
	"src/components/orders/index.ts": `export { default as OrderModal } from './OrderModal'
export { default as OrderList } from './OrderList'`,
	"src/components/orders/OrderRow.tsx": `export default function OrderRow({ id }: { id: string }) {
  return <button onClick={() => fetch(~/api/orders/${id}~, { method: 'PATCH', body: '{}' })}>Update</button>
}`,
	"src/components/orders/RefundRow.tsx": `export default function RefundRow({ id }: { id: string }) {
  return <button onClick={() => fetch(~/api/refunds/${id}~, { method: 'PATCH', body: '{}' })}>Refund</button>
}`,
	"src/app/orders/page.tsx": `'use client'
import { useEffect, useState } from 'react'
import OrderModal from '@/components/orders/OrderModal'
import OrderList from '@/components/orders/OrderList'
export default function OrdersPage() {
  const [orders, setOrders] = useState<any[]>([]); const [open, setOpen] = useState(false)
  useEffect(() => { fetch('/api/orders').then(r => r.json()).then(setOrders) }, [])
  return <div><button onClick={() => setOpen(true)}>New Order</button><OrderList orders={orders} /><OrderModal open={open} /></div>
}`,
	"src/app/jobs/page.tsx": `'use client'
import { useState } from 'react'
import PostJobModal from '@/components/jobs/PostJobModal'
export default function JobBoardPage() {
  const [open, setOpen] = useState(false)
  return <div><button onClick={() => setOpen(true)}>+ Post a Job</button><PostJobModal isOpen={open} /></div>
}`,
	"src/components/jobs/PostJobModal.tsx": `export default function PostJobModal({ isOpen }: { isOpen: boolean }) {
  const submit = () => fetch('/api/jobs', { method: 'POST', body: '{}' })
  return isOpen ? <button onClick={submit}>Post</button> : null
}`,
	"src/app/api/jobs/route.ts": `import { supabase } from '@/lib/supabase'
export async function POST(req: Request) { await supabase.from('jobs').insert({}); return Response.json({}) }`,
	"src/app/api/items/route.ts": `import { supabase } from '@/lib/supabase'
export async function GET() { const { data } = await supabase.from('items').select('*'); return Response.json(data) }
export async function POST(req: Request) { const b = await req.json(); await supabase.from('items').insert({ name: b.name }); return Response.json(b) }`,
	"src/app/api/items/[id]/route.ts": `import { supabase } from '@/lib/supabase'
export async function DELETE(req: Request, { params }: any) { await supabase.from('items').delete().eq('id', params.id); return Response.json({}) }
export async function PATCH(req: Request, { params }: any) { await supabase.from('items').update({}).eq('id', params.id); return Response.json({}) }`,
	"src/app/api/orders/route.ts": `import { supabase } from '@/lib/supabase'
export async function GET() { const { data } = await supabase.from('orders').select('*'); return Response.json(data) }
export async function POST(req: Request) { await supabase.from('orders').insert({}); return Response.json({}) }`,
	"src/app/api/orders/[id]/route.ts": `import { supabase } from '@/lib/supabase'
import { createNotification } from '@/lib/notify'
export async function PATCH(req: Request, { params }: any) {
  await supabase.from('orders').update({}).eq('id', params.id)
  await createNotification('a', 'updated')
  return Response.json({})
}`,
	"src/app/api/refunds/[id]/route.ts": `import { supabase } from '@/lib/supabase'
export async function PATCH(req: Request, { params }: any) { await supabase.from('refunds').update({}).eq('id', params.id); return Response.json({}) }`,
	"supabase/migrations/001.sql": `create table items (id serial primary key, name text);
create table orders (id serial primary key, total int);
create table notifications (id serial primary key, user_id text, msg text);
create table refunds (id serial primary key, status text);
create table jobs (id serial primary key);`,
}

// Bug 1: "X calls URL" must be about X. These all used to read CONNECTED
// because the URL and X each exist, though X never makes the call.
func TestSweepSubjectMustMakeTheCall(t *testing.T) {
	check := sweepProject(t, nextApp)
	runSweep(t, check, []sweepCase{
		{"`Orphan` calls `GET /api/items`", HalfBuilt, "nothing connects it"},
		{"`ItemForm` calls `GET /api/items`", HalfBuilt, ""},
		{"`OrderModal` calls `DELETE /api/items/[id]`", HalfBuilt, ""},
		{"`src/components/ItemRow.tsx` loads items from `GET /api/items`", HalfBuilt, ""},
		{"`ItemForm` sends `POST /api/orders`", HalfBuilt, ""},
		{"`OrderList` loads orders from `GET /api/orders`", HalfBuilt, ""},
		// Controls: the subject really makes the call, itself or through what it imports.
		{"`ItemForm` calls `POST /api/items`", Working, ""},
		{"`src/components/ItemForm.tsx` posts a new item to `/api/items`", Working, ""},
		{"`ItemList` loads items from `GET /api/items`", Working, ""},
		{"`src/lib/items.ts` calls `GET /api/items`", Working, ""},
		{"`ItemRow` deletes an item via `DELETE /api/items/[id]`", Working, ""},
	})
}

// Bug 4: a page as the subject. "JobBoardPage ... hook into POST /api/jobs" was
// HALF BUILT ("route never uses JobBoardPage"): the check ran backwards.
func TestSweepPageAsSubject(t *testing.T) {
	check := sweepProject(t, nextApp)
	runSweep(t, check, []sweepCase{
		{"Wire the \"+ Post a Job\" button in `JobBoardPage` to the modal and hook into `POST /api/jobs`.", Working, ""},
		{"`OrdersPage` loads orders from `GET /api/orders`", Working, ""},
		{"Wire the New Order button in `OrdersPage` to `OrderModal` and `POST /api/orders`", Working, ""},
		{"`OrdersPage` renders `OrderModal`", Working, ""},
		{"`src/app/orders/page.tsx` shows `OrderModal` from the New Order button", Working, ""},
		// Not rendered by the page: still a proven gap.
		{"`OrdersPage` renders `OrderFilters`", HalfBuilt, ""},
	})
}

// Bug 2: a hook the component never uses.
func TestSweepNamedUseMustBeReal(t *testing.T) {
	check := sweepProject(t, nextApp)
	runSweep(t, check, []sweepCase{
		{"`OrderModal` uses `useToast`", HalfBuilt, ""},
		{"`useToast` is used by `OrderModal`", HalfBuilt, ""},
		{"`OrderModal` uses `useAuth`", Working, ""},
		{"`useAuth` is used by `OrderModal`", Working, ""},
	})
}

// Bug 7: two methods share a URL. A line naming PATCH while the screen only
// calls DELETE there is an uncalled route (HALF BUILT), not a broken call.
// A method the server lacks, which no screen calls yet, is not built.
func TestSweepTwoMethodsOneURL(t *testing.T) {
	check := sweepProject(t, nextApp)
	runSweep(t, check, []sweepCase{
		{"`PATCH /api/items/[id]` updates an item", HalfBuilt, "no frontend call"},
		{"Add `GET /api/orders/[id]` to fetch one order", NotBuilt, ""},
		// Controls: a screen really calls a method the server lacks.
		{"Rename an item via `PUT /api/items/[id]`", Broken, ""},
		{"`Rename` calls `PUT /api/items/[id]`", Broken, ""},
	})
}

// Bug 8: a route that writes through an imported helper.
func TestSweepTableThroughHelper(t *testing.T) {
	check := sweepProject(t, nextApp)
	runSweep(t, check, []sweepCase{
		{"`PATCH /api/orders/[id]` inserts a row into the `notifications` table", Working, ""},
		{"`PATCH /api/orders/[id]` sends `createNotification` to the buyer", Working, ""},
		{"`PATCH /api/refunds/[id]` inserts a row into the `notifications` table", HalfBuilt, ""},
		{"`PATCH /api/refunds/[id]` sends `createNotification` to the buyer", HalfBuilt, ""},
	})
}

// Bug 5: Go 1.22's net/http puts the method in the pattern: "GET /api/items".
var goHTTP = map[string]string{
	"main.go": `package main

import (
	"encoding/json"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/items", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode([]string{}) })
	mux.HandleFunc("POST /api/items", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(map[string]string{}) })
	mux.HandleFunc("DELETE /api/items/{id}", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("{}")) })
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("/legacy", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	http.ListenAndServe(":3000", mux)
}
`,
	"src/components/ItemForm.tsx": `export default function ItemForm() { return <button onClick={() => fetch('/api/items', { method: 'POST' })}>Add</button> }`,
	"src/components/ItemList.tsx": `export default function ItemList() { fetch('/api/items'); return null }`,
	"src/components/ItemRow.tsx":  `export default function ItemRow({ id }: { id: string }) { return <button onClick={() => fetch(~/api/items/${id}~, { method: 'DELETE' })}>x</button> }`,
}

func TestSweepGoMethodPatterns(t *testing.T) {
	check := sweepProject(t, goHTTP)
	runSweep(t, check, []sweepCase{
		{"`GET /api/items` returns the items", Working, ""},
		{"`POST /api/items` saves an item", Working, ""},
		{"`DELETE /api/items/[id]` removes an item", Working, ""},
		{"`GET /api/health` is the health check", Working, ""},
		// A method the server never registered is still a gap.
		{"`PUT /api/items` replaces an item", NotBuilt, ""},
	})
}

// Bug 6: axum registers several methods on one route: get(a).post(b).
var axumApp = map[string]string{
	"src/main.rs": `use axum::{routing::{get, post, delete, patch}, Router};
async fn list() -> &'static str { "[]" }
async fn add() -> &'static str { "{}" }
async fn del() -> &'static str { "{}" }
async fn upd() -> &'static str { "{}" }
fn main() {
    let _app: Router = Router::new()
        .route("/api/items", get(list).post(add))
        .route("/api/items/:id", delete(del).patch(upd));
}
`,
	"src/components/ItemForm.tsx": `export default function ItemForm() { return <button onClick={() => fetch('/api/items', { method: 'POST' })}>Add</button> }`,
	"src/components/ItemList.tsx": `export default function ItemList() { fetch('/api/items'); return null }`,
	"src/components/ItemRow.tsx":  `export default function ItemRow({ id }: { id: string }) { return <button onClick={() => fetch(~/api/items/${id}~, { method: 'DELETE' })}>x</button> }`,
}

func TestSweepAxumChainedMethods(t *testing.T) {
	check := sweepProject(t, axumApp)
	runSweep(t, check, []sweepCase{
		{"`GET /api/items` returns the items", Working, ""},
		{"`POST /api/items` saves an item", Working, ""},
		{"`DELETE /api/items/[id]` removes an item", Working, ""},
		{"`PATCH /api/items/[id]` updates an item", HalfBuilt, ""}, // registered, nothing calls it
		{"`PUT /api/items` replaces an item", NotBuilt, ""},
	})
}

// Bug 3: the Pages Router picks the method inside one handler; claims about a
// method that handler never serves must not read CONNECTED.
var pagesApp = map[string]string{
	"pages/api/items/index.ts": `export default async function handler(req: any, res: any) {
  if (req.method === 'GET') return res.json([])
  if (req.method === 'POST') return res.json(req.body)
  res.status(405).end()
}`,
	"pages/api/items/[id].ts": `export default async function handler(req: any, res: any) {
  switch (req.method) {
    case 'DELETE': return res.json({ ok: true })
    case 'PATCH': return res.json({})
    default: res.status(405).end()
  }
}`,
	"pages/api/legacy.ts":         `export default function handler(req: any, res: any) { res.json({ ok: true }) }`,
	"src/components/ItemForm.tsx": `export default function ItemForm() { return <button onClick={() => fetch('/api/items', { method: 'POST' })}>Add</button> }`,
	"src/components/ItemList.tsx": `export default function ItemList() { fetch('/api/items'); return null }`,
	"src/components/ItemRow.tsx":  `export default function ItemRow({ id }: { id: string }) { return <button onClick={() => fetch(~/api/items/${id}~, { method: 'DELETE' })}>x</button> }`,
	"src/components/Legacy.tsx":   `export default function Legacy() { fetch('/api/legacy', { method: 'POST' }); return null }`,
}

func TestSweepPagesRouterMethods(t *testing.T) {
	check := sweepProject(t, pagesApp)
	runSweep(t, check, []sweepCase{
		{"`GET /api/items` returns the items", Working, ""},
		{"`POST /api/items` saves an item", Working, ""},
		{"`DELETE /api/items/[id]` removes an item", Working, ""},
		{"`PATCH /api/items/[id]` updates an item", HalfBuilt, ""},
		{"`PUT /api/items` replaces an item", NotBuilt, ""},
		{"`POST /api/items/[id]` updates an item", NotBuilt, ""},
		// A handler that never looks at req.method serves every method: unknown, not wrong.
		{"`POST /api/legacy` saves", Working, ""},
	})
}

// Bug 10: a Windows path in a plan line (src\components\ItemForm.tsx) was not
// read as a file at all, so the line was judged by its words alone.
func TestSweepWindowsPaths(t *testing.T) {
	check := sweepProject(t, nextApp)
	runSweep(t, check, []sweepCase{
		{"`src\\components\\ItemForm.tsx` posts a new item to `/api/items`", Working, ""},
		{"`src\\components\\Missing.tsx` exists", NotBuilt, "doesn't exist"},
		{"`src\\components\\Missing.tsx` loads items", NotBuilt, "doesn't exist"},
		{"Create src\\components\\Missing.tsx for the page", NotBuilt, "doesn't exist"},
	})
}

// A barrel file that re-exports a component really exports it; one that doesn't, doesn't.
func TestSweepBarrelFile(t *testing.T) {
	check := sweepProject(t, nextApp)
	runSweep(t, check, []sweepCase{
		{"~src/components/orders/index.ts~ exports ~OrderModal~", Working, ""},
		{"~src/components/orders/index.ts~ exports ~OrderList~ and ~OrderModal~", Working, ""},
		{"~src/components/orders/index.ts~ exports ~OrderFilters~", HalfBuilt, "nothing connects"},
	})
}
