package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// The tables a project declares: create table / create view in .sql files
// (migrations), model in schema.prisma. The code graph uses them to tell a
// table the project defines from a name that is one typo away from one
// (found by audit, 2026-10-10: .from('ordres') against a table named orders
// read CONNECTED).
var (
	sqlCreate = regexp.MustCompile(`(?is)\bcreate\s+(?:or\s+replace\s+)?(?:temp(?:orary)?\s+|unlogged\s+|materialized\s+)?(?:table|view)\s+(?:if\s+not\s+exists\s+)?(?:["\x60\[]?\w+["\x60\]]?\.)?["\x60\[]?(\w+)`)
	sqlRename = regexp.MustCompile(`(?is)\balter\s+table\s+(?:if\s+exists\s+)?(?:["\x60\[]?\w+["\x60\]]?\.)?["\x60\[]?\w+["\x60\]]?\s+rename\s+to\s+["\x60\[]?(\w+)`)
)

// readSchema records the tables a .sql or schema.prisma file declares.
func readSchema(g *graph.Graph, rel, ext, src string) {
	add := func(name string) {
		if g.Tables == nil {
			g.Tables = map[string]string{}
		}
		g.Tables[strings.ToLower(name)] = rel
	}
	if ext == ".sql" {
		for _, m := range sqlCreate.FindAllStringSubmatch(stripSQLComments(src), -1) {
			add(m[1])
		}
		for _, m := range sqlRename.FindAllStringSubmatch(stripSQLComments(src), -1) {
			add(m[1])
		}
		return
	}
	for _, m := range parsePrismaModels(src) {
		add(m.name)
		if m.mapped != "" {
			add(m.mapped)
			addTableAlias(g, m.mapped, m.name)
		}
	}
}

// addTableAlias lets a table be found by a second name (see graph.TableAlias).
func addTableAlias(g *graph.Graph, alias, canonical string) {
	if g.TableAlias == nil {
		g.TableAlias = map[string]string{}
	}
	g.TableAlias[strings.ToLower(alias)] = strings.ToLower(canonical)
}

var sqlLineComment = regexp.MustCompile(`--[^\n]*`)
var sqlBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

func stripSQLComments(src string) string {
	return sqlBlockComment.ReplaceAllString(sqlLineComment.ReplaceAllString(src, " "), " ")
}
