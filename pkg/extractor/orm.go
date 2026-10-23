package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// Prisma and Drizzle, read the way the Supabase client already is: a call to a
// table becomes a DataCall node whose Path is the table and whose Op says what
// it does (select, insert, update, upsert, delete). Everything downstream —
// "this screen talks to the database", "this route changes nothing" — then works
// for these two ORMs with no change of its own.
//
// Both are read only when the code proves what it is calling:
//   - Prisma: the accessor in `client.accessor.findMany()` must be a model the
//     project's .prisma files declare. `this.catsService.create(dto)` has the same
//     shape and is not a table. With no .prisma file anywhere, the client itself
//     must look like Prisma's (named prisma/db/tx, in a file that mentions Prisma).
//   - Drizzle: the table in `db.insert(posts)` must be a variable the project
//     declares with pgTable/mysqlTable/sqliteTable/... (`store.update(item)` is not).

// drizzleTable is one table a Drizzle schema declares.
type drizzleTable struct {
	sql  string // the table name in the database (the string passed to pgTable)
	file string
}

type prismaModel struct {
	name   string
	mapped string // @@map("...") — the table's real name, "" when it has none
}

var (
	prismaBlock  = regexp.MustCompile(`(?m)^[ \t]*(?:model|view)[ \t]+(\w+)[ \t]*\{`)
	prismaMapAt  = regexp.MustCompile(`@@map\(\s*(?:name:\s*)?"([^"]+)"\s*\)`)
	drizzleDecl  = regexp.MustCompile(`(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*([A-Za-z_$][\w$.]*)\s*\(\s*['"]([\w-]+)['"]`)
	drizzleMaker = regexp.MustCompile(`(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:\w+\.)?(?:pg|mysql|sqlite|singlestore|mssql)TableCreator\s*\(`)
	drizzleKinds = map[string]bool{"pgTable": true, "mysqlTable": true, "sqliteTable": true, "singlestoreTable": true, "mssqlTable": true}
)

// parsePrismaModels lists the models (and views) a .prisma file declares.
func parsePrismaModels(src string) []prismaModel {
	var out []prismaModel
	for _, m := range prismaBlock.FindAllStringSubmatchIndex(src, -1) {
		pm := prismaModel{name: src[m[2]:m[3]]}
		body := readBalanced(src, m[1]-1)
		if mm := prismaMapAt.FindStringSubmatch(body); mm != nil {
			pm.mapped = mm[1]
		}
		out = append(out, pm)
	}
	return out
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// addPrisma records the models of one .prisma file: the client reaches a model
// named UserProfile as `prisma.userProfile`.
func (in *jsInfo) addPrisma(src string) {
	if in.prismaModels == nil {
		in.prismaModels = map[string]string{}
	}
	for _, m := range parsePrismaModels(src) {
		in.prismaModels[lowerFirst(m.name)] = m.name
	}
}

// addDrizzle records the tables a TypeScript/JavaScript file declares with
// Drizzle. Only a file that imports drizzle-orm is looked at, so a
// `db.table("x")` of some other library is never a Drizzle table.
func (in *jsInfo) addDrizzle(file, src string, cm codeMap) {
	if !strings.Contains(src, "drizzle-orm") {
		return
	}
	makers := map[string]bool{}
	for _, m := range drizzleMaker.FindAllStringSubmatchIndex(src, -1) {
		if cm.codeAt(src, m[0]) {
			makers[src[m[2]:m[3]]] = true
		}
	}
	for _, m := range drizzleDecl.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		callee := src[m[4]:m[5]]
		if !drizzleKinds[callee] && !makers[callee] && !strings.HasSuffix(callee, ".table") {
			continue
		}
		if in.drizzleTables == nil {
			in.drizzleTables = map[string]drizzleTable{}
		}
		in.drizzleTables[src[m[2]:m[3]]] = drizzleTable{sql: src[m[6]:m[7]], file: file}
	}
}

