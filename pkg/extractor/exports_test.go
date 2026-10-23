package extractor

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// A named import whose name the file no longer exports is the commonest way an agent
// "fixes one thing and quietly breaks another" (the founder's point 4): it renames or
// removes an export and leaves a caller behind. These tests pin what is read as an
// export, and what makes a file's exports unknowable (so nothing is ever claimed).

func exportsOf(src string) (string, bool) {
	set := scanJSExports(src)
	var names []string
	for n := range set.Names {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, " "), set.Open
}

func TestScanJSExports(t *testing.T) {
	cases := []struct {
		name, src, names string
		open             bool
	}{
		{"declarations", "export function a() {}\nexport async function b() {}\nexport const c = 1\nexport let d\nexport class E {}\nexport abstract class F {}\n" +
			"export interface G {}\nexport type H = string\nexport enum I {}\nexport const enum J {}\nexport namespace K {}\nexport declare function L(): void\nexport default function m() {}\n",
			"E F G H I J K L a b c d default", false},
		{"a default class", "export default class {}\n", "default", false},
		{"a list", "const x = 1\nexport { x, y as z, w as default }\n", "default x z", false},
		{"re-exports", "export { a, b } from './m'\nexport { default as C, d as D } from './n'\nexport type { T } from './t'\n", "C D T a b", false},
		{"a namespace re-export", "export * as ns from './m'\n", "ns", false},
		{"export star is unknowable", "export * from './m'\nexport const a = 1\n", "a", true},
		{"destructuring is unknowable", "export const { a, b } = obj\n", "", true},
		{"several declarators are unknowable", "export const a = 1, b = 2\n", "a", true},
		{"commas inside brackets are not declarators", "export const f = (x, y) => x + y\nexport const o = { a: 1, b: 2 }\nexport const l = [1, 2]\n", "f l o", false},
		{"commonjs is unknowable", "module.exports = { a: 1 }\n", "", true},
		{"commonjs exports object", "exports.a = 1\n", "", true},
		{"export equals", "export = foo\n", "", true},
		{"comments and strings are not exports", "// export function ghost() {}\nconst s = 'export function str() {}'\n/* export const c = 1 */\nexport function real() {}\n", "real", false},
		{"a line that looks like an export inside a block comment or a template still counts: over-reading an export can only hide a break, never invent one", "/*\nexport const hidden = 1\n*/\nconst t = `\nexport const inString = 2\n`\nexport const shown = 3\n", "hidden inString shown", false},
		{"nothing exported", "const a = 1\nconsole.log(a)\n", "", false},
		{"declare const", "export declare const x: number\n", "x", false},
	}
	for _, c := range cases {
		names, open := exportsOf(c.src)
		if names != c.names || open != c.open {
			t.Errorf("%s:\n  got  names=%q open=%v\n  want names=%q open=%v", c.name, names, open, c.names, c.open)
		}
	}
}

// project writes files under a temp dir and extracts it.
func project(t *testing.T, files map[string]string) *graph.Graph {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g, err := Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func brokenKeys(g *graph.Graph) []string {
	var out []string
	for _, b := range g.BrokenImports() {
		key := filepath.ToSlash(b.File) + ":" + itoa(b.Line) + " " + b.Name
		if b.Missing {
			key = filepath.ToSlash(b.File) + ":" + itoa(b.Line) + " missing " + b.Spec
		}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}
	return string(d)
}

func TestBrokenImports(t *testing.T) {
	g := project(t, map[string]string{
		// the probe's case: formatPrice was renamed, the order list still imports it
		"src/lib/format.js":  "export function fmtPrice(c) { return c }\nexport function formatDate(d) { return d }\n",
		"src/OrderList.jsx":  "import { formatPrice, formatDate } from './lib/format'\nexport default function O() { return null }\n",
		"src/Fine.jsx":       "import { fmtPrice } from './lib/format'\nimport def, { formatDate as fd } from './lib/format'\nexport default function F() { return null }\n",
		"src/types.ts":       "export interface Kept {}\n",
		"src/UsesTypes.ts":   "import type { Kept, Gone } from './types'\nimport { type Kept as K2 } from './types'\n",
		"src/barrel.ts":      "export * from './lib/format'\n",
		"src/UsesBarrel.ts":  "import { anything } from './barrel'\n",
		"src/cjs.js":         "module.exports = { a: 1 }\n",
		"src/UsesCjs.js":     "import { a, b } from './cjs'\n",
		"src/UsesDefault.js": "import Thing from './lib/format'\nimport * as ns from './lib/format'\n",
		"src/Missing.js":     "import x from './gone'\nimport data from './data.json'\nimport s from './s.module.css'\nimport y from '@/lib/gone'\nimport z from 'lodash'\n",
	})
	got := brokenKeys(g)
	want := []string{
		"src/Missing.js:1 missing ./gone",
		"src/OrderList.jsx:1 formatPrice",
		"src/UsesTypes.ts:1 Gone",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("broken imports:\n  got  %q\n  want %q", got, want)
	}
}

func TestBrokenImportsNameWhatIsMissingAndWhereFrom(t *testing.T) {
	g := project(t, map[string]string{
		"a.js": "export const kept = 1\n",
		"b.js": "// header\nimport { kept, lost } from './a'\n",
	})
	bs := g.BrokenImports()
	if len(bs) != 1 {
		t.Fatalf("%+v", bs)
	}
	b := bs[0]
	if b.Name != "lost" || b.Spec != "./a" || filepath.ToSlash(b.Target) != "a.js" || b.Line != 2 || filepath.ToSlash(b.File) != "b.js" {
		t.Errorf("%+v", b)
	}
}

// Found 2026-10-21 measuring the Malveon monorepo: a regex literal holding a quote
// ("/\"/g", as in lib/csv.ts) confuses the code map, which then took every later line for
// a string, and `export function toCsvRow` after it was "not an export". Reading too few
// exports invents a broken import, reading too many only hides one, so exports are read
// from the text, not from the code map.
func TestARegexLiteralHoldingAQuoteDoesNotHideTheNextExport(t *testing.T) {
	src := "export function a(v) {\n  return `\"${v.replace(/\"/g, '\"\"')}\"`;\n}\n\nexport function b() {}\n"
	names, open := exportsOf(src)
	if names != "a b" || open {
		t.Fatalf("names=%q open=%v", names, open)
	}
}

// Found 2026-10-21 measuring on a real project: `import('./x.js').then(...)` was read as "import
// the name `then` from x", the one broken import in 676. `.then`, `.catch`, `.finally`
// are Promise methods and `.default` is the whole module.
func TestPromiseMethodsOnADynamicImportAreNotNames(t *testing.T) {
	g := project(t, map[string]string{
		"lib.ts":  "export const a = 1\n",
		"user.ts": "import('./lib').then((m) => m.a)\nimport('./lib').catch(() => 0)\nimport('./lib').finally(() => 0)\nconst d = import('./lib').default\n",
	})
	if got := brokenKeys(g); len(got) != 0 {
		t.Fatalf("%q", got)
	}
}
