package graph

import (
	"path/filepath"
	"strings"
)

// SourceRole is provenance, not a claim that code ran. A root selected inside
// testdata is scanned normally because its relative paths no longer include it.
func SourceRole(file string) string {
	p := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	parts := strings.Split(p, "/")
	for _, s := range parts[:len(parts)-1] {
		switch s {
		case "testdata", "__fixtures__", "__mocks__":
			return "fixture"
		case "__tests__", "cypress", "e2e":
			return "test"
		}
	}
	b := filepath.Base(p)
	if strings.HasSuffix(b, "_test.go") || strings.HasSuffix(b, "_test.py") || strings.HasPrefix(b, "test_") && strings.HasSuffix(b, ".py") || b == "conftest.py" {
		return "test"
	}
	for _, s := range []string{".test.", ".spec.", ".cy."} {
		if strings.Contains(b, s) {
			return "test"
		}
	}
	return "production"
}
