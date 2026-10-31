package extractor

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// 5PM Club: /api/send-email reads to, subject, text and html but only turns the request away when
// to, subject or text is missing. html is optional, and a page that omits it is not a mismatch.
func TestRequiredFieldsComeFromTheGuard(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	write("app/api/send-email/route.ts", "export async function POST(request: Request) {\n  const body = await request.json();\n  const { to, subject, text, html } = body;\n  if (!to || !subject || !text) {\n    return Response.json({ error: 'Missing' }, { status: 400 });\n  }\n  return Response.json({ ok: html });\n}\n")
	write("server.js", "const express = require('express')\nconst app = express()\napp.post('/api/items', (req, res) => {\n  const { name, note } = req.body\n  if (!name) return res.status(400).end()\n  res.json({ name, note })\n})\n")
	write("noguard.js", "const express = require('express')\nconst app = express()\napp.post('/api/free', (req, res) => {\n  const { a, b } = req.body\n  res.json({ a, b })\n})\n")
	g, err := Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler {
			got[n.Path] = n.RequiredFields
		}
	}
	if want := []string{"to", "subject", "text"}; !reflect.DeepEqual(got["/api/send-email"], want) {
		t.Errorf("send-email required = %v, want %v", got["/api/send-email"], want)
	}
	if want := []string{"name"}; !reflect.DeepEqual(got["/api/items"], want) {
		t.Errorf("items required = %v, want %v", got["/api/items"], want)
	}
	if got["/api/free"] != nil {
		t.Errorf("no guard means unknown (nil), got %v", got["/api/free"])
	}
}
