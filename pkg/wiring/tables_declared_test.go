package wiring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
)

// 109 open-source projects: a table declared in a migration or a Prisma schema but not yet read by any
// code was reported NOT BUILT ("isn't anywhere in the code"), 28 times out of 385 real tables.
func TestADeclaredButUnusedTableIsNotNotBuilt(t *testing.T) {
	dir := t.TempDir()
	w := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	w("supabase/migrations/001.sql", "CREATE TABLE IF NOT EXISTS coexistence.fathom_config (id int);\n")
	w("prisma/schema.prisma", "model Verification {\n  id Int @id\n}\n")
	w("src/index.js", "console.log('hi')\n")
	g, err := extractor.Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fathom_config", "Verification"} {
		fs := []features.Feature{{ID: "x", Name: "`" + name + "` table stores the records"}}
		for _, r := range Run(g, fs) {
			if r.Verdict == NotBuilt {
				t.Errorf("%s is declared in the schema: %s", name, r.Reason)
			}
			if !strings.Contains(strings.ToLower(r.Reason), strings.ToLower(name)) {
				t.Errorf("the reason should name %s: %s", name, r.Reason)
			}
		}
	}
	// and a name that is nowhere is still NOT BUILT
	fs := []features.Feature{{ID: "y", Name: "`ghost_table` table stores the records"}}
	if rs := Run(g, fs); len(rs) != 1 || rs[0].Verdict != NotBuilt {
		t.Errorf("a missing table must stay NOT BUILT, got %+v", rs)
	}
}

// pern-tasks / trek2.0: a screen that calls through an axios instance (baseURL ".../api") is connected to
// the route /api/signin. It must not be BROKEN with "no server route handles that".
func TestAScreenCallingThroughAnAxiosInstanceIsNotBroken(t *testing.T) {
	dir := t.TempDir()
	w := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	w("server/app.js", "const express = require('express')\nconst app = express()\napp.post('/api/signin', (req, res) => res.json({}))\n")
	w("front/api/axios.js", "import axios from 'axios'\nexport default axios.create({ baseURL: 'http://localhost:3000/api' })\n")
	w("front/AuthContext.jsx", "import axios from '../api/axios'\nexport function AuthProvider() {\n  const signin = (d) => axios.post('/signin', d)\n  return null\n}\n")
	g, err := extractor.Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	fs := []features.Feature{{ID: "a", Name: "`AuthProvider` component renders the view"}}
	for _, r := range Run(g, fs) {
		if r.Verdict == Broken {
			t.Errorf("a call through the axios instance reaches /api/signin: %s", r.Reason)
		}
	}
}
