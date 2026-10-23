package extractor

import "testing"

// A barrel file re-exports a default as a name: export { default as X } from './X'.
// The scanner used to record nothing for it (found 2026-10-12: `index.ts exports
// OrderModal` read as unconnected), so every component reached only through a
// barrel looked unimported.
func TestScanJSImportsDefaultAsName(t *testing.T) {
	cases := []struct {
		src  string
		want []rawImport
	}{
		{`export { default as OrderModal } from './OrderModal'`, []rawImport{{spec: "./OrderModal"}}},
		{`import { default as Card } from '@/components/Card'`, []rawImport{{spec: "@/components/Card"}}},
		{`export { default as A, helper } from './x'`, []rawImport{{name: "helper", spec: "./x"}, {spec: "./x"}}},
		// Unchanged shapes.
		{`import Card, { helper as h } from './Card'`, []rawImport{{name: "helper", spec: "./Card"}, {spec: "./Card"}}},
		{`import { a } from './x'`, []rawImport{{name: "a", spec: "./x"}}},
		{`export * from './all'`, []rawImport{{spec: "./all"}}},
	}
	for _, c := range cases {
		got := scanJSImports(c.src)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %+v, want %+v", c.src, got, c.want)
			continue
		}
		for i := range got {
			want := c.want[i]
			want.line = 1 // every source above is one line, and an import now records its line
			if got[i] != want {
				t.Errorf("%s: got %+v, want %+v", c.src, got, c.want)
				break
			}
		}
	}
}
