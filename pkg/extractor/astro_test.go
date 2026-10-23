package extractor

import (
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

const astroPkg = `{"dependencies":{"astro":"^4.15.0"}}`

func pagesOf(g *graph.Graph) map[string]bool {
	out := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind == graph.Page {
			out[n.Path] = true
		}
	}
	return out
}

func TestAstroPages(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json":                   astroPkg,
		"src/pages/index.astro":          "---\n---\n<h1>Home</h1>",
		"src/pages/about.astro":          "<h1>About</h1>",
		"src/pages/blog/index.astro":     "<h1>Blog</h1>",
		"src/pages/blog/[slug].astro":    "<h1>Post</h1>",
		"src/pages/docs/[...path].astro": "<h1>Docs</h1>",
		"src/pages/guide.md":             "# Guide",
		"src/pages/404.astro":            "<h1>Not found</h1>",
		"src/pages/_hidden.astro":        "<h1>no</h1>",
		"src/pages/_partials/x.astro":    "<h1>no</h1>",
		"src/components/Card.astro":      "<div/>",
	})
	got := pagesOf(g)
	for _, w := range []string{"/", "/about", "/blog", "/blog/:slug", "/docs/:...path?", "/guide", "/404"} {
		if !got[w] {
			t.Errorf("missing page %q; have %v", w, got)
		}
	}
	if len(got) != 7 {
		t.Errorf("underscore files and components are not pages: %v", got)
	}
}

func TestAstroEndpoints(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": astroPkg,
		"src/pages/api/posts.ts": `
import type { APIRoute } from "astro"
export const GET: APIRoute = async () => new Response("[]")
export async function POST({ request }) { return new Response(null, { status: 201 }) }`,
		"src/pages/api/posts/[id].ts": `export const DELETE = () => new Response(null, { status: 204 })`,
		"src/pages/api/all.js":        `export const ALL = () => new Response("x")`,
		"src/pages/sitemap.xml.ts":    `export const GET = () => new Response("<xml/>")`,
		"src/pages/api/index.ts":      `export function GET() { return new Response("root") }`,
		"src/pages/api/list.ts":       `const GET = () => new Response(""); const PUT = () => new Response(""); export { GET, PUT as PATCH }`,
		"src/pages/api/_private.ts":   `export const GET = () => new Response("")`,
		"src/pages/api/helpers.ts":    `export const slug = (s: string) => s`,
		"src/lib/not-a-route.ts":      `export const GET = () => new Response("")`,
	})
	wantRoutes(t, g,
		"GET /api/posts", "POST /api/posts", "DELETE /api/posts/:id", "ANY /api/all",
		"GET /sitemap.xml", "GET /api", "GET /api/list", "PATCH /api/list")
	for _, bad := range []string{"GET /api/_private", "GET /api/helpers", "GET /lib/not-a-route", "PUT /api/list"} {
		wantNoRoute(t, g, bad)
	}
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && !n.FileRouted {
			t.Errorf("Astro routes come from file locations: %+v", n)
		}
	}
}

func TestPagesFolderOutsideAstroIsNothing(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json":           `{"dependencies":{"express":"^4"}}`,
		"src/pages/api/posts.ts": `export const GET = () => new Response("")`,
		"src/pages/index.astro":  "<h1/>",
	})
	if len(routesOf(g)) != 0 || len(pagesOf(g)) != 0 {
		t.Errorf("no astro dependency: %v %v", routesOf(g), pagesOf(g))
	}
}

func TestAstroActions(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": astroPkg,
		"src/actions/index.ts": `
import { defineAction } from "astro:actions"
import { z } from "astro:schema"
export const server = {
  subscribe: defineAction({ input: z.object({ email: z.string() }), handler: async () => ({ ok: true }) }),
  blog: {
    like: defineAction({ handler: async () => 1 }),
  },
}`,
		"src/components/Form.tsx": `
import { actions } from "astro:actions"
export async function go() {
  await actions.subscribe({ email: "e" })
  await actions.blog.like({ id: 1 })
  await actions.missing()
}`,
	})
	wantRPC(t, g, "POST /_actions/subscribe", "POST /_actions/blog.like")
	calls := rpcCalls(g)
	if !calls["POST /_actions/subscribe"] || !calls["POST /_actions/blog.like"] || len(calls) != 2 {
		t.Errorf("calls to existing actions only: %v", calls)
	}
}
