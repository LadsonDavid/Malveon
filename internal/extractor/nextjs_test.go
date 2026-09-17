package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LadsonDavid/beta-test/internal/graph"
)

func writeTestFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNextRoutePath(t *testing.T) {
	cases := []struct {
		file string
		want string
		ok   bool
	}{
		{"src/app/api/jobs/route.ts", "/api/jobs", true},
		{"app/api/jobs/route.ts", "/api/jobs", true},
		{"src/app/api/jobs/[id]/route.ts", "/api/jobs/:id", true},
		{"src/app/api/posts/[id]/comments/route.ts", "/api/posts/:id/comments", true},
		{"src/app/(app)/notifications/route.ts", "/notifications", true},
		{"src/app/api/[...slug]/route.ts", "/api/:slug", true},
		{"src/app/api/[[...slug]]/route.ts", "/api/:slug", true},
		{"src/app/route.ts", "/", true},
		{"src/lib/supabase.ts", "", false},
		{"route.ts", "", false}, // no "app" ancestor at all
	}
	for _, tc := range cases {
		got, ok := nextRoutePath(tc.file)
		if ok != tc.ok {
			t.Errorf("nextRoutePath(%q) ok = %v, want %v", tc.file, ok, tc.ok)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("nextRoutePath(%q) = %q, want %q", tc.file, got, tc.want)
		}
	}
}

func TestIsNextRouteFile(t *testing.T) {
	cases := map[string]bool{
		"src/app/api/jobs/route.ts":  true,
		"src/app/api/jobs/route.tsx": true,
		"src/app/api/jobs/route.js":  true,
		"src/app/api/jobs/page.tsx":  false,
		"src/lib/routes.ts":          false,
	}
	for file, want := range cases {
		if got := IsNextRouteFile(file); got != want {
			t.Errorf("IsNextRouteFile(%q) = %v, want %v", file, got, want)
		}
	}
}

func TestScanNextRouteHandlers(t *testing.T) {
	src := `import { NextResponse } from "next/server";

export async function GET(request: Request) {
  return NextResponse.json({ ok: true });
}

export async function POST(request: Request) {
  const body = await request.json();
  return NextResponse.json({ ok: true });
}
`
	sites := scanNextRouteHandlers("src/app/api/jobs/route.ts", src)
	if len(sites) != 2 {
		t.Fatalf("expected 2 sites (GET, POST), got %d: %+v", len(sites), sites)
	}
	methods := map[string]bool{}
	for _, s := range sites {
		if s.kind != graph.RouteHandler {
			t.Errorf("expected RouteHandler kind, got %v", s.kind)
		}
		path, ok := literalPath(s.argRaw)
		if !ok || path != "/api/jobs" {
			t.Errorf("expected path /api/jobs, got %q (ok=%v)", path, ok)
		}
		methods[s.method] = true
	}
	if !methods["GET"] || !methods["POST"] {
		t.Errorf("expected both GET and POST, got %v", methods)
	}
}

func TestScanNextRouteHandlersConstStyle(t *testing.T) {
	src := `export const GET = async (request: Request) => {
  return new Response("ok");
};
`
	sites := scanNextRouteHandlers("src/app/api/health/route.ts", src)
	if len(sites) != 1 || sites[0].method != "GET" {
		t.Fatalf("expected 1 GET site for const-style export, got %+v", sites)
	}
}

// TestExtractFindsNextJSRoutes proves the real end-to-end gap this
// session's field test found is fixed: a Next.js route handler file now
// shows up as a real, wireable backend route, not silent nothing.
func TestExtractFindsNextJSRoutes(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "src/app/api/jobs/route.ts", `
import { NextResponse } from "next/server";

export async function GET(request: Request) {
  return NextResponse.json({ jobs: [] });
}
`)
	writeTestFile(t, dir, "src/components/JobBoard.tsx", `
function loadJobs() {
  fetch("/api/jobs");
}
`)

	g, err := Extract(dir)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	var foundRoute, foundCall bool
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && n.Path == "/api/jobs" && n.Method == "GET" && n.Confidence == graph.Extracted {
			foundRoute = true
		}
		if n.Kind == graph.NetworkCall && n.Path == "/api/jobs" {
			foundCall = true
		}
	}
	if !foundRoute {
		t.Errorf("expected a GET /api/jobs route handler node, got: %+v", g.Nodes)
	}
	if !foundCall {
		t.Errorf("expected a /api/jobs network call node, got: %+v", g.Nodes)
	}
}
