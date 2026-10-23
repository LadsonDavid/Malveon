package wiring

import (
	"strings"
	"testing"
)

// A call from a test file is not the app calling a route (2026-10-14, found on
// trpc/examples-next-prisma-starter, where post.test.ts called post.byId and
// post.add and hid that nothing else would). The route stays HALF BUILT, and
// the reason names the test so the route isn't mistaken for ignored.

// itemsApp is a Next.js app with one API route and a page that doesn't call it.
var itemsApp = map[string]string{
	"package.json":               `{"dependencies":{"next":"15"}}`,
	"src/app/page.tsx":           `export default function Home() { return null }`,
	"src/app/api/items/route.ts": `export async function GET() { return Response.json([]) }`,
	"src/app/api/items/route.test.ts": `
import { GET } from "./route"
it("lists items", async () => { await fetch("/api/items"); expect(GET).toBeDefined() })`,
}

func withFile(base map[string]string, rel, src string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	out[rel] = src
	return out
}

func TestRouteOnlyATestCallsIsHalfBuilt(t *testing.T) {
	check := sweepProject(t, itemsApp)
	r := check("`GET /api/items` lists the items")
	if r.Verdict != HalfBuilt {
		t.Fatalf("want HALF BUILT, got %s: %s", r.Verdict, r.Reason)
	}
	for _, want := range []string{"only tests call it", "route.test.ts"} {
		if !strings.Contains(r.Reason, want) {
			t.Errorf("reason %q should mention %q", r.Reason, want)
		}
	}
}

func TestSpecAndTestFolderCallsDoNotCountEither(t *testing.T) {
	for _, rel := range []string{"src/app/items.spec.tsx", "src/__tests__/items.js", "cypress/e2e/items.cy.ts", "e2e/items.ts"} {
		files := withFile(itemsApp, rel, `fetch("/api/items")`)
		delete(files, "src/app/api/items/route.test.ts")
		if r := sweepProject(t, files)("`GET /api/items` lists the items"); r.Verdict != HalfBuilt {
			t.Errorf("%s: a test call made the route %s: %s", rel, r.Verdict, r.Reason)
		}
	}
}

func TestRealPageCallStillCountsNextToATest(t *testing.T) {
	files := withFile(itemsApp, "src/app/items/page.tsx", `
export default function Items() { fetch("/api/items"); return null }`)
	r := sweepProject(t, files)("`GET /api/items` lists the items")
	if r.Verdict != Working {
		t.Fatalf("want CONNECTED (a page calls it), got %s: %s", r.Verdict, r.Reason)
	}
	if strings.Contains(r.Reason, "only tests") {
		t.Errorf("the page does call it: %s", r.Reason)
	}
}

// A test fetching a URL nothing serves is a test of a 404, not a broken screen.
func TestTestCallToAnUnservedUrlIsNotBroken(t *testing.T) {
	files := withFile(itemsApp, "src/app/missing.test.ts", `it("404s", async () => { await fetch("/api/nope") })`)
	r := sweepProject(t, files)("`GET /api/nope` returns nothing")
	if r.Verdict == Broken {
		t.Fatalf("a test's call must not make a URL BROKEN: %s", r.Reason)
	}
}

func TestTRPCProcedureOnlyATestCallsIsHalfBuilt(t *testing.T) {
	files := withFile(trpcApp, "src/server/api/routers/post.test.ts", `
import { appRouter } from "../root"
const caller = appRouter.createCaller({})
it("greets", async () => { expect(await caller.post.hello()).toBe("hi") })`)
	r := sweepProject(t, files)("The `post.hello` query returns a greeting")
	if r.Verdict != HalfBuilt {
		t.Fatalf("want HALF BUILT, got %s: %s", r.Verdict, r.Reason)
	}
	if !strings.Contains(r.Reason, "only tests call it") || !strings.Contains(r.Reason, "post.test.ts") {
		t.Errorf("reason should name the test: %s", r.Reason)
	}
}

// post.getLatest and post.create are called by the real page in trpcApp: a test
// calling them as well changes nothing.
func TestTRPCProcedureAPageCallsStaysConnectedWithATest(t *testing.T) {
	files := withFile(trpcApp, "src/server/api/routers/post.test.ts", `
import { appRouter } from "../root"
const caller = appRouter.createCaller({})
it("latest", async () => { await caller.post.getLatest() })`)
	if r := sweepProject(t, files)("The `post.getLatest` query shows the latest post"); r.Verdict != Working {
		t.Fatalf("want CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
}

// A plan line that names no URL is matched by its words; a test's fetch must not
// be the thing it matches.
func TestWordMatchIsNotMadeConnectedByATest(t *testing.T) {
	files := map[string]string{
		"server.js":                    `const app = require("express")(); app.post("/refund", (req, res) => res.json({}))`,
		"src/__tests__/refund.test.js": `it("refunds", () => fetch("/refund", { method: "POST" }))`,
	}
	r := sweepProject(t, files)("refund button")
	if r.Verdict != HalfBuilt || !strings.Contains(r.Reason, "only tests call it") {
		t.Fatalf("want HALF BUILT naming the test, got %s: %s", r.Verdict, r.Reason)
	}
}

// A backend whose tests fetch its routes still has no screens, so a route is
// still "the whole job" there (a test file used to make it look like it had a frontend).
func TestBackendOnlyProjectWithTestsIsStillBackendOnly(t *testing.T) {
	files := map[string]string{
		"server.js":      `const app = require("express")(); app.get("/api/items", (req, res) => res.json([]))`,
		"server.test.js": `it("lists", () => fetch("/api/items"))`,
	}
	r := sweepProject(t, files)("`GET /api/items` lists the items")
	if r.Verdict != Working || !strings.Contains(r.Reason, "no screens") {
		t.Fatalf("want CONNECTED as a backend-only route, got %s: %s", r.Verdict, r.Reason)
	}
}
