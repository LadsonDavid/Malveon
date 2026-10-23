package extractor

import (
	"sort"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

const trpcPkg = `{"dependencies":{"@trpc/server":"^11","@trpc/client":"^11","@trpc/react-query":"^11","next":"15"}}`

func rpcRoutes(g *graph.Graph) []string {
	var out []string
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && n.RPC {
			m := n.Method
			if m == "" {
				m = "ANY"
			}
			s := m + " " + n.Path
			if n.Confidence != graph.Extracted {
				s += " ?"
			}
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func rpcCalls(g *graph.Graph) map[string]bool {
	out := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind == graph.NetworkCall && n.RPC {
			out[n.Method+" "+n.Path] = true
		}
	}
	return out
}

func wantRPC(t *testing.T, g *graph.Graph, want ...string) {
	t.Helper()
	have := map[string]bool{}
	for _, r := range rpcRoutes(g) {
		have[r] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("missing procedure %q; have %v", w, rpcRoutes(g))
		}
	}
}

var t3App = map[string]string{
	"package.json": trpcPkg,
	"src/server/api/trpc.ts": `
import { initTRPC } from "@trpc/server"
const t = initTRPC.create()
export const createTRPCRouter = t.router
export const publicProcedure = t.procedure`,
	"src/server/api/routers/post.ts": `
import { z } from "zod"
import { createTRPCRouter, publicProcedure } from "../trpc"
export const postRouter = createTRPCRouter({
  hello: publicProcedure.input(z.object({ text: z.string() })).query(({ input }) => ({ greeting: input.text })),
  create: publicProcedure.input(z.object({ name: z.string().min(1) })).mutation(async ({ input }) => {
    return { ok: true }
  }),
  getLatest: publicProcedure.query(() => null),
})`,
	"src/server/api/root.ts": `
import { postRouter } from "./routers/post"
import { createTRPCRouter } from "./trpc"
export const appRouter = createTRPCRouter({ post: postRouter })
export type AppRouter = typeof appRouter`,
	"src/app/api/trpc/[trpc]/route.ts": `
import { fetchRequestHandler } from "@trpc/server/adapters/fetch"
import { appRouter } from "~/server/api/root"
const handler = (req: Request) => fetchRequestHandler({ endpoint: "/api/trpc", req, router: appRouter, createContext: () => ({}) })
export { handler as GET, handler as POST }`,
	"src/app/_components/post.tsx": `
"use client"
import { api } from "~/trpc/react"
export function LatestPost() {
  const [latest] = api.post.getLatest.useSuspenseQuery()
  const utils = api.useUtils()
  const create = api.post.create.useMutation({ onSuccess: async () => { await utils.post.getLatest.invalidate() } })
  return <button onClick={() => create.mutate({ name: "x" })}>go</button>
}`,
	"src/trpc/react.tsx": `
import { createTRPCReact } from "@trpc/react-query"
export const api = createTRPCReact()`,
}

func TestTRPCProceduresGetTheirWireUrls(t *testing.T) {
	g := extractProject(t, t3App)
	wantRPC(t, g, "GET /api/trpc/post.hello", "POST /api/trpc/post.create", "GET /api/trpc/post.getLatest")
	if len(rpcRoutes(g)) != 3 {
		t.Errorf("exactly the three procedures: %v", rpcRoutes(g))
	}
}

func TestTRPCClientCallsBecomeCallsToTheirProcedure(t *testing.T) {
	g := extractProject(t, t3App)
	calls := rpcCalls(g)
	if !calls["POST /api/trpc/post.create"] || !calls["GET /api/trpc/post.getLatest"] {
		t.Errorf("hooks call their procedures: %v", calls)
	}
	if len(calls) != 2 {
		t.Errorf("invalidate() and useUtils() are not calls, and post.hello is never called: %v", calls)
	}
}

func TestTRPCVanillaClientAndCaller(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": trpcPkg,
		"server/router.ts": `
import { initTRPC } from "@trpc/server"
const t = initTRPC.create()
export const appRouter = t.router({
  user: t.router({
    byId: t.procedure.input(String).query(() => ({})),
    add: t.procedure.input(Object).mutation(() => ({})),
  }),
})
export type AppRouter = typeof appRouter`,
		"client/index.ts": `
import { createTRPCClient, httpBatchLink } from "@trpc/client"
const client = createTRPCClient({ links: [httpBatchLink({ url: "http://localhost:3000/trpc" })] })
export async function go() {
  await client.user.byId.query("1")
  await client.user.add.mutate({ name: "x" })
  await client.user.nope.query()
}`,
		"server/caller.ts": `
import { appRouter } from "./router"
const caller = appRouter.createCaller({})
export const x = () => caller.user.byId("1")`,
		"server/db.ts": `export const y = () => prisma.user.byId("1")`,
	})
	wantRPC(t, g, "GET /trpc/user.byId", "POST /trpc/user.add")
	calls := rpcCalls(g)
	if !calls["GET /trpc/user.byId"] || !calls["POST /trpc/user.add"] {
		t.Errorf("vanilla client calls: %v", calls)
	}
	if calls["GET /trpc/user.nope"] || calls["POST /trpc/user.nope"] {
		t.Errorf("a path that is no procedure is not a call to one: %v", calls)
	}
	n := 0
	for _, nd := range g.Nodes {
		if nd.Kind == graph.NetworkCall && nd.RPC && nd.File == "server/db.ts" {
			n++
		}
	}
	if n != 0 {
		t.Errorf("prisma.user.byId() is not a tRPC caller")
	}
}

