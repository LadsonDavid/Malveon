package extractor

import (
	"os"
	"path/filepath"
	"testing"
)

// A package inside a monorepo (newtab/) with its own jsconfig baseUrl "src" writes
// `import { Button } from 'components/Elements'`. That is newtab/src/components/Elements, not a
// file at the repository root, and the closest package wins over a distant one.
func TestNestedPackageAliasImportsResolve(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"newtab/src/features/Modals.jsx":           "import { Button } from 'components/Elements';\nimport u from '@/utils/u';\nexport const M = Button;\n",
		"newtab/src/components/Elements/index.jsx": "export * from './Button';\n",
		"newtab/src/components/Elements/Button.jsx": "export const Button = 1;\n",
		"newtab/src/utils/u.js":                     "export default 1;\n",
		"other/src/components/Elements/index.jsx":   "export const Other = 1;\n",
		"other/src/x.jsx":                           "import { Other } from 'components/Elements';\nexport const X = Other;\n",
		"newtab/src/p.jsx":                          "import React from 'react';\nexport const P = React;\n",
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
	find := func(file, spec string) string {
		for f, imps := range g.Imports {
			if filepath.ToSlash(f) != file {
				continue
			}
			for _, im := range imps {
				if im.Spec == spec {
					return filepath.ToSlash(im.File)
				}
			}
		}
		return ""
	}
	if got := find("newtab/src/features/Modals.jsx", "components/Elements"); got != "newtab/src/components/Elements/index.jsx" {
		t.Fatalf("bare alias in a nested package: got %q", got)
	}
	if got := find("newtab/src/features/Modals.jsx", "@/utils/u"); got != "newtab/src/utils/u.js" {
		t.Fatalf("@/ alias in a nested package: got %q", got)
	}
	if got := find("other/src/x.jsx", "components/Elements"); got != "other/src/components/Elements/index.jsx" {
		t.Fatalf("closest package must win: got %q", got)
	}
	if got := find("newtab/src/p.jsx", "react"); got != "" {
		t.Fatalf("a package must stay a package: got %q", got)
	}
}
