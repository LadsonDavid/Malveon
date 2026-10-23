package extractor

import (
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

const remixPkg = `{"dependencies":{"@remix-run/react":"^2","@remix-run/node":"^2"}}`
const rrPkg = `{"dependencies":{"react-router":"^7","@react-router/node":"^7"},"devDependencies":{"@react-router/dev":"^7"}}`

func TestRemixFlatRoutesBecomePagesAndResourceRoutes(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json":          remixPkg,
		"app/root.tsx":          `export default function Root() { return null }`,
		"app/routes/_index.tsx": `export default function Index() { return null }`,
		"app/routes/about.tsx":  `export default function About() { return null }`,
		"app/routes/posts.$id.tsx": `
export async function loader() { return null }
export async function action() { return null }
export default function Post() { return null }`,
		"app/routes/api.items.ts": `
export async function loader() { return Response.json([]) }
export const action = async () => Response.json({})`,
		"app/routes/_auth.login.tsx":     `export default function Login() { return null }`,
		"app/routes/($lang).docs.tsx":    `export default function Docs() { return null }`,
		"app/routes/files.$.tsx":         `export default function Files() { return null }`,
		"app/routes/sitemap[.]xml.ts":    `export const loader = () => new Response("<xml/>")`,
		"app/routes/dashboard/route.tsx": `export default function D() { return null }`,
		"app/routes/dashboard/helper.ts": `export const x = 1`,
		"app/routes/concerts_.mine.tsx":  `export default function M() { return null }`,
	})
	got := pagesOf(g)
	for _, w := range []string{"/", "/about", "/posts/:id", "/login", "/docs", "/:lang/docs", "/files/:...splat?", "/dashboard", "/concerts/mine"} {
		if !got[w] {
			t.Errorf("missing page %q; have %v", w, got)
		}
	}
	wantRoutes(t, g,
		"POST /posts/:id", "PUT /posts/:id", "PATCH /posts/:id", "DELETE /posts/:id",
		"GET /api/items", "POST /api/items", "DELETE /api/items",
		"GET /sitemap.xml")
	wantNoRoute(t, g, "GET /posts/:id") // a page's loader is the page itself
	wantNoRoute(t, g, "GET /dashboard/helper")
	for _, n := range g.Nodes {
		if (n.Kind == graph.RouteHandler || n.Kind == graph.Page) && !n.FileRouted {
			t.Errorf("Remix routes come from file locations: %+v", n)
		}
	}
}

func TestReactRouterConfigRoutes(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": rrPkg,
		"app/routes.ts": `
import { type RouteConfig, route, index, layout, prefix } from "@react-router/dev/routes"
export default [
  index("./home.tsx"),
  route("about", "./about.tsx"),
  layout("./auth/layout.tsx", [
    route("login", "./auth/login.tsx"),
  ]),
  ...prefix("concerts", [
    index("./concerts/home.tsx"),
    route(":city", "./concerts/city.tsx"),
  ]),
  route("api/items/:itemId", "./api/item.ts"),
  route("files/*", "./files.tsx"),
] satisfies RouteConfig`,
		"app/home.tsx":          `export default function H() { return null }`,
		"app/about.tsx":         `export default function A() { return null }`,
		"app/auth/layout.tsx":   `export default function L() { return null }`,
		"app/auth/login.tsx":    `export async function action() { return null }\nexport default function L() { return null }`,
		"app/concerts/home.tsx": `export default function C() { return null }`,
		"app/concerts/city.tsx": `export default function C() { return null }`,
		"app/api/item.ts":       `export async function loader() { return null }`,
		"app/files.tsx":         `export default function F() { return null }`,
		"app/unused.tsx":        `export default function U() { return null }`,
	})
	got := pagesOf(g)
	for _, w := range []string{"/", "/about", "/login", "/concerts", "/concerts/:city", "/files/:...splat?"} {
		if !got[w] {
			t.Errorf("missing page %q; have %v", w, got)
		}
	}
	if got["/unused"] || got["/auth"] {
		t.Errorf("only configured modules are routes and layouts are not pages: %v", got)
	}
	wantRoutes(t, g, "GET /api/items/:itemId", "POST /login")
}

func TestRemixCalls(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": remixPkg,
		"app/routes/posts.$id.tsx": `
import { Form, useFetcher, useSubmit } from "@remix-run/react"
export async function action() { return null }
export default function Post() {
  const fetcher = useFetcher()
  const submit = useSubmit()
  return (
    <div>
      <Form method="post"><button>Own</button></Form>
      <Form method="delete" action="/api/items"><button>Del</button></Form>
      <fetcher.Form method="post" action="/api/items"><button>Add</button></fetcher.Form>
      <Form method="get"><button>Search</button></Form>
      <button onClick={() => fetcher.submit({ a: 1 }, { method: "post", action: "/api/items" })}>x</button>
      <button onClick={() => fetcher.load("/api/items")}>y</button>
      <button onClick={() => submit(null, { method: "put", action: "/api/other" })}>z</button>
    </div>
  )
}`,
		"app/routes/api.items.ts": `export async function loader() { return null }\nexport async function action() { return null }`,
	})
	calls := map[string]int{}
	for _, n := range g.Nodes {
		if n.Kind == graph.NetworkCall && n.Confidence == graph.Extracted {
			calls[n.Method+" "+n.Path]++
		}
	}
	if calls["POST /posts/:id"] != 1 {
		t.Errorf("a <Form> with no action posts to its own route: %v", calls)
	}
	if calls["DELETE /api/items"] != 1 || calls["POST /api/items"] != 2 || calls["GET /api/items"] != 1 || calls["PUT /api/other"] != 1 {
		t.Errorf("forms, fetcher.submit, fetcher.load and submit all call their action: %v", calls)
	}
	if calls["GET /posts/:id"] != 0 {
		t.Errorf("a GET <Form> with no action is not a call: %v", calls)
	}
}

func TestRoutesFolderOutsideRemixIsNothing(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json":         `{"dependencies":{"react":"^18"}}`,
		"app/routes/about.tsx": `export default function About() { return null }`,
		"app/routes/api.x.ts":  `export const loader = () => null`,
	})
	if len(routesOf(g)) != 0 || len(pagesOf(g)) != 0 {
		t.Errorf("no Remix or React Router dependency: %v %v", routesOf(g), pagesOf(g))
	}
}
