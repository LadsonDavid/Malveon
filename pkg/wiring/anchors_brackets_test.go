package wiring

import "testing"

// A path a person wraps in brackets or parentheses is still a path: "Add numbers [src/add.js]" used
// to read as no file at all, so the line was judged NOT BUILT or not judged.
func TestAnchorsInSeesAPathInsideBrackets(t *testing.T) {
	for _, line := range []string{"Add numbers [src/add.js]", "Add numbers (src/add.js)", "Add numbers (see src/add.js)"} {
		found := false
		for _, a := range anchorsIn(line) {
			if a.kind == fileAnchor && a.text == "src/add.js" {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: no file anchor for src/add.js, got %+v", line, anchorsIn(line))
		}
	}
	for _, a := range anchorsIn("Add numbers (src/add.js") {
		if a.kind == fileAnchor && a.text != "src/add.js" {
			t.Errorf("an unbalanced bracket became part of the name: %q", a.text)
		}
	}
}
