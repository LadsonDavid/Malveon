package extractor

import (
	"strings"
	"testing"
)

// Found by scanning steltic_india (a lead's repo, 2026-11-10): the browser code calls its server
// through jpost(path, body) -> api(path, opts) -> fetch(path, opts). Only a helper that
// itself calls fetch was read, so every route that screen calls looked unused.
const chainAPI = `async function api(path, opts = {}) {
  const r = await fetch(path, opts);
  if (!r.ok) throw new Error(path);
  return r;
}
async function jpost(path, body) {
  return api(path, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
}
async function jdelete(p) { return api("/api" + p, { method: "DELETE" }); }
const jget = (path) => api(path);
async function unrelated(path, other) { return api("/fixed", { other }); }
async function save() { await jpost("/api/creds", { a: 1 }); }
async function gone() { await jdelete("/items"); }
async function list() { await jget("/api/items"); }
`

func has(got []string, want string) bool {
	for _, g := range got {
		if g == want {
			return true
		}
	}
	return false
}

func TestHelperThatCallsAHelperThatCallsFetchIsARequest(t *testing.T) {
	got := callsIn(t, map[string]string{"frontend/app.js": chainAPI})
	for _, want := range []string{"POST /api/creds", "GET /api/items"} {
		if !has(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
}

func TestChainedHelperKeepsItsOwnPrefixAndMethod(t *testing.T) {
	got := callsIn(t, map[string]string{"frontend/app.js": chainAPI})
	if !has(got, "DELETE /api/items") {
		t.Errorf("jdelete adds /api and sends DELETE: %v", got)
	}
}

func TestAHelperThatDoesNotPassItsParameterOnIsNotARequestHelper(t *testing.T) {
	got := callsIn(t, map[string]string{"frontend/app.js": chainAPI + "async function use() { await unrelated('/should/not/count') }\n"})
	if strings.Contains(strings.Join(got, "\n"), "/should/not/count") {
		t.Errorf("a helper that ignores its path parameter is not a wrapper: %v", got)
	}
}

// Same project: the page links to routes the server serves (a plain link, and a link built in code).
func TestLinksToServedRoutesAreCalls(t *testing.T) {
	got := callsIn(t, map[string]string{
		"steltic/main.py": "from fastapi import FastAPI\napp = FastAPI()\n" +
			"@app.get(\"/terms\")\ndef terms():\n    return 1\n" +
			"@app.get(\"/api/report/{building}/{path:path}\")\ndef report(building: str, path: str):\n    return 1\n",
		"frontend/index.html": "<html><a href=\"/terms\" target=\"_blank\">terms</a> <a href=\"/not-served\">x</a> <a href=\"/files/guide.pdf\">pdf</a></html>\n",
		"frontend/app.js":     "function show(building) {\n  const a = document.createElement('a')\n  a.href = `/api/report/${encodeURIComponent(building)}/report.html`\n  b.href = '/also-not-served'\n}\n",
	})
	for _, want := range []string{"GET /terms", "GET /api/report/:param/report.html"} {
		if !has(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	for _, g := range got {
		if strings.Contains(g, "not-served") || strings.Contains(g, "guide.pdf") {
			t.Errorf("a link to nothing the server serves must not become a call: %v", got)
		}
	}
}

// steltic: api("/api/log/" + building). The text after the slash is one value, so it fills a whole
// segment the way `${building}` does.
func TestAddressMadeOfAFolderAndOneValueIsOneParam(t *testing.T) {
	got := callsIn(t, map[string]string{"app.js": "async function api(p){ return fetch(p) }\n" +
		"async function go(id, a) {\n" +
		"  await fetch('/direct/' + id)\n" +
		"  await api('/via/' + encodeURIComponent(id))\n" +
		"  await fetch('/mixed/' + a + '.json')\n" +
		"  await fetch('/glued' + id)\n" +
		"  await fetch('/pick/' + (a ? 'x' : 'y'))\n" +
		"}\n"})
	for _, want := range []string{"GET /direct/:param", "GET /via/:param"} {
		if !has(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	for _, g := range got {
		if strings.Contains(g, "mixed") || strings.Contains(g, "glued") || strings.Contains(g, "pick") {
			t.Errorf("a value mixed into a segment, or chosen by a condition, stays unread: %v", got)
		}
	}
}
