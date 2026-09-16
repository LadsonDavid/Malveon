package extractor

import (
	"strings"
	"testing"

	"github.com/LadsonDavid/beta-test/internal/graph"
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
