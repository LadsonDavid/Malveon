package extractor

import (
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

const nuxtPkg = `{"dependencies":{"nuxt":"^3.12.0","vue":"^3.4.0"}}`

func TestNuxtServerRoutesFollowTheirFilesAndMethodSuffix(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json":                         nuxtPkg,
		"server/api/hello.get.ts":              `export default defineEventHandler(() => 'hi')`,
		"server/api/users/index.post.ts":       `export default defineEventHandler(async (e) => readBody(e))`,
		"server/api/users/[id].ts":             `export default defineEventHandler((e) => getRouterParam(e, 'id'))`,
		"server/api/files/[...slug].delete.ts": `export default defineEventHandler(() => 1)`,
		"server/routes/health.ts":              `export default eventHandler(() => 'ok')`,
		"server/utils/helper.ts":               `export function help() { return 1 }`,
		"server/middleware/log.ts":             `export default defineEventHandler(() => {})`,
	})
	wantRoutes(t, g,
		"GET /api/hello", "POST /api/users", "ANY /api/users/:id",
		"DELETE /api/files/:...slug", "ANY /health")
	for _, r := range routesOf(g) {
		if r == "ANY /api/utils/helper" || r == "ANY /middleware/log" || r == "ANY /api/helper" {
			t.Errorf("utils and middleware are not routes: %v", routesOf(g))
		}
	}
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && !n.FileRouted {
			t.Errorf("a Nuxt route comes from its file location: %+v", n)
		}
	}
}

func TestServerFolderOutsideNuxtIsNotRoutes(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json":      `{"dependencies":{"express":"^4"}}`,
		"server/api/foo.ts": `export default function handler() {}`,
	})
	if got := routesOf(g); len(got) != 0 {
		t.Errorf("server/api in an Express project is just a folder: %v", got)
	}
}

func TestNuxtPages(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json":              nuxtPkg,
		"pages/index.vue":           `<template><div/></template>`,
		"pages/about.vue":           `<template><div/></template>`,
		"pages/users/[id].vue":      `<template><div/></template>`,
		"pages/users/index.vue":     `<template><div/></template>`,
		"pages/(auth)/login.vue":    `<template><div/></template>`,
		"pages/docs/[...slug].vue":  `<template><div/></template>`,
		"components/pages/Card.vue": `<template><div/></template>`,
	})
	pages := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind == graph.Page {
			pages[n.Path] = true
		}
	}
	for _, w := range []string{"/", "/about", "/users/:id", "/users", "/login", "/docs/:...slug?"} {
		if !pages[w] {
			t.Errorf("missing page %q; have %v", w, pages)
		}
	}
	if len(pages) != 6 {
		t.Errorf("a components/pages folder is not the pages folder: %v", pages)
	}
}

func TestNuxt4AppDirectory(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json":        nuxtPkg,
		"app/pages/index.vue": `<template><div/></template>`,
		"app/pages/cart.vue":  `<template><div/></template>`,
	})
	if len(g.PagesAt("/cart")) != 1 || len(g.PagesAt("/")) != 1 {
		t.Errorf("Nuxt 4 keeps pages in app/pages: %v", g.Nodes)
	}
}

func TestVueSingleFileComponentsMakeCalls(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": nuxtPkg,
		"components/Form.vue": `
<script setup lang="ts">
const id = ref(1)
const hello = await $fetch('/api/hello')
const { data } = await useFetch('/api/users', { method: 'POST', body: { name: 'x', email: 'e' } })
const { data: u } = await useFetch(() => ~/api/users/${id.value}~)
const lazy = useLazyFetch('/api/lazy')
</script>
<template>
  <form action="/api/contact" method="post"><button>go</button></form>
</template>
<style scoped>
.a { background: url("/api/not-a-call.png"); }
</style>`,
	})
	calls := map[string]graph.Node{}
	for _, n := range g.Nodes {
		if n.Kind == graph.NetworkCall && n.Confidence == graph.Extracted {
			calls[n.Method+" "+n.Path] = n
		}
	}
	for _, w := range []string{"GET /api/hello", "POST /api/users", "GET /api/users/:param", "GET /api/lazy", "POST /api/contact"} {
		if _, ok := calls[w]; !ok {
			t.Errorf("missing call %q; have %v", w, calls)
		}
	}
	if n := calls["POST /api/users"]; len(n.BodyFields) != 2 {
		t.Errorf("the body object's fields are the request fields: %+v", n)
	}
	if _, ok := calls["GET /api/not-a-call.png"]; ok {
		t.Errorf("CSS in a style block is not a call")
	}
}

func TestVueFileNeverRegistersServerRoutes(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json":  nuxtPkg,
		"src/Thing.vue": `<script>const cache = new Map(); cache.get('/api/x'); app.post('/api/y', h)</script>`,
	})
	if got := routesOf(g); len(got) != 0 {
		t.Errorf("a component does not register server routes: %v", got)
	}
}

func TestVueComponentIsFoundByItsFileName(t *testing.T) {
	g := extractProject(t, map[string]string{
		"src/components/CartButton.vue": `<template><button/></template>`,
		"src/App.vue": `<script setup>
import CartButton from './components/CartButton.vue'
</script>`,
	})
	if got := g.FindFiles("CartButton.vue"); len(got) != 1 {
		t.Fatalf("component file should exist in the graph: %v", g.Files)
	}
	if imps := g.ImportersOf(g.FindFiles("CartButton.vue")[0]); len(imps) != 1 {
		t.Errorf("App.vue imports the component: %v", g.Imports)
	}
}
