package extractor

import (
	"strings"
	"testing"
)

func TestCodeMap(t *testing.T) {
	cases := []struct {
		name, src, token string
		sx               syntax
		code             bool // is the first byte of token code?
	}{
		{"real call", "fetch('/x')", "fetch", jsSyntax, true},
		{"line comment", "// fetch('/x')\n", "fetch", jsSyntax, false},
		{"block comment", "/* a\n fetch('/x') */", "fetch", jsSyntax, false},
		{"inside a string", "const m = \"call fetch('/x')\";", "fetch", jsSyntax, false},
		{"the string argument itself is not code", "fetch('/api/x')", "/api/x", jsSyntax, false},
		{"after a comment line", "// note\nfetch('/x')", "fetch", jsSyntax, true},
		{"inside a template literal", "const m = `see fetch('/x') ${a}`;", "fetch", jsSyntax, false},
		{"nested template", "const m = `a ${b ? `c` : 'd'} e`; fetch('/x')", "fetch", jsSyntax, true},
		{"regex literal holding a quote", "const r = /['\"]/g; fetch('/x')", "fetch", jsSyntax, true},
		{"division is not a regex", "const a = b / c; fetch('/x') // d / e", "fetch", jsSyntax, true},
		{"apostrophe in JSX text only costs its line", "<p>Don't</p>\nfetch('/x')", "fetch", jsSyntax, true},
		{"after a JSX closing tag", "const a = <i>x</i>; const b = () => fetch('/api/x')", "fetch", jsSyntax, true},
		{"after a self-closing JSX tag", "const c = <Foo bar={1}/>; const d = () => fetch('/api/x')", "fetch", jsSyntax, true},
		{"a real regex still hides what is inside it", "const r = /fetch('x')/; ok()", "fetch", jsSyntax, false},
		{"python hash comment", "# requests.get('/x')\n", "requests", pySyntax, false},
		{"python docstring", "\"\"\"\n@app.get('/x')\n\"\"\"\n", "app", pySyntax, false},
		{"python decorator", "@app.get('/x')\n", "app", pySyntax, true},
		{"rust lifetime is not a string", "fn f<'a>(x: &'a str) {}\n#[get(\"/x\")]\n", "get", rustSyntax, true},
		{"php hash comment", "# Route::get('/x');\n", "Route", phpSyntax, false},
		{"ruby comment", "  # get '/x'\n", "get", hashSyntax, false},
	}
	for _, c := range cases {
		m := newCodeMap(c.src, c.sx)
		at := strings.Index(c.src, c.token)
		if at < 0 {
			t.Fatalf("%s: token %q not in source", c.name, c.token)
		}
		if got := m.codeAt(c.src, at); got != c.code {
			t.Errorf("%s: %q code=%v, want %v", c.name, c.token, got, c.code)
		}
	}
}
