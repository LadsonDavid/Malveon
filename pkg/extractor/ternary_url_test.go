package extractor

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

func callsIn(t *testing.T, files map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	g, err := Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, n := range g.Nodes {
		if n.Kind == graph.NetworkCall && n.Path != "" {
			out = append(out, n.Method+" "+n.Path)
		}
	}
	sort.Strings(out)
	return out
}

// 5PM Club: the URL is chosen by a ternary of two literals, kept in a variable, and sometimes
// followed by a query string. Both literals are real calls; reading neither made the routes look
// like nobody calls them (HALF BUILT) and made the calls "unreadable".
func TestAUrlChosenByATernaryIsBothCalls(t *testing.T) {
	got := callsIn(t, map[string]string{
		"app/logs/page.tsx":        "export default function L({ selectedDomain }) {\n  async function load() {\n    const url = selectedDomain\n      ? `/api/weekly-logs?domain=${selectedDomain}`\n      : \"/api/weekly-logs\";\n    const res = await fetch(url);\n  }\n  return null\n}\n",
		"app/submissions/page.tsx": "export default function S({ showMine }) {\n  async function load() {\n    const endpoint = showMine ? \"/api/submissions/mine\" : \"/api/submissions\";\n    const params = new URLSearchParams();\n    const res = await fetch(`${endpoint}?${params.toString()}`);\n  }\n  return null\n}\n",
	})
	want := map[string]bool{"GET /api/weekly-logs": true, "GET /api/submissions/mine": true, "GET /api/submissions": true}
	seen := map[string]bool{}
	for _, c := range got {
		seen[c] = true
	}
	for w := range want {
		if !seen[w] {
			t.Errorf("missing call %q, got %v", w, got)
		}
	}
}

// A call to another company's API (Cloudinary, Stripe) is not a call to this project's server, so it
// is not one that "may reach a route nobody else calls".
func TestACallToAnotherHostIsNotAnUnreadableCall(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "up.tsx"), []byte("export async function up(sign) {\n  const r = await fetch(`https://api.cloudinary.com/v1_1/${sign.cloud_name}/image/upload`, { method: 'POST' })\n  const q = await fetch(buildUrl())\n}\n"), 0o644)
	g, err := Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	un := g.UnreadableCalls()
	if len(un) != 1 || un[0].Line != 3 {
		t.Fatalf("only the call built by buildUrl() should be unreadable, got %+v", un)
	}
}

// vibeworks: the client helper is `api<T = unknown>(url, opts)` and it is called as
// `api<TeamList>(`/api/teams/${id}`)`. Generics between the name and the parenthesis made both the
// helper and its 20 callers invisible.
func TestAGenericRequestHelperIsFollowed(t *testing.T) {
	got := callsIn(t, map[string]string{
		"lib/api.ts":  "export async function api<T = unknown>(url: string, opts: { method?: string } = {}): Promise<T> {\n  const res = await fetch(url, { method: opts.method })\n  return res.json()\n}\n",
		"app/Hub.tsx": "import { api } from '../lib/api'\nexport function Hub({ id }) {\n  api<TeamList>(`/api/teams/${id}/workflows`)\n  api<Notices>('/api/notifications/center', { method: 'PUT' })\n  return null\n}\n",
	})
	seen := map[string]bool{}
	for _, c := range got {
		seen[c] = true
	}
	if !seen["PUT /api/notifications/center"] {
		t.Errorf("api<T>('/api/notifications/center') not seen, got %v", got)
	}
	found := false
	for c := range seen {
		if len(c) > 12 && c[:4] == "GET " && strings.Contains(c, "/api/teams/") {
			found = true
		}
	}
	if !found {
		t.Errorf("api<T>(`/api/teams/...`) not seen, got %v", got)
	}
}

// boxyhq: `${team?.slug}` (optional chaining) inside the address is still one path parameter.
func TestOptionalChainingInAnAddressIsAParameter(t *testing.T) {
	got := callsIn(t, map[string]string{
		"c/Pricing.tsx": "export function P({ team }) {\n  return fetch(`/api/teams/${team?.slug}/payments/create-checkout-session`, { method: 'POST' })\n}\n",
	})
	ok := false
	for _, c := range got {
		if strings.HasPrefix(c, "POST /api/teams/") && strings.HasSuffix(c, "/payments/create-checkout-session") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("got %v", got)
	}
}

// pern-tasks, DevConnect, trek2.0: `import axios from "../api/axios"` is `axios.create({ baseURL: ".../api" })`.
// `axios.post("/signin")` therefore reaches /api/signin; reading it as the address "/signin" made a
// connected screen BROKEN ("no server route handles it"). Same for calls on a named instance.
func TestACallThroughAnAxiosInstanceReachesTheRouteUnderItsBase(t *testing.T) {
	dir := t.TempDir()
	w := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	w("src/routes.js", "const express = require('express')\nconst app = express()\napp.post('/api/signin', (req, res) => res.json({}))\napp.get('/api/profile/:id', (req, res) => res.json({}))\n")
	w("front/api/axios.js", "import axios from 'axios'\nconst client = axios.create({ baseURL: 'http://localhost:3000/api' })\nexport default client\n")
	w("front/Auth.jsx", "import axios from '../api/axios'\nimport api from '../api/axios'\nexport const signin = (d) => axios.post('/signin', d)\n")
	w("front/Profile.jsx", "import client from '../api/axios'\nexport const load = (id) => client.get(`/profile/${id}`)\n")
	g, err := Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/signin", "/api/profile/:id"} {
		var route graph.Node
		for _, n := range g.Nodes {
			if n.Kind == graph.RouteHandler && n.Path == path {
				route = n
			}
		}
		if len(g.CallsToRoute(route)) == 0 {
			t.Errorf("%s has a caller through the axios instance", path)
		}
	}
}

