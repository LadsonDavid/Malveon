package extractor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

func TestScanPython(t *testing.T) {
	src := `
@app.route("/refund", methods=["POST"])
def refund():
    pass

def call_it(user_id):
    requests.get(f"/profile/{user_id}")
`
	sites := scanPython(src)
	if len(sites) != 2 {
		t.Fatalf("expected 2 call sites, got %d: %+v", len(sites), sites)
	}

	route := sites[0]
	if route.kind != graph.RouteHandler {
		t.Errorf("expected first site to be a route handler, got %v", route.kind)
	}
	path, ok := literalPath(route.argRaw)
	if !ok || path != "/refund" {
		t.Errorf("expected literal path /refund, got %q (ok=%v)", path, ok)
	}

	call := sites[1]
	if call.kind != graph.NetworkCall {
		t.Errorf("expected second site to be a network call, got %v", call.kind)
	}
	if _, ok := literalPath(call.argRaw); ok {
		t.Errorf("expected f-string with interpolation to be non-literal, got resolvable path from %q", call.argRaw)
	}
}

func TestScanGo(t *testing.T) {
	src := `
package main

import "net/http"

func main() {
	router := newRouter()
	router.GET("/refund", refundHandler)
	http.HandleFunc("/legacy-refund", legacyHandler)
	httpClient.Get("/upstream/status")
}
`
	fset, file, err := parseGoFile("test.go", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sites := scanGo(src, fset, file)

	var routes, calls int
	for _, s := range sites {
		switch s.kind {
		case graph.RouteHandler:
			routes++
		case graph.NetworkCall:
			calls++
		}
	}
	if routes != 2 {
		t.Errorf("expected 2 route registrations (router.GET, http.HandleFunc), got %d: %+v", routes, sites)
	}
	if calls != 1 {
		t.Errorf("expected 1 outbound call (httpClient.Get), got %d: %+v", calls, sites)
	}
}

// TestWordsForExcludesReservedFilenames covers the real matching bug a
// real project found: a plain-English word in a feature description
// ("page") must never accidentally match every file that happens to
// share a framework-reserved name, since that name says nothing about
// what the file actually does.
func TestWordsForExcludesReservedFilenames(t *testing.T) {
	cases := []struct {
		file string
		path string
		want []string
	}{
		{"src/app/circles/page.tsx", "", nil},
		{"src/app/circles/page.tsx", "/circles", []string{"circles"}},
		{"src/index.ts", "", nil},
		{"cmd/malveon/main.go", "", nil},
		{"pkg/__init__.py", "", nil},
		{"src/components/RefundButton.jsx", "", []string{"refund", "button"}},
	}
	for _, tc := range cases {
		got := wordsFor(tc.file, tc.path)
		if !equalSets(got, tc.want) {
			t.Errorf("wordsFor(%q, %q) = %v, want %v", tc.file, tc.path, got, tc.want)
		}
	}
}

// TestExtractSkipsBuildOutput covers a real bug a reviewer hit: .next
// (Next.js's build output) was missing from the extractor's own skip
// list even though it had been added to the plan-detector's copy —
// Extract was walking into compiled, bundled JS chunks and citing them
// as "evidence" instead of the real source that produced them.
func TestExtractSkipsBuildOutput(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "src/app/api/jobs/route.ts", `
export async function GET(request: Request) {
  return new Response("ok");
}
`)
	// A compiled chunk sitting inside .next that, if scanned, would look
	// like a route registration of its own — Extract must never see it.
	writeTestFile(t, dir, ".next/server/chunks/ssr/fake_bundle.js", `
app.get("/totally-fake-bundled-route", handler);
`)
	writeTestFile(t, dir, "__pycache__/fake.py", `
requests.get("/should-not-be-scanned")
`)

	g, err := Extract(dir)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range g.Nodes {
		if strings.Contains(n.File, ".next") || strings.Contains(n.File, "__pycache__") {
			t.Errorf("expected no nodes from build output/cache dirs, got one from %s", n.File)
		}
	}
}

