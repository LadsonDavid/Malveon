package extractor

import (
	"os"
	"path/filepath"
	"testing"
)

// Graph.Uses is the proof behind "nothing in the app ever calls this function".
// A wrong 0 would tell someone their working button is dead, so every way a name
// can be used without being written next to a call is a case here, and each one
// must count as a use. The unused cases must be 0.
func TestGraphUses(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		fn    string
		used  bool
	}{
		{"declared and never mentioned again",
			map[string]string{"a.ts": "export async function doRefund() { await fetch('/api/x'); }\n"}, "doRefund", false},
		{"called in the same file",
			map[string]string{"a.ts": "export function doRefund() {}\nexport function run() { doRefund(); }\n"}, "doRefund", true},
		{"named in a JSX handler",
			map[string]string{"a.tsx": "export function doRefund() {}\nexport default function B() { return <button onClick={doRefund}>x</button>; }\n"}, "doRefund", true},
		{"imported by another file and never used there",
			map[string]string{
				"lib/api.ts": "export async function doRefund() {}\n",
				"b.tsx":      "import { doRefund } from './lib/api';\nexport default function B() { return <button>x</button>; }\n"}, "doRefund", false},
		{"imported and called",
			map[string]string{
				"lib/api.ts": "export async function doRefund() {}\n",
				"b.tsx":      "import { doRefund } from './lib/api';\nexport default function B() { return <button onClick={() => doRefund()}>x</button>; }\n"}, "doRefund", true},
		{"imported under another name (an alias hides the call)",
			map[string]string{
				"lib/api.ts": "export async function doRefund() {}\n",
				"b.tsx":      "import { doRefund as refund } from './lib/api';\nexport default function B() { return <button onClick={() => refund()}>x</button>; }\n"}, "doRefund", true},
		{"imported over several lines and never used",
			map[string]string{
				"lib/api.ts": "export async function doRefund() {}\nexport async function other() {}\n",
				"b.tsx":      "import {\n  other,\n  doRefund,\n} from './lib/api';\nexport const x = other;\n"}, "doRefund", false},
		{"default export may be imported under any name",
			map[string]string{"a.tsx": "export default function Refund() { return null; }\n"}, "Refund", true},
		{"export default name",
			map[string]string{"a.tsx": "function Refund() { return null; }\nexport default Refund;\n"}, "Refund", true},
		{"only listed in an export list",
			map[string]string{"a.ts": "function doRefund() {}\nexport { doRefund };\n"}, "doRefund", false},
		{"re-exported by name from another file only",
			map[string]string{
				"lib/api.ts":   "export async function doRefund() {}\n",
				"lib/index.ts": "export { doRefund } from './api';\n"}, "doRefund", false},
		{"re-exported under another name",
			map[string]string{
				"lib/api.ts":   "export async function doRefund() {}\n",
				"lib/index.ts": "export { doRefund as refund } from './api';\n"}, "doRefund", true},
		{"used through a namespace import",
			map[string]string{
				"lib/api.ts": "export async function doRefund() {}\n",
				"b.ts":       "import * as api from './lib/api';\nexport const go = () => api.doRefund();\n"}, "doRefund", true},
		{"used from a plain HTML page",
			map[string]string{
				"a.js":       "function doRefund() {}\n",
				"index.html": "<button onclick=\"doRefund()\">x</button>\n"}, "doRefund", true},
		{"used from a Vue template",
			map[string]string{
				"a.ts":  "export function doRefund() {}\n",
				"B.vue": "<template><button @click=\"doRefund\">x</button></template>\n<script setup>\nimport { doRefund } from './a';\n</script>\n"}, "doRefund", true},
		{"only a comment mentions it (counts as used: errs toward used)",
			map[string]string{"a.ts": "// call doRefund later\nexport function doRefund() {}\n"}, "doRefund", true},
		{"assigned to a const arrow and never used",
			map[string]string{"a.tsx": "export default function B() {\n  const handleRefund = async () => { await fetch('/api/x'); };\n  return <button>x</button>;\n}\n"}, "handleRefund", false},
		{"module.exports lists it",
			map[string]string{"a.js": "function doRefund() {}\nmodule.exports = { doRefund };\n"}, "doRefund", true},
		{"a longer name that contains it is not a use",
			map[string]string{"a.ts": "export function refund() {}\nexport function refundAll() {}\nexport const x = refundAll;\n"}, "refund", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			for rel, src := range c.files {
				p := filepath.Join(root, filepath.FromSlash(rel))
				os.MkdirAll(filepath.Dir(p), 0o755)
				if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			g, err := Extract(root)
			if err != nil {
				t.Fatal(err)
			}
			n, defined := g.Uses[c.fn]
			if !defined {
				t.Fatalf("%s is defined here but g.Uses has no entry for it: %v", c.fn, g.Uses)
			}
			if c.used && n <= 0 {
				t.Errorf("Uses[%s] = %d, but something uses it", c.fn, n)
			}
			if !c.used && n != 0 {
				t.Errorf("Uses[%s] = %d, but nothing uses it", c.fn, n)
			}
		})
	}
}

// Extract fills Graph.Triggers for JSX files, with the file named.
func TestExtractFillsTriggers(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "src", "components", "RefundButton.tsx")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(inComponent(`<button className="btn">Refund</button>`)), 0o644)
	os.WriteFile(filepath.Join(root, "notes.ts"), []byte("export const x = a < b;\n"), 0o644)
	g, err := Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Triggers) != 1 {
		t.Fatalf("want 1 trigger, got %d: %+v", len(g.Triggers), g.Triggers)
	}
	if got := g.Triggers[0]; got.File != filepath.Join("src", "components", "RefundButton.tsx") || got.Tag != "button" {
		t.Errorf("trigger = %+v", got)
	}
}
