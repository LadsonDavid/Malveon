package extractor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// extractProject writes files (~ stands for a backtick) into a temp project and
// runs the real extractor over it.
func extractProject(t *testing.T, files map[string]string) *graph.Graph {
	t.Helper()
	root := t.TempDir()
	for rel, src := range files {
		writeTestFile(t, root, rel, strings.ReplaceAll(src, "~", "`"))
	}
	g, err := Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func dataCalls(g *graph.Graph) map[string]bool {
	out := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind == graph.DataCall {
			out[n.Method+" "+n.Path+" "+n.Op] = true
		}
	}
	return out
}

func wantData(t *testing.T, g *graph.Graph, want ...string) {
	t.Helper()
	got := dataCalls(g)
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing data call %q; have %v", w, got)
		}
	}
}

func wantNoData(t *testing.T, g *graph.Graph) {
	t.Helper()
	if got := dataCalls(g); len(got) != 0 {
		t.Errorf("expected no data calls, got %v", got)
	}
}

const prismaSchema = `
model User {
  id    Int    @id
  posts Post[]
}

model UserProfile {
  id Int @id
  @@map("user_profiles")
}

model Post {
  id Int @id
}
`

func TestPrismaCallsAreReadThroughTheSchema(t *testing.T) {
	g := extractProject(t, map[string]string{
		"prisma/schema.prisma": prismaSchema,
		"src/app/api/posts/route.ts": `
import { prisma } from "@/lib/prisma"
export async function GET() {
  return Response.json(await prisma.post.findMany())
}
export async function POST(req: Request) {
  const body = await req.json()
  const post = await prisma.post.create({ data: body })
  await prisma.userProfile.upsert({ where: { id: 1 }, create: {}, update: {} })
  await prisma.user.deleteMany()
  await prisma.user.updateMany({ data: {} })
  return Response.json(post)
}`,
	})
	wantData(t, g,
		"TABLE Post select", "TABLE Post insert",
		"TABLE UserProfile upsert", "TABLE User delete", "TABLE User update")
}

func TestPrismaServiceNamedLikeAModelAccessorIsNotATable(t *testing.T) {
	// this.catsService.create(dto) has the same two-dot shape as prisma.cat.create(...).
	g := extractProject(t, map[string]string{
		"prisma/schema.prisma": prismaSchema,
		"src/cats/cats.controller.ts": `
export class CatsController {
  create(dto) { return this.catsService.create(dto) }
  remove(id) { return this.usersRepository.delete(id) }
  list() { return this.catsService.findMany() }
}`,
	})
	wantNoData(t, g)
}

func TestPrismaWithoutASchemaNeedsAPrismaLookingClient(t *testing.T) {
	g := extractProject(t, map[string]string{
		"src/a.ts": `
import { PrismaClient } from "@prisma/client"
const prisma = new PrismaClient()
export const a = () => prisma.invoice.findMany()
export const b = () => store.invoice.findMany()
export const c = () => this.invoiceService.create({})`,
	})
	wantData(t, g, "TABLE invoice select")
	if len(dataCalls(g)) != 1 {
		t.Errorf("only the call on the prisma client counts: %v", dataCalls(g))
	}
}

func TestPrismaRawSQLIsClassified(t *testing.T) {
	g := extractProject(t, map[string]string{
		"prisma/schema.prisma": prismaSchema,
		"src/raw.ts": `
export async function f(prisma, id) {
  await prisma.$queryRaw~SELECT * FROM "User" WHERE id = ${id}~
  await prisma.$executeRaw~UPDATE public.posts SET title = 'x'~
  await prisma.$executeRaw~DELETE FROM "Post"~
  await prisma.$queryRawUnsafe("INSERT INTO audit_log(x) VALUES (1) RETURNING id")
  await prisma.$executeRawUnsafe(sqlFromSomewhere)
}`,
	})
	wantData(t, g,
		"TABLE User select", "TABLE posts update", "TABLE Post delete",
		"TABLE audit_log insert", "SQL sql ")
}

func TestPrismaMappedTableNameIsAnAlias(t *testing.T) {
	g := extractProject(t, map[string]string{
		"prisma/schema.prisma": prismaSchema,
		"src/a.ts":             `export const a = (prisma) => prisma.userProfile.findMany()`,
	})
	if got := g.DataCallsTo("user_profiles"); len(got) != 1 {
		t.Errorf("the mapped table name should find the model's call, got %v", got)
	}
	if got := g.DataCallsTo("userprofile"); len(got) != 1 {
		t.Errorf("lookup ignores case, got %v", got)
	}
	if _, ok := g.Tables["user_profiles"]; !ok {
		t.Errorf("the mapped table is a declared table: %v", g.Tables)
	}
}

