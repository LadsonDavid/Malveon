package wiring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
)

// A backend-only project (no screens in the repo) used to get HALF BUILT on
// every endpoint, "no frontend call to it was found", unless each line said
// "public API". With no frontend code at all, a route registered at exactly
// the URL the line names is the whole job (founder's call, 2026-09-29).
// Guards stay: a route that only catches the URL through a placeholder, a
// missing route, and a wrong method are still what they were.
func TestBackendOnlyProjectRouteIsEnough(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "app.py"), []byte(`from flask import Flask
app = Flask(__name__)

@app.route("/api/login", methods=["POST"])
def login():
    return {"token": "x"}

@app.route("/api/users/<int:id>", methods=["GET"])
def user(id):
    return {}

@app.route("/api/posts/<slug>", methods=["GET"])
def post(slug):
    return {}

@app.route("/api/plain")
def plain():
    return {}
`), 0o644)
	g, err := extractor.Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	check := func(line string) Result { return Run(g, []features.Feature{{ID: "x", Name: line}})[0] }
	for _, c := range []struct {
		line      string
		want      Verdict
		reasonHas string
	}{
		{"`POST /api/login` returns a token", Working, "no screens"},
		{"`GET /api/users/<id>` returns one user", Working, "no screens"},
		{"`GET /api/users/{id}` returns one user", Working, "no screens"},
		{"`GET /api/posts/pending` lists pending posts", HalfBuilt, "no frontend call"}, // only caught by /api/posts/<slug>
		// The URL exists but the server never registered DELETE, and no screen sends
		// one: not built yet (BROKEN needs a caller that would fail).
		{"`DELETE /api/users/<id>` removes a user", NotBuilt, "no DELETE handler"},
		{"`GET /api/orders` lists orders", NotBuilt, ""},
		// Registered without saying which methods: can't prove the POST was built.
		{"`POST /api/plain` saves a thing", HalfBuilt, ""},
		// No method named: the route existing is enough.
		{"`/api/plain` exists for the mobile app", Working, "no screens"},
	} {
		r := check(c.line)
		if r.Verdict != c.want {
			t.Errorf("%s\n  got %s: %s\n  want %s", c.line, r.Verdict, r.Reason, c.want)
			continue
		}
		if c.reasonHas != "" && !strings.Contains(r.Reason, c.reasonHas) {
			t.Errorf("%s: reason should mention %q: %s", c.line, c.reasonHas, r.Reason)
		}
	}
}

// A project with screens keeps the rule: a route nothing calls is HALF BUILT.
func TestProjectWithScreensStillNeedsACaller(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "app.py"), []byte("@app.route(\"/api/login\", methods=[\"POST\"])\ndef login():\n    return {}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "web.js"), []byte("export const load = () => fetch('/api/profile')\n"), 0o644)
	g, err := extractor.Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	if r := Run(g, []features.Feature{{ID: "x", Name: "`POST /api/login` returns a token"}})[0]; r.Verdict != HalfBuilt {
		t.Errorf("got %s (%s), want HALF BUILT", r.Verdict, r.Reason)
	}
}
