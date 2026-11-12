package extractor

import (
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sameDefs(t *testing.T, label, src string) {
	t.Helper()
	want := defPatterns["js"].FindAllStringSubmatchIndex(src, -1)
	got := scanJSDefs(src)
	if len(want) == 0 && len(got) == 0 {
		return
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%s: the pattern found %v, the scanner %v, in %q", label, want, got, src)
	}
}

// The scanner is checked against the regular expression it replaced: hand-picked awkward text,
// then a great deal of random text built from the pattern's own words.
func TestDefinitionScannerAgreesWithThePattern(t *testing.T) {
	for _, src := range []string{
		"", "function f() {}", "export default function f() {}", "export default async function* g() {}",
		"export  \t const   x = 1;let y=2;var z", "async function a(){}\nasync  function b(){}", "class A {}\nclass\nB {}",
		"const let x", "let letter = 1", "typeof x", "type T = 1\ninterface I {}\nenum E {}\nexport enum F {}",
		"x;function f(){}", ";function f(){}", "a.function f", "function *f", "function* f", "function*f", "export const", "export default",
		"\nfunction f(){}", "\r\nconst a = 1", "\ffunction f(){}", "\vfunction f(){}", "é function f(){}", "日const x = 1",
		"export async default function f(){}", "default function f(){}", "async\nfunction f(){}", "export\nconst x=1",
		"constx = 1", "const_x = 1", "const $a = 1", "const 1a = 1", "function  \n\n  $x_1() {}",
		"export const a = 1; export const b = 2;\nexport function c() {}", "// function f()\nconst g = 1", "'function f'\nlet h",
	} {
		sameDefs(t, "case", src)
	}
	words := []string{"export", "default", "async", "function", "*", "class", "const", "let", "var", "interface", "type", "enum",
		" ", "  ", "\n", "\t", ";", "x", "$a", "_b", "1", "é", "\r", "\f", "\v", "=", "(", "typeof", "letter", "exports"}
	rng := rand.New(rand.NewSource(11))
	for n := 0; n < 20000; n++ {
		var b strings.Builder
		for i := rng.Intn(14); i > 0; i-- {
			b.WriteString(words[rng.Intn(len(words))])
		}
		sameDefs(t, "random", b.String())
	}
}

// With MALVEON_CORPUS set to a folder of real projects, every JavaScript and TypeScript file in
// it must give the same answer from both. (Off by default; the file count is capped.)
func TestDefinitionScannerAgreesOnRealCode(t *testing.T) {
	root := os.Getenv("MALVEON_CORPUS")
	if root == "" {
		t.Skip("set MALVEON_CORPUS to a folder of projects to compare on real code")
	}
	checked := 0
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".vue", ".svelte", ".astro":
		default:
			return nil
		}
		if checked >= 12000 {
			return filepath.SkipAll
		}
		if b, err := os.ReadFile(p); err == nil && len(b) < 4<<20 {
			sameDefs(t, p, string(b))
			checked++
		}
		return nil
	})
	t.Logf("compared %d real files", checked)
	if checked == 0 {
		t.Fatal("no files found")
	}
}
