package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// open-managed-agents: Hono sub-apps are mounted with app.route("/slack", slackRoutes). Without
// reading that, every sub-app's "/start" looked like the same URL and "collided" with the others.
func TestHonoRouteMountsGiveSubAppsTheirPrefix(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"src/index.ts":  "import { Hono } from 'hono'\nimport slack from './slack'\nimport feishu from './feishu'\nconst app = new Hono()\napp.route('/slack', slack)\napp.route('/feishu', feishu)\nexport default app\n",
		"src/slack.ts":  "import { Hono } from 'hono'\nconst app = new Hono()\napp.post('/start', (c) => c.json({ ok: 1 }))\nexport default app\n",
		"src/feishu.ts": "import { Hono } from 'hono'\nconst app = new Hono()\napp.post('/start', (c) => c.json({ ok: 2 }))\nexport default app\n",
	}
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	g, err := Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler {
			got[n.Method+" "+n.Path] = true
		}
	}
	if !got["POST /slack/start"] || !got["POST /feishu/start"] || got["POST /start"] {
		t.Fatalf("sub-apps must get their mount prefix, got %v", got)
	}
}

// A file holding two routers is mounted twice, each router elsewhere: the mount found for the file
// may be the other router's, so its routes are marked as having an unknown real URL.
func TestAFileWithTwoRoutersHasUnknownUrls(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"index.js": "const express = require('express')\nconst { adminRouter, apiRouter } = require('./mcp')\nconst app = express()\napp.use('/api', adminRouter)\napp.use('/api/mcp/v1', apiRouter)\n",
		"mcp.js":   "const { Router } = require('express')\nconst adminRouter = Router()\nconst apiRouter = Router()\nadminRouter.get('/keys', (req, res) => res.json([]))\napiRouter.get('/agents', (req, res) => res.json([]))\nmodule.exports = { adminRouter, apiRouter }\n",
	}
	for rel, body := range files {
		os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644)
	}
	g, err := Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && !n.Unmounted {
			t.Errorf("%s %s should be Unmounted: the file holds two routers", n.Method, n.Path)
		}
	}
}
