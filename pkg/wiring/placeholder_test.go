package wiring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
)

// Found 2026-09-29 in a user's-eye audit: a plan line naming a URL with a
// Flask/Django (<id>) or FastAPI/Spring/.NET/Laravel ({id}) placeholder was
// cut at the placeholder and matched a different route, so an endpoint that
// was never written came back HALF BUILT, "the server route for /api/users
// exists". And a Flask route's own <int:id> was read as literal text, so
// no real call could ever reach it.
func TestPlaceholderStylesGiveTrueVerdicts(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"app.py": `from flask import Flask, jsonify
app = Flask(__name__)

@app.route("/api/users", methods=["GET"])
def users():
    return jsonify([])

@app.route("/api/users/<int:id>", methods=["GET"])
def user(id):
    return jsonify({})

@app.route("/api/orders", methods=["GET"])
def orders():
    return jsonify([])
`,
		"static/app.js": "export async function load(id) {\n  await fetch(`/api/users/${id}`)\n  await fetch('/api/users')\n  await fetch('/api/orders')\n}\n",
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

	for _, c := range []struct {
		line      string
		want      Verdict
		reasonHas string
		reasonNot string
	}{
		// Never written: no route under /api/orders/<something> at all.
		{"`DELETE /api/orders/{id}` cancels an order", NotBuilt, "/api/orders/:id", "/api/orders exists"},
		{"`DELETE /api/orders/<id>` cancels an order", NotBuilt, "/api/orders/:id", "/api/orders exists"},
		// Written and called: the Flask <int:id> route is reached by the page's `/api/users/${id}`.
		{"`GET /api/users/<id>` shows one user", Working, "", ""},
		{"`GET /api/users/{id}` shows one user", Working, "", ""},
		{"`GET /api/users/:id` shows one user", Working, "", ""},
	} {
		r := check(c.line)
		if r.Verdict != c.want {
			t.Errorf("%s\n  got %s: %s\n  want %s", c.line, r.Verdict, r.Reason, c.want)
			continue
		}
		if c.reasonHas != "" && !strings.Contains(r.Reason, c.reasonHas) {
			t.Errorf("%s: reason should name %q, got %q", c.line, c.reasonHas, r.Reason)
		}
		if c.reasonNot != "" && strings.Contains(r.Reason, c.reasonNot) {
			t.Errorf("%s: reason must not claim %q, got %q", c.line, c.reasonNot, r.Reason)
		}
	}
}

// Found 2026-09-30 on a real Next.js app: with the read-all route file deleted, the
// unrelated /api/notifications/:id/read route counted as "nested under"
// /api/notifications/read-all, and the page's call to the :id route made the
// line CONNECTED. The page also calls read-all, which no route serves: BROKEN.
func TestWildcardSiblingRouteIsNotUnderLiteralPlanURL(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"src/app/api/notifications/[id]/read/route.ts": "export async function PATCH(req: Request) { return Response.json({}) }\n",
		"src/app/notifications/page.tsx":               "export default function P() {\n  const one = (id: string) => fetch(`/api/notifications/${id}/read`, { method: 'PATCH' })\n  const all = () => fetch('/api/notifications/read-all', { method: 'PATCH' })\n  return <button onClick={all}>Mark all read</button>\n}\n",
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
	r := Run(g, []features.Feature{{ID: "x", Name: "Notifications: the \"Mark all read\" button calls `PATCH /api/notifications/read-all`"}})[0]
	if r.Verdict != Broken {
		t.Fatalf("got %s: %s\nwant BROKEN", r.Verdict, r.Reason)
	}
}