// registerDrizzleTables makes every table a Drizzle schema declares a table the
// project defines, and lets the JS variable that holds it stand for the table.
func registerDrizzleTables(g *graph.Graph, info *jsInfo) {
	if info == nil {
		return
	}
	for ident, t := range info.drizzleTables {
		if g.Tables == nil {
			g.Tables = map[string]string{}
		}
		g.Tables[strings.ToLower(t.sql)] = t.file
		if !strings.EqualFold(ident, t.sql) {
			addTableAlias(g, ident, t.sql)
		}
	}
}

const prismaOpAlt = `(findUniqueOrThrow|findUnique|findFirstOrThrow|findFirst|findMany|createManyAndReturn|createMany|create|` +
	`updateManyAndReturn|updateMany|update|upsert|deleteMany|delete|count|aggregate|groupBy)`

var (
	// Prisma client calls: client.model.op(
	prismaCall = regexp.MustCompile(`([A-Za-z_$][\w$]*(?:\s*\??\.\s*[A-Za-z_$][\w$]*)*)\s*\??\.\s*([A-Za-z_$][\w$]*)\s*\??\.\s*` + prismaOpAlt + `\s*\(`)
	// ...reached through a call: getDb().post.create(
	prismaAfterCall = regexp.MustCompile(`\)\s*\??\.\s*([A-Za-z_$][\w$]*)\s*\??\.\s*` + prismaOpAlt + `\s*\(`)
	// ...with the model chosen at runtime: prisma[model].create(, (prisma as any)[model].create(
	prismaDynamic = regexp.MustCompile(`\(?\s*\b(prisma|db|tx|trx|client|prismaClient)\b(?:\s+as\s+\w+)?\s*\)?\s*\[[^\]\n]+\]\s*\.\s*` + prismaOpAlt + `\s*\(`)
)

var prismaOps = map[string]string{
	"findUnique": "select", "findUniqueOrThrow": "select", "findFirst": "select", "findFirstOrThrow": "select",
	"findMany": "select", "count": "select", "aggregate": "select", "groupBy": "select",
	"create": "insert", "createMany": "insert", "createManyAndReturn": "insert",
	"update": "update", "updateMany": "update", "updateManyAndReturn": "update",
	"upsert": "upsert", "delete": "delete", "deleteMany": "delete",
}

// With no schema to check against, only a client named like Prisma's, in a
// file that mentions Prisma, counts.
var prismaClientNames = map[string]bool{"prisma": true, "db": true, "tx": true, "trx": true, "client": true, "prismaclient": true, "database": true}

// $queryRaw`...`, $executeRaw(...), $queryRaw<T>`...`
var prismaRaw = regexp.MustCompile(`\$(queryRawUnsafe|queryRaw|executeRawUnsafe|executeRaw)\b\s*(?:<[^>(\x60]*>)?\s*[(\x60]`)

// Drizzle builder calls, on a variable the schema declares.
var (
	drizzleWrite = regexp.MustCompile(`\.\s*(insert|update|delete)\s*\(\s*([A-Za-z_$][\w$]*)\s*\)`)
	drizzleFrom  = regexp.MustCompile(`\.\s*from\s*\(\s*([A-Za-z_$][\w$]*)\s*\)`)
	drizzleQuery = regexp.MustCompile(`\.\s*query\s*\.\s*([A-Za-z_$][\w$]*)\s*\.\s*(?:findMany|findFirst)\s*\(`)
	drizzleExec  = regexp.MustCompile(`\.\s*(?:execute|run|all|values)\s*\(\s*sql\s*\x60`)
	// any .execute( — those that aren't followed by sql` are statements built elsewhere
	drizzleExecAny = regexp.MustCompile(`\.\s*execute\s*\(`)
	drizzleCount   = regexp.MustCompile(`\.\s*\$count\s*\(\s*([A-Za-z_$][\w$]*)`)
)