func TestPrismaSchemaCanLiveInAnyDotPrismaFile(t *testing.T) {
	g := extractProject(t, map[string]string{
		"prisma/schema/user.prisma": "model Widget {\n  id Int @id\n}\n",
		"src/a.ts":                  `export const a = (db) => db.widget.create({ data: {} })`,
	})
	wantData(t, g, "TABLE Widget insert")
}

const drizzleSchema = `
import { pgTable, serial, text } from "drizzle-orm/pg-core"
export const users = pgTable("user", { id: serial("id").primaryKey() })
export const posts = pgTable("posts", { id: serial("id").primaryKey() })
`

func TestDrizzleBuilderCalls(t *testing.T) {
	g := extractProject(t, map[string]string{
		"src/db/schema.ts": drizzleSchema,
		"src/api.ts": `
import { users, posts } from "./db/schema"
export async function run(db) {
  await db.select().from(users)
  await db.insert(posts).values({ title: "x" })
  await db
    .update(posts)
    .set({ title: "y" })
  await db.delete(users)
  await db.insert(users).values({}).onConflictDoUpdate({ target: users.id, set: {} })
  const r = await db.query.posts.findMany({ with: { author: true } })
}`,
	})
	wantData(t, g,
		"TABLE user select", "TABLE posts insert", "TABLE posts update",
		"TABLE user delete", "TABLE user upsert", "TABLE posts select")
}

func TestDrizzleOnlyKnownTablesCount(t *testing.T) {
	g := extractProject(t, map[string]string{
		"src/db/schema.ts": drizzleSchema,
		"src/other.ts": `
export function f(store, list, item) {
  store.update(item)
  list.delete(item)
  cache.insert(item)
  return db.select().from(somethingElse)
}`,
	})
	wantNoData(t, g)
}

func TestDrizzleTableCreatorAndSchemaObject(t *testing.T) {
	g := extractProject(t, map[string]string{
		"src/server/db/schema.ts": `
import { pgTableCreator, pgSchema } from "drizzle-orm/pg-core"
export const createTable = pgTableCreator((name) => ~app_${name}~)
export const posts = createTable("post", {})
const auth = pgSchema("auth")
export const sessions = auth.table("sessions", {})`,
		"src/x.ts": `
export const f = (db) => { db.insert(posts).values({}); db.delete(sessions) }`,
	})
	wantData(t, g, "TABLE post insert", "TABLE sessions delete")
}

func TestDrizzleExecuteRawSQL(t *testing.T) {
	g := extractProject(t, map[string]string{
		"src/db/schema.ts": drizzleSchema,
		"src/x.ts": `
export const f = async (db) => {
  await db.execute(sql~DELETE FROM posts WHERE id = ${1}~)
  await db.execute(sql~SELECT * FROM "user"~)
}`,
	})
	wantData(t, g, "TABLE posts delete", "TABLE user select")
}

func TestDrizzleTablesJoinTheDeclaredTables(t *testing.T) {
	g := extractProject(t, map[string]string{"src/db/schema.ts": drizzleSchema})
	for _, name := range []string{"user", "posts"} {
		if _, ok := g.Tables[name]; !ok {
			t.Errorf("drizzle table %q should be a declared table: %v", name, g.Tables)
		}
	}
	if g.TableAlias["users"] != "user" {
		t.Errorf("the JS name users should alias the table user: %v", g.TableAlias)
	}
}

func TestORMCallInACommentOrStringIsNotACall(t *testing.T) {
	g := extractProject(t, map[string]string{
		"prisma/schema.prisma": prismaSchema,
		"src/db/schema.ts":     drizzleSchema,
		"src/a.ts": `
// prisma.post.create({ data: {} })
const doc = "db.insert(posts).values({})"
/* await prisma.user.delete() */`,
	})
	wantNoData(t, g)
}

func TestPrismaOpsClassification(t *testing.T) {
	cases := map[string]string{
		"findUnique": "select", "findUniqueOrThrow": "select", "findFirst": "select", "findFirstOrThrow": "select",
		"findMany": "select", "count": "select", "aggregate": "select", "groupBy": "select",
		"create": "insert", "createMany": "insert", "createManyAndReturn": "insert",
		"update": "update", "updateMany": "update", "updateManyAndReturn": "update",
		"upsert": "upsert", "delete": "delete", "deleteMany": "delete",
	}
	for op, want := range cases {
		g := extractProject(t, map[string]string{
			"prisma/schema.prisma": prismaSchema,
			"src/a.ts":             "export const a = (prisma) => prisma.post." + op + "({})",
		})
		wantData(t, g, "TABLE Post "+want)
	}
}

var _ = os.Stdout
var _ = filepath.Join