func TestTRPCExpressMountGivesTheBasePath(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": trpcPkg,
		"src/router.ts": `
import { initTRPC } from "@trpc/server"
const t = initTRPC.create()
export const appRouter = t.router({ ping: t.procedure.query(() => "pong") })`,
		"src/server.ts": `
import express from "express"
import * as trpcExpress from "@trpc/server/adapters/express"
const app = express()
app.use("/rpc", trpcExpress.createExpressMiddleware({ router: appRouter }))`,
	})
	wantRPC(t, g, "GET /rpc/ping")
}

func TestTRPCMergedRoutersAndProcedureConstants(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": trpcPkg,
		"src/a.ts": `
import { initTRPC } from "@trpc/server"
const t = initTRPC.create()
const listAll = t.procedure.query(() => [])
export const aRouter = t.router({ listAll, other: listAll })
export const bRouter = t.router({ health: t.procedure.query(() => "ok") })
export const appRouter = t.mergeRouters(aRouter, bRouter)`,
	})
	wantRPC(t, g, "GET /api/trpc/listAll", "GET /api/trpc/other", "GET /api/trpc/health")
}

func TestTRPCUnresolvedSubRouterLeavesAMarker(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": trpcPkg,
		"src/router.ts": `
import { initTRPC } from "@trpc/server"
import { billingRouter } from "@acme/billing"
const t = initTRPC.create()
export const appRouter = t.router({ billing: billingRouter, ok: t.procedure.query(() => 1) })`,
	})
	wantRPC(t, g, "GET /api/trpc/ok", "ANY  ?")
}

func TestRouterCallsOutsideTRPCProjectsAreIgnored(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": `{"dependencies":{"express":"^4"}}`,
		"src/a.ts": `
const appRouter = router({ post: x.query(() => 1) })
api.post.create.useMutation()`,
	})
	if len(rpcRoutes(g)) != 0 || len(rpcCalls(g)) != 0 {
		t.Errorf("no @trpc dependency, no tRPC: %v %v", rpcRoutes(g), rpcCalls(g))
	}
}

// Found on a real project (trpc/examples-next-prisma-starter): a comment with an
// apostrophe ("we'll use as next cursor") inside a router hid every procedure after it.
func TestTRPCCommentsWithApostrophesDoNotHideProcedures(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": trpcPkg,
		"src/router.ts": `
import { initTRPC } from "@trpc/server"
const t = initTRPC.create()
export const postRouter = t.router({
  list: t.procedure
    .input(Object)
    .query(async ({ input }) => {
      // get an extra item at the end which we'll use as next cursor
      const take = input.limit + 1
      /* it's fine, don't worry */
      return []
    }),
  byId: t.procedure.query(() => {
    // can't be null
    return 1
  }),
  add: t.procedure.mutation(() => 2),
})
export const appRouter = t.router({ post: postRouter })`,
	})
	wantRPC(t, g, "GET /api/trpc/post.list", "GET /api/trpc/post.byId", "POST /api/trpc/post.add")
}