// TestPlanCheckSignals covers what the plan-items check (2026-09-23)
// reads from code: templated URLs, query strings, direct Supabase calls,
// Next.js pages, and a typed TS function's real line span.
func TestPlanCheckSignals(t *testing.T) {
	for raw, want := range map[string]string{
		"`/api/circles/${id}/members`": "/api/circles/:param/members",
		"'/api/posts?limit=5'":         "/api/posts",
		"`/files/report-${id}`":        "", // value mixed into a segment: unresolved
		"`${BASE}/jobs`":               "", // dynamic prefix: unresolved
	} {
		got, ok := literalPath(raw)
		if (want == "") == ok || got != want {
			t.Errorf("literalPath(%s) = %q, %v; want %q", raw, got, ok, want)
		}
	}

	src := "const { data } = await supabase.from('profiles').select('*')\n" +
		"await supabase.rpc('generate_referral_code')\n" +
		"await supabase.auth.resetPasswordForEmail(email)\n" +
		"const xs = Array.from('abc')\n"
	got := map[string]string{}
	for _, s := range scanJSLike(src) {
		if s.kind == graph.DataCall {
			got[s.argRaw] = s.method
		}
	}
	want := map[string]string{"profiles": "TABLE", "generate_referral_code": "RPC", "resetPasswordForEmail": "AUTH"}
	if len(got) != len(want) {
		t.Fatalf("data calls = %v, want %v (Array.from is never a table)", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("data call %q = %q, want %q", k, got[k], v)
		}
	}

	if s, ok := nextPageSite(filepath.Join("src", "app", "(app)", "circles", "[id]", "page.tsx")); !ok || s.argRaw != `"/circles/:id"` {
		t.Errorf("page site = %+v, %v; want /circles/:id", s, ok)
	}

	g := &graph.Graph{}
	recordDefs(g, "svc.ts", ".ts", "export async function send(\n  id: string\n): Promise<void> {\n  go()\n}\n")
	if d := g.Defs["send"]; len(d) != 1 || d[0].End != 5 {
		t.Errorf("typed function span = %+v, want lines 1-5", d)
	}
}

// Flask's @app.route(path, methods=[...]) gives one route per listed
// method; no list, or a non-literal one, stays unknown (never guessed).
func TestFlaskRouteMethodsKwarg(t *testing.T) {
	src := `@app.route("/api/users/<int:id>", methods=["GET", "DELETE"])
def user(id):
    return {}

@app.route("/api/plain")
def plain():
    return {}

@bp.route("/api/dyn", methods=ALLOWED)
def dyn():
    return {}
`
	got := map[string][]string{}
	for _, s := range scanPython(src) {
		got[s.argRaw] = append(got[s.argRaw], s.method)
	}
	for path, want := range map[string]string{`"/api/users/<int:id>"`: "GET,DELETE", `"/api/plain"`: "", `"/api/dyn"`: ""} {
		if strings.Join(got[path], ",") != want {
			t.Errorf("%s: methods %q, want %q", path, got[path], want)
		}
	}
}

// A call made with an HTTP client library is a call, never also a route
// (found by audit: axios.post('/api/pay') in a frontend-only project made the
// plan line read CONNECTED, with the call as its own server route).
func TestHTTPClientCallsAreNotRoutes(t *testing.T) {
	src := "axios.post('/api/pay', {a: 1});\nconst r = got.get('/api/x');\napp.post('/api/real', (req, res) => {});\n"
	var routes, calls int
	for _, s := range scanJSLike(src) {
		switch s.kind {
		case graph.RouteHandler:
			routes++
		case graph.NetworkCall:
			calls++
		}
	}
	if routes != 1 || calls != 1 {
		t.Errorf("want 1 route (app.post) and 1 call (axios.post), got %d routes, %d calls", routes, calls)
	}
}

// A project whose own folder is called build/dist/vendor/venv/coverage is
// still a project (found by audit: --root .../build read as empty, so every
// plan line came back NOT BUILT).
func TestRootNamedLikeASkippedDirIsStillScanned(t *testing.T) {
	root := filepath.Join(t.TempDir(), "build")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "server.js"), []byte("app.get('/api/items', (req, res) => {});\n"), 0o644)
	g, err := Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 1 {
		t.Fatalf("want the one route found, got %d nodes", len(g.Nodes))
	}
}

