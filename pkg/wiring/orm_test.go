package wiring

import (
	"strings"
	"testing"
)

// Prisma and Drizzle are read like the Supabase client: a route's "Changes" hop
// names the insert, and a route that provably only reads is HALF BUILT when its
// line says it saves something.

var prismaApp = map[string]string{
	"prisma/schema.prisma": "model Post {\n  id Int @id\n}\nmodel User {\n  id Int @id\n}\n",
	"src/lib/prisma.ts":    "import { PrismaClient } from '@prisma/client'\nexport const prisma = new PrismaClient()\n",
	"src/app/api/posts/route.ts": `
import { prisma } from '@/lib/prisma'
export async function GET() { return Response.json(await prisma.post.findMany()) }
export async function POST(req: Request) {
  const body = await req.json()
  return Response.json(await prisma.post.create({ data: body }))
}`,
	"src/app/api/ping/route.ts": `
import { prisma } from '@/lib/prisma'
export async function POST() { return Response.json(await prisma.post.findMany()) }`,
	"src/app/posts/page.tsx": `
export default function Posts() {
  const save = () => fetch('/api/posts', { method: 'POST', body: JSON.stringify({ title: 'x' }) })
  const ping = () => fetch('/api/ping', { method: 'POST' })
  return <button onClick={save}>Save</button>
}`,
}

