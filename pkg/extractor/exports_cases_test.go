package extractor

import "testing"

func exportSetOf(src string) (names map[string]bool, open bool) {
	s := scanJSExports(src)
	return s.Names, s.Open
}

// Found by running malveon over 145 projects: three ways a real export was not read.
func TestExportsReadFromRealWorldFiles(t *testing.T) {
	cases := []struct {
		name, src, want string
		openOK          bool // an open file (every name) is an acceptable reading
	}{
		{"minified bundle ends with an export list", "var a=1;function f(){};export{f as resolveDefaultSessionDir,a as other};\n", "resolveDefaultSessionDir", false},
		{"export list after a semicolon on a long line", "const x=1;export { x as renamed };", "renamed", false},
		{"a regex literal with brackets before a second declarator", `export const SGR_RE = /\x1b\[[0-9;:]*m/gu, RESET = '\x1b[0m';` + "\n", "RESET", true},
		{"a byte order mark before the first export", "\ufeffexport const notFound = (req) => 1;\nexport const errorHandler = () => 2;\n", "notFound", false},
	}
	for _, c := range cases {
		names, open := exportSetOf(c.src)
		if !names[c.want] && !open {
			t.Errorf("%s: %q not read as exported (names %v)", c.name, c.want, names)
		}
		if !c.openOK && !names[c.want] {
			t.Errorf("%s: should be read by name, got open=%v names=%v", c.name, open, names)
		}
	}
}

// An import that really is missing must still be reported: the fix must not make every file open.
func TestAnOrdinaryFileStillHasAnExactExportList(t *testing.T) {
	names, open := exportSetOf("export const a = 1;\nexport function b() {}\nconst c = { x: 1, y: 2 };\n")
	if open || names["c"] || !names["a"] || !names["b"] {
		t.Fatalf("exact list expected, open=%v names=%v", open, names)
	}
}
