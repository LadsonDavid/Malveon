package wiring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
)

// A plan line that names no URL/file/component used to be a vague-line verdict
// even when nothing in the code resembles it at all. When no code
// mentions any of its meaningful words, the true answer is NOT BUILT, and
// the reason names the words it looked for (founder's call, 2026-09-29).
// Only when some code does mention them, but not strongly enough to tell
// which, is it SKIPPED as unchecked (it never blocks).
func TestLinesNothingMentionsAreNotBuilt(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "orders.js"), []byte(
		"export function OrderList() { return fetch('/api/orders') }\nexport function formatTotal(order) { return order.total }\n"), 0o644)
	g, err := extractor.Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	check := func(line string) Result { return Run(g, []features.Feature{{ID: "x", Name: line}})[0] }

	for _, c := range []struct {
		line      string
		want      Verdict
		reasonHas string
	}{
		{"Refund button", NotBuilt, "refund"},
		{"Make checkout feel faster", NotBuilt, "checkout"},
		{"Invoices can be exported", NotBuilt, "invoices"},
		// "total" appears in code, but nothing strongly matches the line: can't tell which, so unchecked.
		{"Show the total nicely", Skipped, ""},
		// Says something should NOT exist: code not mentioning it may mean it is already true.
		{"No refund banners remain on the checkout", Skipped, ""},
		// Only generic words: nothing to look for, unchecked.
		{"Make it better", Skipped, ""},
	} {
		r := check(c.line)
		if r.Verdict != c.want {
			t.Errorf("%q: got %s (%s), want %s", c.line, r.Verdict, r.Reason, c.want)
			continue
		}
		if c.want == Skipped && (!r.Unchecked || r.Reason != ReasonNamesNothing) {
			t.Errorf("%q: a line malveon can't look up must be unchecked with the standard reason, got %v %q", c.line, r.Unchecked, r.Reason)
		}
		if c.reasonHas != "" && !strings.Contains(r.Reason, c.reasonHas) {
			t.Errorf("%q: reason should name %q: %s", c.line, c.reasonHas, r.Reason)
		}
	}
}