// scanORM finds Prisma and Drizzle calls in one JavaScript/TypeScript file.
func scanORM(src string, cm codeMap, info *jsInfo) []callSite {
	var sites []callSite
	table := func(off int, name, op string) {
		sites = append(sites, callSite{kind: graph.DataCall, method: "TABLE", line: lineOf(src, off), argRaw: name, op: op})
	}

	mentionsPrisma := strings.Contains(src, "PrismaClient") || strings.Contains(src, "@prisma/client") || strings.Contains(strings.ToLower(src), "prisma")
	for _, m := range prismaCall.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		chain, accessor, op := src[m[2]:m[3]], src[m[4]:m[5]], prismaOps[src[m[6]:m[7]]]
		var name string
		switch {
		case info != nil && info.prismaModels[accessor] != "":
			name = info.prismaModels[accessor]
		case info != nil && len(info.prismaModels) == 0 && mentionsPrisma && prismaClientNames[strings.ToLower(lastIdent(chain))]:
			name = accessor
		default:
			continue
		}
		table(m[0], name, op)
	}
	if info != nil && len(info.prismaModels) > 0 {
		for _, m := range prismaAfterCall.FindAllStringSubmatchIndex(src, -1) {
			if !cm.codeAt(src, m[0]) {
				continue
			}
			if model := info.prismaModels[src[m[2]:m[3]]]; model != "" {
				table(m[0], model, prismaOps[src[m[4]:m[5]]])
			}
		}
	}
	if mentionsPrisma {
		for _, m := range prismaDynamic.FindAllStringIndex(src, -1) {
			if cm.codeAt(src, m[0]) {
				sites = append(sites, callSite{kind: graph.DataCall, method: "SQL", line: lineOf(src, m[0]), argRaw: "a model chosen at runtime"})
			}
		}
	}
	for _, m := range prismaRaw.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		sites = append(sites, rawSQLSite(src, m[1]-1, m[0]))
	}

	if info == nil || len(info.drizzleTables) == 0 {
		return sites
	}
	for _, m := range drizzleWrite.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue
		}
		t, ok := info.drizzleTables[src[m[4]:m[5]]]
		if !ok {
			continue
		}
		op := src[m[2]:m[3]]
		if op == "insert" && strings.Contains(statementAfter(src, m[1]), ".onConflictDoUpdate") {
			op = "upsert"
		}
		table(m[0], t.sql, op)
	}
	for _, m := range drizzleFrom.FindAllStringSubmatchIndex(src, -1) {
		if t, ok := info.drizzleTables[src[m[2]:m[3]]]; ok && cm.codeAt(src, m[0]) {
			table(m[0], t.sql, "select")
		}
	}
	for _, m := range drizzleCount.FindAllStringSubmatchIndex(src, -1) {
		if t, ok := info.drizzleTables[src[m[2]:m[3]]]; ok && cm.codeAt(src, m[0]) {
			table(m[0], t.sql, "select")
		}
	}
	for _, m := range drizzleQuery.FindAllStringSubmatchIndex(src, -1) {
		if t, ok := info.drizzleTables[src[m[2]:m[3]]]; ok && cm.codeAt(src, m[0]) {
			table(m[0], t.sql, "select")
		}
	}
	for _, m := range drizzleExec.FindAllStringIndex(src, -1) {
		if cm.codeAt(src, m[0]) {
			sites = append(sites, rawSQLSite(src, m[1]-1, m[0]))
		}
	}
	// db.execute(someVariable): a statement built somewhere else.
	for _, m := range drizzleExecAny.FindAllStringIndex(src, -1) {
		if cm.codeAt(src, m[0]) && !drizzleExec.MatchString(src[m[0]:min(m[1]+8, len(src))]) {
			sites = append(sites, callSite{kind: graph.DataCall, method: "SQL", line: lineOf(src, m[0]), argRaw: "raw SQL built elsewhere"})
		}
	}
	return sites
}

func lastIdent(chain string) string {
	if i := strings.LastIndexAny(chain, ". "); i >= 0 {
		return chain[i+1:]
	}
	return chain
}

