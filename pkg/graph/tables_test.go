package graph

import "testing"

// Which tables a project uses or declares, by one name each: a Prisma model and the
// table it is mapped to are the same table, and the case of a name never matters.
func TestTableUsesNamesEachTableOnceWhateverItIsCalled(t *testing.T) {
	g := &Graph{
		Nodes: []Node{
			{Kind: DataCall, File: "src/a.ts", Line: 3, Method: "TABLE", Path: "Profiles", Op: "select"},
			{Kind: DataCall, File: "src/b.ts", Line: 9, Method: "TABLE", Path: "user_profiles", Op: "insert"},
			{Kind: DataCall, File: "src/b.ts", Line: 11, Method: "TABLE", Path: "", Op: "insert"}, // not a literal name
			{Kind: DataCall, File: "src/b.ts", Line: 12, Method: "RPC", Path: "do_thing"},         // not a table
			{Kind: DataCall, File: "src/b.ts", Line: 13, Method: "STORAGE", Path: "avatars"},      // a bucket, not a table
			{Kind: NetworkCall, File: "src/b.ts", Line: 14, Method: "GET", Path: "/api/profiles"}, // not a database call
		},
		Tables:     map[string]string{"profiles": "db/001.sql", "userprofile": "prisma/schema.prisma"},
		TableAlias: map[string]string{"user_profiles": "userprofile"},
	}
	uses := g.TableUses()
	if len(uses) != 2 {
		t.Fatalf("want 2 tables (profiles, userprofile), got %v", uses)
	}
	if len(uses["profiles"]) != 2 || len(uses["userprofile"]) != 2 {
		t.Fatalf("each table lists where it is used and where it is declared: %v", uses)
	}
	declared := uses["profiles"][1]
	if declared.File != "db/001.sql" || declared.Line != 1 {
		t.Errorf("the declaration should point at the schema file: %+v", declared)
	}
}
