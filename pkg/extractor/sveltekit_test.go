package extractor

import (
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

const svelteKitPkg = `{"devDependencies":{"@sveltejs/kit":"^2.0.0","svelte":"^5.0.0"}}`

func TestSvelteKitServerEndpoints(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": svelteKitPkg,
		"src/routes/api/posts/+server.ts": `
import { json } from '@sveltejs/kit'
export const GET = async () => json([])
export async function POST({ request }) { return json(await request.json()) }`,
		"src/routes/api/posts/[id]/+server.ts": `
export const DELETE = () => new Response(null, { status: 204 })
export const fallback = () => new Response('no')`,
		"src/routes/api/list/+server.js": `
const GET = () => new Response('a')
const PUT = () => new Response('b')
export { GET, PUT as PATCH }`,
		"src/routes/(app)/api/grouped/+server.ts":         `export function GET() { return new Response('x') }`,
		"src/routes/files/[...rest]/+server.ts":           `export const GET = () => new Response('f')`,
		"src/routes/blog/[[lang]]/[slug=word]/+server.ts": `export const GET = () => new Response('b')`,
	})
	wantRoutes(t, g,
		"GET /api/posts", "POST /api/posts", "DELETE /api/posts/:id", "ANY /api/posts/:id",
		"GET /api/list", "PATCH /api/list",
		"GET /api/grouped",
		"GET /files/:...rest?",
		"GET /blog/:slug", "GET /blog/:lang/:slug")
	wantNoRoute(t, g, "PUT /api/list")
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && !n.FileRouted {
			t.Errorf("SvelteKit routes come from file locations: %+v", n)
		}
	}
}

func TestSvelteKitPagesAreOnlyPageFiles(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json":                            svelteKitPkg,
		"src/routes/+page.svelte":                 `<h1>Home</h1>`,
		"src/routes/+layout.svelte":               `<slot/>`,
		"src/routes/+error.svelte":                `<p>oops</p>`,
		"src/routes/(app)/dashboard/+page.svelte": `<h1>Dash</h1>`,
		"src/routes/users/[id]/+page.svelte":      `<h1>User</h1>`,
		"src/routes/helper.ts":                    `export const x = 1`,
		"src/lib/components/Card.svelte":          `<div/>`,
	})
	pages := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind == graph.Page {
			pages[n.Path] = true
		}
	}
	if len(pages) != 3 || !pages["/"] || !pages["/dashboard"] || !pages["/users/:id"] {
		t.Errorf("want /, /dashboard, /users/:id, got %v", pages)
	}
}

func TestSvelteKitFormActionsAndTheFormsThatPostToThem(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": svelteKitPkg,
		"src/routes/login/+page.server.ts": `
export const actions = {
  default: async ({ request }) => {},
  register: async () => {}
}
export const load = () => ({})`,
		"src/routes/login/+page.svelte": `
<script>
  import { enhance } from '$app/forms'
</script>
<form method="POST" action="?/register" use:enhance><button>Go</button></form>
<form method="post"><button>Login</button></form>
<form method="GET" action="/search"><button>Find</button></form>
<form method="POST" action="/other?/save"><button>Save</button></form>`,
		"src/routes/contact/+page.server.ts": `export const load = () => ({})`,
		"src/routes/search/+server.ts":       `export const GET = () => new Response('')`,
	})
	wantRoutes(t, g, "POST /login", "GET /search")
	wantNoRoute(t, g, "POST /contact")
	calls := map[string]int{}
	for _, n := range g.Nodes {
		if n.Kind == graph.NetworkCall && n.Confidence == graph.Extracted {
			calls[n.Method+" "+n.Path]++
		}
	}
	if calls["POST /login"] != 2 {
		t.Errorf("both forms without an outside action post to their own page: %v", calls)
	}
	if calls["GET /search"] != 1 || calls["POST /other"] != 1 {
		t.Errorf("a form with an absolute action calls it: %v", calls)
	}
}

func TestRoutesFolderOutsideSvelteKitIsNotRoutes(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json":                  `{"dependencies":{"express":"^4"}}`,
		"src/routes/api/+server.ts":     `export const GET = () => new Response('')`,
		"src/routes/index/+page.svelte": `<h1>x</h1>`,
	})
	if len(routesOf(g)) != 0 || len(g.PagesAt("/index")) != 0 {
		t.Errorf("a +server.ts file means nothing outside SvelteKit: %v", g.Nodes)
	}
}

// SvelteKit RealWorld: /profile/@[user] is "@" followed by a parameter.
func TestSvelteKitTextAroundAParameterIsAWildcardSegment(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": svelteKitPkg,
		"src/routes/profile/@[user]/+page.svelte":   `<h1>Profile</h1>`,
		"src/routes/profile/@[user]/+layout.svelte": `<form method="POST" action="/profile/@{data.user}?/follow"><button/></form>`,
	})
	if len(g.PagesAt("/profile/@alice")) != 1 {
		t.Errorf("a page at /profile/@[user] serves /profile/@alice: %v", pagesOf(g))
	}
	found := false
	for _, n := range g.Nodes {
		if n.Kind == graph.NetworkCall && n.Method == "POST" && graph.PathsMatch(n.Path, "/profile/@alice") {
			found = true
		}
	}
	if !found {
		t.Errorf("an action with @{name} in it is a call to a profile URL: %v", g.Nodes)
	}
}

func TestSvelteKitLibAliasResolvesImports(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json":            svelteKitPkg,
		"src/lib/Card.svelte":     `<div/>`,
		"src/routes/+page.svelte": "<script>\nimport Card from '$lib/Card.svelte'\n</script>\n<Card/>",
	})
	if imps := g.ImportersOf(g.FindFiles("Card.svelte")[0]); len(imps) != 1 {
		t.Errorf("$lib/ is src/lib/: %v", g.Imports)
	}
}