// statementAfter is the rest of a builder chain: this line and every following
// line that carries on with a `.method()`, up to a semicolon. A chain written
// without semicolons ends where the next line does not start with a dot.
func statementAfter(src string, from int) string {
	end := from
	for end < len(src) {
		nl := strings.IndexByte(src[end:], '\n')
		if nl < 0 {
			end = len(src)
			break
		}
		end += nl + 1
		if strings.HasPrefix(strings.TrimLeft(src[end:min(end+80, len(src))], " \t\r"), ".") {
			continue
		}
		break
	}
	s := src[from:end]
	if i := strings.IndexByte(s, ';'); i >= 0 {
		s = s[:i]
	}
	return s
}

// rawSQLSite turns a raw SQL call into a data call: the statement's own verb and
// first table when they can be read, else "raw SQL" with an unknown effect.
func rawSQLSite(src string, open, at int) callSite {
	op, tbl := classifySQL(rawSQLText(src, open))
	if tbl != "" && op != "" && op != "write" {
		return callSite{kind: graph.DataCall, method: "TABLE", line: lineOf(src, at), argRaw: tbl, op: op}
	}
	return callSite{kind: graph.DataCall, method: "SQL", line: lineOf(src, at), argRaw: "sql", op: op}
}

// rawSQLText is the SQL text of a tagged template or a call whose first
// argument is a string, starting at the backtick or parenthesis at open; ""
// when it is built from something else.
func rawSQLText(src string, open int) string {
	if open >= len(src) {
		return ""
	}
	if src[open] == '`' {
		end := skipTemplate(src, open)
		if end-1 > open+1 && end-1 <= len(src) {
			return src[open+1 : end-1]
		}
		return ""
	}
	arg := readFirstArg(src, open)
	arg = strings.TrimPrefix(arg, "Prisma.sql")
	arg = strings.TrimPrefix(arg, "sql")
	if len(arg) >= 2 && (arg[0] == '`' || arg[0] == '"' || arg[0] == '\'') && arg[len(arg)-1] == arg[0] {
		return arg[1 : len(arg)-1]
	}
	return ""
}

var (
	sqlTable  = `(?:only\s+)?((?:["\x60\[]?\w+["\x60\]]?\.)*["\x60\[]?\w+["\x60\]]?)`
	sqlLead   = regexp.MustCompile(`(?is)^\s*(select|insert|update|delete|with|merge|truncate|alter|create|drop)\b`)
	sqlInsert = regexp.MustCompile(`(?is)\binsert\s+into\s+` + sqlTable)
	sqlUpdate = regexp.MustCompile(`(?is)^\s*update\s+` + sqlTable)
	sqlDelete = regexp.MustCompile(`(?is)^\s*delete\s+from\s+` + sqlTable)
	sqlFrom   = regexp.MustCompile(`(?is)\bfrom\s+` + sqlTable)
	sqlWrites = regexp.MustCompile(`(?is)\b(insert|update|delete)\b`)
)

// classifySQL reads a statement's verb and first table. op is one of select,
// insert, update, delete, "write" (DDL or merge), or "" when the statement can't
// be read; table is "" when none can be named (an interpolated name).
func classifySQL(sql string) (op, table string) {
	sql = stripSQLComments(sql)
	lead := sqlLead.FindStringSubmatch(sql)
	if lead == nil {
		return "", ""
	}
	var m []string
	switch verb := strings.ToLower(lead[1]); verb {
	case "select":
		op, m = "select", sqlFrom.FindStringSubmatch(sql)
	case "with":
		// A data-modifying CTE writes; any other WITH ... SELECT reads.
		if sqlWrites.MatchString(sql) {
			return "", ""
		}
		op, m = "select", sqlFrom.FindStringSubmatch(sql)
	case "insert":
		op, m = "insert", sqlInsert.FindStringSubmatch(sql)
	case "update":
		op, m = "update", sqlUpdate.FindStringSubmatch(sql)
	case "delete":
		op, m = "delete", sqlDelete.FindStringSubmatch(sql)
	default:
		return "write", ""
	}
	if m == nil {
		return op, ""
	}
	t := strings.Trim(m[1], "\"`[]")
	if i := strings.LastIndex(t, "."); i >= 0 {
		t = strings.Trim(t[i+1:], "\"`[]")
	}
	return op, t
}
