package graph

import "strings"

// TableUses is every database table the project uses or declares, by one lowercase
// name each (a second name for the same table, such as a Prisma @@map, is folded into
// the first), with the places it appears: each call that names it, and the schema
// file that declares it (line 1; a migration has no single line worth pointing at).
// Calls that name no literal table, database functions and storage buckets are not
// tables.
func (g *Graph) TableUses() map[string][]Loc {
	canon := func(name string) string {
		name = strings.ToLower(name)
		if a, ok := g.TableAlias[name]; ok {
			return a
		}
		return name
	}
	out := map[string][]Loc{}
	for _, n := range g.Nodes {
		if n.Kind == DataCall && n.Method == "TABLE" && n.Path != "" {
			name := canon(n.Path)
			out[name] = append(out[name], Loc{File: n.File, Line: n.Line})
		}
	}
	for name, file := range g.Tables {
		name = canon(name)
		out[name] = append(out[name], Loc{File: file, Line: 1})
	}
	return out
}
