package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

func TestImportsAndDataOps(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"src/app/page.tsx": `import Card, { helper as h } from '@/components/Card'
import { useState } from 'react'
import * as lib from '../lib/db'
import './globals.css'
import { gone } from '@/lib/gone'`,
		"src/components/Card.tsx": `export default function Card() {}
export function helper() {}`,
		"src/lib/db.ts": `export const q = supabase
  .from('jobs')
  .insert({ a: 1 })
export const r = supabase.from('jobs').select('*')
export const s = supabase.storage.from('avatars').upload('x', f)
const query = supabase.from('later')
export const u = supabase.auth.signOut()`,
		"src/app/globals.css": `body {}`,
		// a TypeScript type reference names one thing, not the whole file
		"src/app/types.ts": `type P = import("@/components/Card").Props`,
	}
	for rel, src := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(src), 0o644)
	}
	g, err := Extract(root)
	if err != nil {
		t.Fatal(err)
	}

	page := filepath.FromSlash("src/app/page.tsx")
	got := map[string]bool{}
	for _, im := range g.Imports[page] {
		got[filepath.ToSlash(im.File)+"#"+im.Name+"#"+im.Missing] = true
	}
	for _, want := range []string{
		"src/components/Card.tsx##",       // default import: the whole file
		"src/components/Card.tsx#helper#", // named, by its defined name (not the alias)
		"src/lib/db.ts##",                 // namespace import, relative path
		"src/app/globals.css##",           // side-effect import
		"#gone#@/lib/gone",                // a project path with no file: kept as missing
	} {
		if !got[want] {
			t.Errorf("missing import %q in %v", want, got)
		}
	}
	if len(g.Imports[page]) != 5 {
		t.Errorf("got %d imports, want 5 (the react package import is never kept): %v", len(g.Imports[page]), g.Imports[page])
	}
	if imp := g.ImportersOf(filepath.FromSlash("src/components/Card.tsx")); len(imp) != 2 {
		t.Errorf("ImportersOf(Card.tsx) = %v, want page.tsx and types.ts", imp)
	}
	if got := g.Imports[filepath.FromSlash("src/app/types.ts")]; len(got) != 1 || got[0].Name != "Props" {
		t.Errorf("import(\"...\").Props = %v, want one import named Props", got)
	}

	ops := map[string]string{}
	for _, n := range g.Nodes {
		if n.Kind == graph.DataCall {
			ops[n.Method+":"+n.Path+":"+n.Op] = "y"
		}
	}
	for _, want := range []string{
		"TABLE:jobs:insert", // chained across lines
		"TABLE:jobs:select",
		"STORAGE:avatars:upload",
		"TABLE:later:", // not chained directly: operation unknown
		"AUTH:signOut:signOut",
	} {
		if ops[want] == "" {
			t.Errorf("missing data call %q in %v", want, ops)
		}
	}
}
