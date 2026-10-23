package wiring

import (
	"strings"
	"testing"
)

// End-to-end checks for the frameworks added after Next.js: each is a small
// project and plan lines whose true answer is known.

var djangoApp = map[string]string{
	"config/settings.py": `ROOT_URLCONF = "config.urls"`,
	"config/urls.py": `
from django.urls import include, path
urlpatterns = [path("shop/", include("shop.urls"))]`,
	"shop/urls.py": `
from django.urls import path
from . import views
urlpatterns = [
    path("orders/", views.orders, name="orders"),
    path("orders/<int:pk>/cancel/", views.cancel_order, name="cancel_order"),
    path("orders/<int:pk>/refund/", views.refund_order, name="refund_order"),
]`,
	"shop/views.py": `
from django.views.decorators.http import require_POST
def orders(request): pass

@require_POST
def cancel_order(request, pk): pass

@require_POST
def refund_order(request, pk): pass
`,
	"shop/templates/orders.html": `
<form method="post" action="{% url 'cancel_order' order.pk %}"><button>Cancel</button></form>
<a href="{% url 'orders' %}">All orders</a>
`,
}

func TestDjangoFormReachesAViewThatAcceptsPost(t *testing.T) {
	check := sweepProject(t, djangoApp)
	r := check("`POST /shop/orders/<int:pk>/cancel/` cancels an order")
	if r.Verdict != Working {
		t.Fatalf("want CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
}

func TestDjangoRouteNothingCallsIsHalfBuilt(t *testing.T) {
	check := sweepProject(t, djangoApp)
	r := check("`POST /shop/orders/<int:pk>/refund/` refunds an order")
	if r.Verdict != HalfBuilt {
		t.Fatalf("a view no template or script calls is HALF BUILT, got %s: %s", r.Verdict, r.Reason)
	}
}

func TestDjangoViewThatDoesNotAcceptTheMethodIsNotBuilt(t *testing.T) {
	check := sweepProject(t, djangoApp)
	r := check("`DELETE /shop/orders/<int:pk>/cancel/` removes an order")
	if r.Verdict != NotBuilt || !strings.Contains(r.Reason, "no DELETE handler") {
		t.Fatalf("want NOT BUILT (no DELETE handler), got %s: %s", r.Verdict, r.Reason)
	}
}

func TestDjangoMissingUrlIsNotBuilt(t *testing.T) {
	check := sweepProject(t, djangoApp)
	r := check("`GET /shop/invoices/` lists invoices")
	if r.Verdict != NotBuilt {
		t.Fatalf("want NOT BUILT, got %s: %s", r.Verdict, r.Reason)
	}
}

var nuxtApp = map[string]string{
	"package.json": `{"dependencies":{"nuxt":"^3.12.0"}}`,
	"server/api/subscribe.post.ts": `
export default defineEventHandler(async (event) => {
  const { email } = await readBody(event)
  return { ok: true, email }
})`,
	"server/api/stats.get.ts": `export default defineEventHandler(() => ({ users: 1 }))`,
	"pages/index.vue": `
<script setup lang="ts">
const email = ref('')
async function subscribe() {
  await $fetch('/api/subscribe', { method: 'POST', body: { email: email.value } })
}
</script>
<template><button @click="subscribe">Join</button></template>`,
	"pages/pricing.vue": `<template><div>Pricing</div></template>`,
}

func TestNuxtServerRouteCalledFromAComponentIsConnected(t *testing.T) {
	check := sweepProject(t, nuxtApp)
	r := check("`POST /api/subscribe` subscribes an email")
	if r.Verdict != Working {
		t.Fatalf("want CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
	if len(r.Path) == 0 || !strings.Contains(r.Path[0], "page /") {
		t.Errorf("the path should start at the page that makes the call: %v", r.Path)
	}
}

func TestNuxtRouteNothingCallsIsHalfBuilt(t *testing.T) {
	check := sweepProject(t, nuxtApp)
	if r := check("`GET /api/stats` returns the user count"); r.Verdict != HalfBuilt {
		t.Fatalf("want HALF BUILT, got %s: %s", r.Verdict, r.Reason)
	}
}

func TestNuxtPageUrlIsFoundAndAMissingOneIsNot(t *testing.T) {
	check := sweepProject(t, nuxtApp)
	// A named URL claims a connection, so a static page is HALF BUILT as in Next.js;
	// what matters here is that the page is found at all.
	if r := check("Add a `/pricing` page"); !strings.Contains(r.Reason, "page /pricing exists (pages") {
		t.Errorf("the Nuxt page should be found, got %s: %s", r.Verdict, r.Reason)
	}
	if r := check("Add a `/careers` page"); r.Verdict != NotBuilt {
		t.Errorf("want NOT BUILT for the missing page, got %s: %s", r.Verdict, r.Reason)
	}
}

func TestVueComponentNameInALineIsFound(t *testing.T) {
	files := map[string]string{
		"package.json":                 `{"dependencies":{"vue":"^3"}}`,
		"src/components/CartBadge.vue": "<template><span/></template>",
	}
	check := sweepProject(t, files)
	if r := check("Show the count in `CartBadge.vue`"); r.Verdict == NotBuilt {
		t.Fatalf("the component file exists: %s", r.Reason)
	}
}

var nestApp = map[string]string{
	"package.json": `{"dependencies":{"@nestjs/core":"^10","@nestjs/common":"^10","next":"14"}}`,
	"api/src/main.ts": `
import { NestFactory } from '@nestjs/core'
async function bootstrap() {
  const app = await NestFactory.create(AppModule)
  app.setGlobalPrefix('api')
  await app.listen(3000)
}`,
	"api/src/cats/cats.controller.ts": `
import { Controller, Get, Post, Delete, Body } from '@nestjs/common'
@Controller('cats')
export class CatsController {
  @Post() create(@Body() dto: CreateCatDto) { return {} }
  @Get() findAll() { return [] }
  @Delete(':id') remove() { return {} }
}`,
	"web/src/app/page.tsx": `
export default function Page() {
  const add = () => fetch('/api/cats', { method: 'POST', body: JSON.stringify({ name: 'x' }) })
  const load = () => fetch('/api/cats')
  return <button onClick={add}>Add</button>
}`,
}

func TestNestControllerRoutesAreReachedByTheFrontendThroughTheGlobalPrefix(t *testing.T) {
	check := sweepProject(t, nestApp)
	for _, line := range []string{"`POST /api/cats` adds a cat", "`GET /api/cats` lists the cats"} {
		if r := check(line); r.Verdict != Working {
			t.Errorf("%q: want CONNECTED, got %s: %s", line, r.Verdict, r.Reason)
		}
	}
}

func TestNestRouteNothingCallsIsHalfBuiltAndMissingOneIsNotBuilt(t *testing.T) {
	check := sweepProject(t, nestApp)
	if r := check("`DELETE /api/cats/:id` removes a cat"); r.Verdict != HalfBuilt {
		t.Errorf("want HALF BUILT, got %s: %s", r.Verdict, r.Reason)
	}
	if r := check("`PUT /api/cats/:id` renames a cat"); r.Verdict != NotBuilt {
		t.Errorf("want NOT BUILT, got %s: %s", r.Verdict, r.Reason)
	}
}

var svelteKitApp = map[string]string{
	"package.json": `{"devDependencies":{"@sveltejs/kit":"^2.0.0"}}`,
	"src/routes/api/todos/+server.ts": `
import { json } from '@sveltejs/kit'
export const GET = () => json([])
export async function POST({ request }) { return json(await request.json(), { status: 201 }) }`,
	"src/routes/api/todos/[id]/+server.ts": `export const DELETE = () => new Response(null, { status: 204 })`,
	"src/routes/+page.svelte": `
<script>
  async function add() { await fetch('/api/todos', { method: 'POST', body: JSON.stringify({ title: 'x' }) }) }
  async function load() { return fetch('/api/todos') }
</script>
<button on:click={add}>Add</button>`,
	"src/routes/login/+page.server.ts":  `export const actions = { default: async () => ({ ok: true }) }`,
	"src/routes/login/+page.svelte":     `<form method="POST"><button>Log in</button></form>`,
	"src/routes/logout/+page.server.ts": `export const actions = { default: async () => ({ ok: true }) }`,
	"src/routes/logout/+page.svelte":    `<p>Bye</p>`,
}

func TestSvelteKitEndpointCalledFromAPageIsConnected(t *testing.T) {
	check := sweepProject(t, svelteKitApp)
	if r := check("`POST /api/todos` creates a todo"); r.Verdict != Working {
		t.Fatalf("want CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
}

func TestSvelteKitFormActionReachedByItsOwnForm(t *testing.T) {
	check := sweepProject(t, svelteKitApp)
	if r := check("`POST /login` signs the user in"); r.Verdict != Working {
		t.Fatalf("a page's own form posting to its actions is connected, got %s: %s", r.Verdict, r.Reason)
	}
	if r := check("`POST /logout` signs the user out"); r.Verdict != HalfBuilt {
		t.Fatalf("an action no form posts to is HALF BUILT, got %s: %s", r.Verdict, r.Reason)
	}
}

func TestSvelteKitEndpointNothingCallsAndMissingMethod(t *testing.T) {
	check := sweepProject(t, svelteKitApp)
	if r := check("`DELETE /api/todos/:id` deletes a todo"); r.Verdict != HalfBuilt {
		t.Errorf("want HALF BUILT, got %s: %s", r.Verdict, r.Reason)
	}
	if r := check("`PUT /api/todos` replaces the todos"); r.Verdict != NotBuilt {
		t.Errorf("want NOT BUILT, got %s: %s", r.Verdict, r.Reason)
	}
}

var trpcApp = map[string]string{
	"package.json": `{"dependencies":{"@trpc/server":"^11","@trpc/react-query":"^11","next":"15"}}`,
	"src/server/api/trpc.ts": `
import { initTRPC } from "@trpc/server"
const t = initTRPC.create()
export const createTRPCRouter = t.router
export const publicProcedure = t.procedure`,
	"src/server/api/routers/post.ts": `
import { createTRPCRouter, publicProcedure } from "../trpc"
export const postRouter = createTRPCRouter({
  hello: publicProcedure.query(() => "hi"),
  create: publicProcedure.input(Object).mutation(async () => ({ ok: true })),
  getLatest: publicProcedure.query(() => null),
})`,
	"src/server/api/root.ts": `
import { postRouter } from "./routers/post"
import { createTRPCRouter } from "./trpc"
export const appRouter = createTRPCRouter({ post: postRouter })`,
	"src/app/api/trpc/[trpc]/route.ts": `
import { fetchRequestHandler } from "@trpc/server/adapters/fetch"
const handler = (req: Request) => fetchRequestHandler({ endpoint: "/api/trpc", req, router: appRouter, createContext: () => ({}) })
export { handler as GET, handler as POST }`,
	"src/trpc/react.tsx": `
import { createTRPCReact } from "@trpc/react-query"
export const api = createTRPCReact()`,
	"src/app/page.tsx": `
"use client"
import { api } from "~/trpc/react"
export default function Page() {
  const latest = api.post.getLatest.useQuery()
  const create = api.post.create.useMutation()
  return <button onClick={() => create.mutate({ name: "x" })}>go</button>
}`,
}

func TestTRPCProcedureNamedInALineIsCheckedAsItsUrl(t *testing.T) {
	check := sweepProject(t, trpcApp)
	r := check("The `post.create` mutation saves a post")
	if r.Verdict != Working {
		t.Fatalf("want CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
	if !strings.Contains(strings.Join(r.Path, " | "), "/api/trpc/post.create") {
		t.Errorf("the path should name the procedure's URL: %v", r.Path)
	}
}

func TestTRPCProcedureNothingCallsIsHalfBuilt(t *testing.T) {
	check := sweepProject(t, trpcApp)
	if r := check("The `post.hello` query returns a greeting"); r.Verdict != HalfBuilt {
		t.Fatalf("want HALF BUILT, got %s: %s", r.Verdict, r.Reason)
	}
}

func TestTRPCMissingProcedureInAnExistingRouterIsNotBuilt(t *testing.T) {
	check := sweepProject(t, trpcApp)
	r := check("The `post.remove` mutation deletes a post")
	if r.Verdict != NotBuilt || !strings.Contains(r.Reason, "has no `remove` procedure") || !strings.Contains(r.Reason, "create") {
		t.Fatalf("want NOT BUILT naming what the router has, got %s: %s", r.Verdict, r.Reason)
	}
}

func TestTRPCClientPrefixIsIgnoredInALine(t *testing.T) {
	check := sweepProject(t, trpcApp)
	if r := check("`api.post.getLatest` shows the latest post"); r.Verdict != Working {
		t.Fatalf("want CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
}

func TestTRPCUrlWrittenInTheLineWorksToo(t *testing.T) {
	check := sweepProject(t, trpcApp)
	if r := check("`POST /api/trpc/post.create` stores a post"); r.Verdict != Working {
		t.Fatalf("want CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
}

func TestTRPCNotAnotherLibrarysDottedName(t *testing.T) {
	files := map[string]string{}
	for k, v := range trpcApp {
		files[k] = v
	}
	files["prisma/schema.prisma"] = "model Post {\n  id Int @id\n}\n"
	check := sweepProject(t, files)
	r := check("Each post is saved with `prisma.post.create`")
	if r.Verdict == NotBuilt && strings.Contains(r.Reason, "procedure") {
		t.Fatalf("prisma.post.create is a Prisma call, not a tRPC procedure: %s", r.Reason)
	}
}

var astroApp = map[string]string{
	"package.json": `{"dependencies":{"astro":"^4.15.0","react":"^18"}}`,
	"src/pages/api/subscribe.ts": `
export const POST = async ({ request }) => new Response(JSON.stringify({ ok: true }))
export const GET = () => new Response("x")`,
	"src/pages/index.astro": `
---
---
<form id="f"><input name="email" /></form>
<script>
  document.getElementById("f").addEventListener("submit", async () => {
    await fetch("/api/subscribe", { method: "POST", body: "{}" })
  })
</script>`,
	"src/actions/index.ts": `
import { defineAction } from "astro:actions"
export const server = {
  like: defineAction({ handler: async () => 1 }),
  share: defineAction({ handler: async () => 1 }),
}`,
	"src/components/Like.tsx": `
import { actions } from "astro:actions"
export function Like() { return <button onClick={() => actions.like()}>like</button> }`,
}

func TestAstroEndpointCalledFromAPageIsConnected(t *testing.T) {
	check := sweepProject(t, astroApp)
	if r := check("`POST /api/subscribe` stores an email"); r.Verdict != Working {
		t.Fatalf("want CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
	if r := check("`GET /api/subscribe` returns the list"); r.Verdict != HalfBuilt {
		t.Fatalf("want HALF BUILT (nothing calls the GET), got %s: %s", r.Verdict, r.Reason)
	}
}

func TestAstroActionsAreCheckedByName(t *testing.T) {
	check := sweepProject(t, astroApp)
	if r := check("The `actions.like` action records a like"); r.Verdict != Working {
		t.Errorf("a called action is CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
	if r := check("The `actions.share` action shares a post"); r.Verdict != HalfBuilt {
		t.Errorf("an action nothing calls is HALF BUILT, got %s: %s", r.Verdict, r.Reason)
	}
}

var remixApp = map[string]string{
	"package.json": `{"dependencies":{"@remix-run/react":"^2","@remix-run/node":"^2"}}`,
	"app/routes/_index.tsx": `
import { Form } from "@remix-run/react"
export default function Index() {
  return <Form method="post" action="/subscribe"><button>Join</button></Form>
}`,
	"app/routes/subscribe.tsx": `
export async function action({ request }) { return null }`,
	"app/routes/health.tsx": `
export async function loader() { return Response.json({ ok: true }) }`,
	"app/routes/notes.$id.tsx": `
import { Form } from "@remix-run/react"
export async function action() { return null }
export default function Note() { return <Form method="post"><button>Save</button></Form> }`,
}

func TestRemixActionPostedByAFormIsConnected(t *testing.T) {
	check := sweepProject(t, remixApp)
	if r := check("`POST /subscribe` stores an email"); r.Verdict != Working {
		t.Fatalf("want CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
	if r := check("`POST /notes/:id` saves a note"); r.Verdict != Working {
		t.Fatalf("a page's own form posting to its action is CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
}

func TestRemixResourceRouteNothingCallsIsHalfBuilt(t *testing.T) {
	check := sweepProject(t, remixApp)
	if r := check("`GET /health` returns ok"); r.Verdict != HalfBuilt {
		t.Fatalf("want HALF BUILT, got %s: %s", r.Verdict, r.Reason)
	}
}

// A server that serves pages through routes can also serve a plain file; a URL
// such a file could answer is never called missing.
func TestPageUrlAStaticFileMayServeIsNotCalledMissing(t *testing.T) {
	files := map[string]string{
		"server.js": `
const app = require('express')()
app.get('/pricing', (req, res) => res.send('p'))
app.use(require('express').static('public'))`,
		"public/about.html": "<h1>About</h1>",
	}
	check := sweepProject(t, files)
	if r := check("Add an `/about` page"); r.Verdict == NotBuilt {
		t.Errorf("public/about.html may serve /about: %s", r.Reason)
	}
	if r := check("Add a `/careers` page"); r.Verdict != NotBuilt {
		t.Errorf("nothing could serve /careers: %s %s", r.Verdict, r.Reason)
	}
}

// Found running the real binary on trpc/examples-next-prisma-starter: a query
// reported "inserts into table" because its router file also holds a mutation, and
// the adapter's catch-all route stood in for the procedure.
func TestTRPCProcedureOnlyReportsItsOwnWrites(t *testing.T) {
	files := map[string]string{
		"package.json":         `{"dependencies":{"@trpc/server":"^11","@trpc/react-query":"^11","next":"15"}}`,
		"prisma/schema.prisma": "model Post {\n  id Int @id\n}\n",
		"src/server/routers/post.ts": `
import { initTRPC } from "@trpc/server"
import { prisma } from "~/server/prisma"
const t = initTRPC.create()
export const appRouter = t.router({
  post: t.router({
    list: t.procedure.query(async () => {
      // the user's posts, we'll page them later
      return prisma.post.findMany()
    }),
    add: t.procedure.mutation(async () => {
      return prisma.post.create({ data: {} })
    }),
  }),
})`,
		"src/server/prisma.ts": "import { PrismaClient } from '@prisma/client'\nexport const prisma = new PrismaClient()\n",
		"src/pages/api/trpc/[trpc].ts": `
import { createNextApiHandler } from "@trpc/server/adapters/next"
export default createNextApiHandler({ router: appRouter })`,
		"src/pages/index.tsx": `
import { trpc } from "~/utils/trpc"
export default function Home() {
  const list = trpc.post.list.useQuery()
  const add = trpc.post.add.useMutation()
  return null
}`,
		"src/components/Wrong.tsx": `
import { trpc } from "~/utils/trpc"
export function Wrong() {
  const wrong = trpc.post.list.useMutation()
  return null
}`,
		"src/utils/trpc.ts": `
import { createTRPCNext } from "@trpc/next"
export const trpc = createTRPCNext({})`,
	}
	check := sweepProject(t, files)
	add := check("The `post.add` mutation creates a post")
	if !strings.Contains(strings.Join(add.Path, " | "), "inserts into table `Post`") {
		t.Errorf("post.add inserts: %v", add.Path)
	}
	list := check("The `post.list` query lists the posts")
	if strings.Contains(strings.Join(list.Path, " | "), "inserts into") {
		t.Errorf("post.list only reads; the insert belongs to post.add: %v", list.Path)
	}
	if wrong := check("`src/components/Wrong.tsx` loads the posts"); wrong.Verdict != Broken {
		t.Errorf("a component calling the query post.list as a mutation: want BROKEN, got %s: %s", wrong.Verdict, wrong.Reason)
	}
}

// Found on remix-run/indie-stack: the page's own <Form> sits in the same file as
// its action and was read as the action calling another server.
func TestRemixActionIsNotCreditedWithItsOwnPagesForm(t *testing.T) {
	files := map[string]string{
		"package.json": `{"dependencies":{"@remix-run/react":"^2"}}`,
		"app/routes/notes.new.tsx": `
import { Form } from "@remix-run/react"
export async function action({ request }) { return null }
export default function NewNote() { return <Form method="post"><button>Save</button></Form> }`,
	}
	check := sweepProject(t, files)
	r := check("`POST /notes/new` creates a note")
	if strings.Contains(strings.Join(r.Path, " | "), "passes the request on") {
		t.Errorf("the page's own form is not an outbound call of the action: %v", r.Path)
	}
}

func TestRemixPageAnswersGetEvenWhenItsRouteOnlyListsActions(t *testing.T) {
	files := map[string]string{
		"package.json": `{"dependencies":{"@remix-run/react":"^2"}}`,
		"app/routes/notes.$id.tsx": `
export async function loader() { return null }
export async function action() { return null }
export default function Note() { return null }`,
	}
	check := sweepProject(t, files)
	r := check("`GET /notes/archive` shows archived notes")
	if strings.Contains(r.Reason, "no GET handler") {
		t.Errorf("a page route answers GET: %s", r.Reason)
	}
}