func TestPythonRouterPrefixes(t *testing.T) {
	src := "router = APIRouter(prefix=\"/api\")\nbp = Blueprint('x', __name__, url_prefix='/v1')\napp.register_blueprint(bp, url_prefix='/shop')\n" +
		"@router.get(\"/items\")\ndef a(): pass\n@bp.route(\"/cart\")\ndef b(): pass\n@app.get(\"/health\")\ndef c(): pass\n"
	got := map[string]bool{}
	for _, s := range scanPython(src) {
		p, _ := literalPath(s.argRaw)
		got[p] = true
	}
	for _, want := range []string{"/api/items", "/shop/v1/cart", "/health"} {
		if !got[want] {
			t.Errorf("missing route %s in %v", want, got)
		}
	}
}

// Express mounts: prefixes apply to the mounted file's routes, nest through
// routers mounted on routers, and a router nothing mounts is marked unknown.
func TestExpressMounts(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"app.js":            "const v1 = require('./routes/v1');\nconst loose = require('./routes/loose');\napp.use('/api', v1);\n",
		"routes/v1.js":      "const r = require('express').Router();\nr.use('/users', require('./users'));\nmodule.exports = r;\n",
		"routes/users.js":   "const r = require('express').Router();\nr.get('/list', (q, s) => s.json([]));\nmodule.exports = r;\n",
		"routes/loose.js":   "const r = require('express').Router();\nr.get('/list', (q, s) => s.json([]));\nmodule.exports = r;\n",
		"routes/another.js": "const r = require('express').Router();\nr.get('/list', (q, s) => s.json([]));\n",
	}
	for rel, src := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(src), 0o644)
	}
	g, err := Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	var nested, unmounted int
	for _, n := range g.Nodes {
		if n.Kind != graph.RouteHandler {
			continue
		}
		if n.Path == "/api/users/list" {
			nested++
		}
		if n.Unmounted {
			unmounted++
		}
	}
	if nested != 1 {
		t.Errorf("want /api/users/list once (mounted at /api, then /users), got %d", nested)
	}
	if unmounted != 2 { // loose.js is required but never mounted; another.js is not mounted at all
		t.Errorf("want 2 routes marked unmounted, got %d", unmounted)
	}
}

func TestLineOf(t *testing.T) {
	src := "a\nbb\n\nccc"
	for off, want := range map[int]int{0: 1, 1: 1, 2: 2, 4: 2, 5: 3, 6: 4, 8: 4} {
		if got := lineOf(src, off); got != want {
			t.Errorf("lineOf(%d) = %d, want %d", off, got, want)
		}
	}
	other := "x\ny\nz"
	if got := lineOf(other, 4); got != 3 { // a different string rebuilds the index
		t.Errorf("lineOf on another file = %d, want 3", got)
	}
	if got := lineOf(src, 6); got != 4 {
		t.Errorf("lineOf back on the first file = %d, want 4", got)
	}
}

// A downloaded editor build under .vscode-test is not the project's code: its
// routes and calls must not appear in the graph (and, at tens of megabytes, its
// files must not be read at all).
func TestExtractSkipsDownloadedEditorBuilds(t *testing.T) {
	root := t.TempDir()
	for rel, src := range map[string]string{
		"server.js": "app.get('/api/real', (q, s) => s.json({}));\n",
		".vscode-test/vscode-win32-x64-1.127.0/resources/app/extensions/x/index.js": "app.get('/api/from-vscode', (q, s) => s.json({}));\nfetch('/api/from-vscode');\n",
		".yarn/releases/yarn-4.0.0.cjs":                                             "app.get('/api/from-yarn', (q, s) => s.json({}));\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g, err := Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if strings.Contains(n.File, "vscode-test") || strings.Contains(n.File, ".yarn") {
			t.Errorf("read %s:%d from a folder that holds downloaded code (%s %s)", n.File, n.Line, n.Method, n.Path)
		}
	}
	for _, f := range g.Files {
		if strings.Contains(f, "vscode-test") || strings.Contains(f, ".yarn") {
			t.Errorf("walked into %s", f)
		}
	}
	found := false
	for _, n := range g.Nodes {
		found = found || n.Path == "/api/real"
	}
	if !found {
		t.Error("lost the project's own route")
	}
}