func TestPrismaRouteThatCreatesIsConnectedAndShowsTheInsert(t *testing.T) {
	check := sweepProject(t, prismaApp)
	r := check("`POST /api/posts` saves the post")
	if r.Verdict != Working {
		t.Fatalf("want CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
	if !strings.Contains(strings.Join(r.Path, " | "), "inserts into table `Post`") {
		t.Errorf("the Changes hop should name the insert, got %v", r.Path)
	}
}

func TestPrismaRouteThatOnlyReadsIsHalfBuiltWhenTheLineSaves(t *testing.T) {
	check := sweepProject(t, prismaApp)
	r := check("`POST /api/ping` saves a ping")
	if r.Verdict != HalfBuilt || !strings.Contains(r.Reason, "never changes any data") {
		t.Fatalf("want HALF BUILT (never changes data), got %s: %s", r.Verdict, r.Reason)
	}
}

func TestPrismaModelNamedInALineIsFoundByModelOrTableName(t *testing.T) {
	check := sweepProject(t, prismaApp)
	for _, line := range []string{
		"Each post is stored with `prisma.post.create` in `src/app/api/posts/route.ts`",
		"The `Post` model is used by `src/app/posts/page.tsx`",
	} {
		if r := check(line); r.Verdict != Working {
			t.Errorf("%q: want CONNECTED, got %s: %s", line, r.Verdict, r.Reason)
		}
	}
}

func TestComponentNamedLikeAModelIsStillAComponent(t *testing.T) {
	files := map[string]string{
		"prisma/schema.prisma":    "model Post {\n  id Int @id\n}\n",
		"src/lib/db.ts":           "export const prisma = new (require('@prisma/client').PrismaClient)()\n",
		"src/components/Post.tsx": "export function Post() { return <div>hello</div> }\n",
		"src/app/api/posts/route.ts": `
import { prisma } from '@/lib/db'
export async function GET() { return Response.json(await prisma.post.findMany()) }`,
	}
	check := sweepProject(t, files)
	r := check("Render `Post` in `src/components/Post.tsx`")
	if strings.Contains(r.Reason, "table") {
		t.Errorf("a component named like a model is a component, not a table: %s", r.Reason)
	}
}

var drizzleApp = map[string]string{
	"src/server/db/index.ts": `
import { drizzle } from "drizzle-orm/node-postgres"
export const db = drizzle(process.env.DATABASE_URL!)`,
	"src/server/db/schema.ts": `
import { pgTable, serial, text } from "drizzle-orm/pg-core"
export const posts = pgTable("post", { id: serial("id").primaryKey(), name: text("name") })`,
	"src/server/queries.ts": `
import { db } from "@/server/db"
import { posts } from "@/server/db/schema"
export async function addPost(name: string) { await db.insert(posts).values({ name }) }`,
	"src/app/api/posts/route.ts": `
import { addPost } from "@/server/queries"
export async function POST(req: Request) { await addPost((await req.json()).name); return Response.json({ ok: true }) }`,
	"src/app/api/count/route.ts": `
import { db } from "@/server/db"
import { posts } from "@/server/db/schema"
export async function POST() { return Response.json(await db.select().from(posts)) }`,
	"src/app/page.tsx": `
export default function Home() {
  const a = () => fetch('/api/posts', { method: 'POST', body: '{}' })
  const b = () => fetch('/api/count', { method: 'POST' })
  return <div />
}`,
}

func TestDrizzleWriteThroughAnImportedHelper(t *testing.T) {
	check := sweepProject(t, drizzleApp)
	r := check("`POST /api/posts` saves a post")
	if r.Verdict != Working || !strings.Contains(strings.Join(r.Path, " | "), "inserts into table `post`") {
		t.Fatalf("want CONNECTED with the insert, got %s: %s %v", r.Verdict, r.Reason, r.Path)
	}
}

func TestDrizzleRouteThatOnlySelectsIsHalfBuiltWhenTheLineSaves(t *testing.T) {
	check := sweepProject(t, drizzleApp)
	if r := check("`POST /api/count` saves a count"); r.Verdict != HalfBuilt {
		t.Fatalf("want HALF BUILT, got %s: %s", r.Verdict, r.Reason)
	}
}

// A database package inside the repo (a workspace package) is code the route
// reaches by its package name. If malveon could not follow it, a route that
// saves through it would look like one that changes nothing.
func TestWorkspacePackageIsFollowed(t *testing.T) {
	files := map[string]string{
		"package.json":              `{"name":"root","workspaces":["apps/*","packages/*"]}`,
		"apps/web/package.json":     `{"name":"web","dependencies":{"@acme/db":"*"}}`,
		"packages/db/package.json":  `{"name":"@acme/db","main":"src/index.ts"}`,
		"packages/db/src/schema.ts": `import { pgTable, serial } from "drizzle-orm/pg-core"` + "\n" + `export const items = pgTable("items", { id: serial("id") })`,
		"packages/db/src/index.ts":  `import { items } from "./schema"` + "\n" + `export async function createItem(db, v) { await db.insert(items).values(v) }`,
		"apps/web/app/api/items/route.ts": `
import { createItem } from "@acme/db"
export async function POST(req: Request) { await createItem(null, await req.json()); return Response.json({}) }`,
		"apps/web/app/page.tsx": `export default function P() { const s = () => fetch('/api/items', { method: 'POST' }); return <div/> }`,
	}
	check := sweepProject(t, files)
	r := check("`POST /api/items` saves an item")
	if r.Verdict != Working {
		t.Fatalf("want CONNECTED, got %s: %s", r.Verdict, r.Reason)
	}
	if !strings.Contains(strings.Join(r.Path, " | "), "inserts into table `items`") {
		t.Errorf("the insert in the workspace package should be the Changes hop: %v", r.Path)
	}
}

func TestDynamicPrismaAccessMeansUnknownNotNoChange(t *testing.T) {
	files := map[string]string{
		"prisma/schema.prisma": "model Post {\n  id Int @id\n}\n",
		"src/lib/prisma.ts":    "import { PrismaClient } from '@prisma/client'\nexport const prisma = new PrismaClient()\n",
		"src/app/api/any/route.ts": `
import { prisma } from '@/lib/prisma'
export async function POST(req: Request) {
  const { model, data } = await req.json()
  await (prisma as any)[model].create({ data })
  return Response.json(await prisma.post.findMany())
}`,
		"src/app/page.tsx": `export default function P() { const s = () => fetch('/api/any', { method: 'POST' }); return <div/> }`,
	}
	check := sweepProject(t, files)
	if r := check("`POST /api/any` saves a record"); r.Verdict == HalfBuilt {
		t.Fatalf("a computed model access may write: not HALF BUILT: %s", r.Reason)
	}
}