// DevConnect: api/axios.js is `export default axios.create({ baseURL })`. The default export has no
// name there, so the files that import it as `api` were not recognised as making requests.
func TestAnImportedDefaultAxiosInstanceMakesRequests(t *testing.T) {
	got := callsIn(t, map[string]string{
		"client/src/api/axios.js":      "import axios from 'axios'\nexport default axios.create({ baseURL: process.env.REACT_APP_API })\n",
		"client/src/ProfileContext.js": "import api from '../api/axios'\nexport const load = (id) => api.get(`/profile/${id}`)\nexport const save = (d) => api.post('/profile', d)\n",
	})
	seen := map[string]bool{}
	for _, c := range got {
		seen[c] = true
	}
	if !seen["POST /profile"] {
		t.Errorf("api.post('/profile') not seen, got %v", got)
	}
	ok := false
	for c := range seen {
		if strings.HasPrefix(c, "GET /profile/") {
			ok = true
		}
	}
	if !ok {
		t.Errorf("api.get(`/profile/${id}`) not seen, got %v", got)
	}
}

// DevConnect: `router.route('/:username').get(verify, handler)` is an Express route. The chained form
// was not read at all, so the screen that called it looked broken.
func TestChainedExpressRoutesAreRoutes(t *testing.T) {
	dir := t.TempDir()
	w := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	w("server/routes/profile.js", "const router = require('express').Router()\nrouter.route('/:username')\n    .get(verifyJWT, verifyRoles('user'), viewProfile)\n    .delete(verifyJWT, removeProfile)\n\nrouter.route('/:username/edit')\n    .put(verifyJWT, updateProfile)\nmodule.exports = router\n")
	w("server/server.js", "const express = require('express')\nconst profileRoutes = require('./routes/profile')\nconst app = express()\napp.use('/profile', profileRoutes)\nconst x = req.get('host')\n")
	g, err := Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && n.Path != "" {
			got[n.Method+" "+n.Path] = true
		}
	}
	for _, want := range []string{"GET /profile/:username", "DELETE /profile/:username", "PUT /profile/:username/edit"} {
		if !got[want] {
			t.Errorf("missing route %q, got %v", want, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("exactly the three chained routes expected, got %v", got)
	}
}

// vibeworks: the helper is `api(url, { method, body })` and it POSTs when a body is given and no method
// is named. Reading the call as a GET made a connected screen BROKEN ("the server only accepts POST").
func TestAHelperCallWithABodyAndNoMethodIsNotAGet(t *testing.T) {
	got := callsIn(t, map[string]string{
		"lib/api.ts":   "export async function api<T = unknown>(url: string, opts: { method?: string; body?: unknown } = {}): Promise<T> {\n  const method = opts.method ?? (opts.body ? 'POST' : 'GET')\n  const res = await fetch(url, { method, body: JSON.stringify(opts.body) })\n  return res.json()\n}\n",
		"app/Head.tsx": "import { api } from '../lib/api'\nexport function H({ p }) {\n  api(`/api/projects/${p.id}/grave`, { body: { action: 'resurrect' } })\n  api('/api/items')\n  return null\n}\n",
	})
	for _, c := range got {
		if strings.HasPrefix(c, "GET /api/projects/") {
			t.Errorf("a call with a body and no method must not be read as GET: %v", got)
		}
	}
	seen := false
	for _, c := range got {
		if c == "GET /api/items" {
			seen = true
		}
	}
	if !seen {
		t.Errorf("a call with no body is still a GET: %v", got)
	}
}

// Airbnb-clone: `router\n  .route("/")\n  .get(...)\n  .post(...)`: the chain starts on the next line.
func TestChainedRouteStartingOnTheNextLine(t *testing.T) {
	dir := t.TempDir()
	w := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	w("routes/listing.js", "const router = require('express').Router()\n\nrouter\n  .route(\"/\")\n  .get(wrap(index))\n  .post(isLoggedIn, wrap(create));\n\nmodule.exports = router\n")
	w("app.js", "const express = require('express')\nconst listingRoutes = require('./routes/listing')\nconst app = express()\napp.use('/listings', listingRoutes)\n")
	g, err := Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && n.Path != "" {
			got[n.Method+" "+n.Path] = true
		}
	}
	if !got["GET /listings"] || !got["POST /listings"] {
		t.Fatalf("got %v", got)
	}
}

// remuda: `sendJSON("DELETE", `/api/keys/${id}`)`: a helper that takes the method as its first argument.
func TestAHelperThatTakesTheMethodFirst(t *testing.T) {
	got := callsIn(t, map[string]string{
		"lib/http.ts": "export async function sendJSON(method: string, url: string, body?: unknown) {\n  const res = await fetch(url, { method, body: JSON.stringify(body) })\n  return res.json()\n}\n",
		"app/api.ts":  "import { sendJSON } from '../lib/http'\nexport const revoke = (id: string) => sendJSON(\"DELETE\", `/api/keys/${encodeURIComponent(id)}`)\nexport const make = (n: string) => sendJSON(\"POST\", \"/api/keys\", { n })\n",
	})
	seen := map[string]bool{}
	for _, c := range got {
		seen[c] = true
	}
	ok := false
	for c := range seen {
		if strings.HasPrefix(c, "DELETE /api/keys/") {
			ok = true
		}
	}
	if !ok || !seen["POST /api/keys"] {
		t.Fatalf("the method is the first argument, got %v", got)
	}
}
