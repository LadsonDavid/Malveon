package extractor

import (
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

func TestIsStubHandler(t *testing.T) {
	cases := []struct {
		name, body string
		want       bool
	}{
		{"express 500 one-liner", "(req,res)=>res.status(500).json({error:'x'})", true},
		{"501", "(req,res)=>{ res.sendStatus(501) }", true},
		{"not implemented", "(req,res)=>{ throw new Error('not implemented') }", true},
		{"next response", "{ return Response.json({ error: 'nope' }, { status: 501 }) }", true},
		{"a real handler with an error branch", "(req,res)=>{\n try {\n const r = await db.save(req.body)\n res.status(200).json(r)\n } catch (e) {\n res.status(500).json({error: 'x'})\n }\n}", false},
		{"short success", "(req,res)=>res.json({ok:true})", false},
		{"short try/catch with a 500 branch", "async (req, res) => {\n  try { await db.save(req.body); res.json({ ok: true }); }\n  catch (e) { res.status(500).json({ error: 'x' }); }\n}", false},
		{"early 500 then a real answer", "(req, res) => {\n  const ok = remove(req.query.id);\n  if (!ok) return res.status(500).json({ error: 'x' });\n  res.send('deleted');\n}", false},
		{"a 404 is an answer, not a stub", "(req, res) => res.status(404).json({ error: 'missing' })", false},
		{"does work, then fails", "async (req, res) => {\n  await audit(req);\n  res.status(500).json({ error: 'x' });\n}", false},
		{"stub with both lines", "(req, res) => {\n  res.status(501);\n  res.json({ error: 'not implemented' });\n}", true},
		{"next handler returning 500 only", "{\n  return NextResponse.json({ error: 'todo' }, { status: 500 });\n}", true},
	}
	for _, c := range cases {
		if got := isStubHandler(c.body); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFetchWrapperAndInstanceCalls(t *testing.T) {
	lib := "export async function api(p, opts) { return fetch(BASE + p, opts); }\nexport const http = axios.create({ baseURL: '/v1' });\n"
	use := "export const a = () => api('/items');\nexport const b = () => api('/items', { method: 'POST' });\nexport const c = () => http.get('/things');\n"
	info := &jsInfo{clients: map[string]bool{}, wrappers: map[string]wrapper{}, mentions: map[string]int{}}
	if idx, m, ok := fetchesParam("{ return fetch(BASE + p, opts); }", "p, opts"); !ok || idx != 0 || m != "GET" {
		t.Fatalf("api() should be found to pass its first parameter to fetch, got %d %q %v", idx, m, ok)
	}
	info.wrappers["api"] = wrapper{param: 0, method: "GET"}
	info.clients["http"] = true
	_ = lib
	var got []string
	for _, s := range scanJSLikeWith(use, info) {
		if s.kind == graph.NetworkCall {
			p, _ := literalPath(s.argRaw)
			got = append(got, s.method+" "+p)
		}
		if s.kind == graph.RouteHandler {
			t.Errorf("a call through %q was read as a route registration", s.argRaw)
		}
	}
	want := map[string]bool{"GET /items": true, "POST /items": true, "GET /things": true}
	for _, g := range got {
		delete(want, g)
	}
	if len(want) != 0 {
		t.Errorf("missing calls %v (got %v)", want, got)
	}
}

func TestUseSWRFormAndConstTemplate(t *testing.T) {
	src := "const BASE = '';\nconst a = useSWR('/api/b');\nconst c = fetch(`${BASE}/api/c`);\nconst f = <form action=\"/api/e\" method=\"post\"></form>;\n"
	got := map[string]string{}
	for _, s := range scanJSLike(src) {
		if s.kind == graph.NetworkCall {
			p, _ := literalPath(s.argRaw)
			got[p] = s.method
		}
	}
	for p, m := range map[string]string{"/api/b": "GET", "/api/c": "GET", "/api/e": "POST"} {
		if got[p] != m {
			t.Errorf("%s: got %q, want %q (all: %v)", p, got[p], m, got)
		}
	}
}

func TestSchemaTablesAndTypos(t *testing.T) {
	g := &graph.Graph{}
	readSchema(g, "m/001.sql", ".sql", "-- create table ghost (id int);\nCREATE TABLE IF NOT EXISTS public.orders (id int);\ncreate view order_totals as select 1;\n")
	readSchema(g, "schema.prisma", ".prisma", "model Customer {\n  id Int\n  @@map(\"customers\")\n}\n")
	for _, name := range []string{"orders", "order_totals", "customer", "customers"} {
		if _, ok := g.Tables[name]; !ok {
			t.Errorf("table %q should be declared (have %v)", name, g.Tables)
		}
	}
	if _, ok := g.Tables["ghost"]; ok {
		t.Error("a commented-out create table is not a table")
	}
	if got := g.NearTable("ordres"); got != "orders" {
		t.Errorf("ordres should be flagged as a typo of orders, got %q", got)
	}
	if got := g.NearTable("orders"); got != "" {
		t.Errorf("a declared table is not a typo, got %q", got)
	}
	if got := g.NearTable("invoices"); got != "" {
		t.Errorf("a name far from every table proves nothing, got %q", got)
	}
}

func TestPythonStubHandlers(t *testing.T) {
	cases := []struct {
		name, src string
		want      bool
	}{
		{"raise NotImplementedError", "@router.get('/x')\ndef x():\n    raise NotImplementedError\n", true},
		{"with a docstring", "@router.get('/x')\nasync def x():\n    \"\"\"Not done yet.\"\"\"\n    raise NotImplementedError('todo')\n", true},
		{"HTTPException 501", "@router.get('/x')\ndef x():\n    raise HTTPException(\n        status_code=501,\n        detail='nope',\n    )\n", true},
		{"flask abort", "@app.route('/x')\ndef x():\n    abort(501)\n", true},
		{"return a 501 tuple", "@app.route('/x')\ndef x():\n    return jsonify(error='x'), 501\n", true},
		{"pass alone says nothing", "@app.get('/x')\ndef x():\n    pass\n", false},
		{"pass then raise", "@app.get('/x')\ndef x():\n    pass\n    raise NotImplementedError\n", true},
		{"a real handler", "@router.get('/x')\ndef x():\n    items = db.all()\n    return items\n", false},
		{"a 404 is an answer", "@router.get('/x')\ndef x():\n    raise HTTPException(status_code=404)\n", false},
		{"does work, then fails", "@router.get('/x')\ndef x():\n    audit()\n    raise NotImplementedError\n", false},
		{"success tuple", "@app.route('/x')\ndef x():\n    return jsonify(ok=True), 200\n", false},
		{"stacked decorators", "@router.get('/x')\n@auth\ndef x():\n    raise NotImplementedError\n", true},
	}
	for _, c := range cases {
		stub := false
		for _, s := range scanPython(c.src) {
			stub = s.stub
		}
		if stub != c.want {
			t.Errorf("%s: stub=%v, want %v", c.name, stub, c.want)
		}
	}
}
