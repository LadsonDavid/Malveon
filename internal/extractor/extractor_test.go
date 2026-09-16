package extractor

import (
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
