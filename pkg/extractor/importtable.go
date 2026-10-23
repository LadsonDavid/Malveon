package extractor

import (
	"regexp"
	"strings"
)

// importTable maps every name a JavaScript/TypeScript file imports or requires
// to the module it came from:
//
//	import items from './items'             import * as items from './items'
//	import { a, b as c } from './items'     const items = require('./items')
//	const { a, b: c } = require('./items')
//
// It is built once per file, on first use. Looking one name up with fresh
// regexes over the whole file for every app.use(name) call was quadratic: a
// large bundle with tens of thousands of .use( calls took minutes.
func importTable(src string) map[string]string {
	table := map[string]string{}
	local := func(name string) string {
		// "a as b" and "a: b" both mean the local name is the last word
		f := strings.FieldsFunc(name, func(r rune) bool { return r == ' ' || r == ':' || r == '\t' || r == '\n' || r == '\r' })
		if len(f) == 0 {
			return ""
		}
		return f[len(f)-1]
	}
	for _, m := range importDefault.FindAllStringSubmatch(src, -1) {
		table[m[1]] = m[2]
	}
	for _, m := range importNamespace.FindAllStringSubmatch(src, -1) {
		table[m[1]] = m[2]
	}
	for _, m := range importNamed.FindAllStringSubmatch(src, -1) {
		for _, n := range strings.Split(m[1], ",") {
			if l := local(n); l != "" && l != "type" {
				table[l] = m[2]
			}
		}
	}
	for _, m := range requireDefault.FindAllStringSubmatch(src, -1) {
		table[m[1]] = m[2]
	}
	for _, m := range requireDestructured.FindAllStringSubmatch(src, -1) {
		for _, n := range strings.Split(m[1], ",") {
			if l := local(n); l != "" {
				table[l] = m[2]
			}
		}
	}
	return table
}

var (
	importDefault       = regexp.MustCompile(`import\s+([A-Za-z_$][\w$]*)\s*(?:,[^'"{]*)?from\s*['"]([^'"]+)['"]`)
	importNamespace     = regexp.MustCompile(`import\s*\*\s*as\s+([A-Za-z_$][\w$]*)\s+from\s*['"]([^'"]+)['"]`)
	importNamed         = regexp.MustCompile(`import\s*(?:[A-Za-z_$][\w$]*\s*,\s*)?\{([^}]*)\}\s*from\s*['"]([^'"]+)['"]`)
	requireDefault      = regexp.MustCompile(`(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:await\s+)?require\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	requireDestructured = regexp.MustCompile(`(?:const|let|var)\s*\{([^}]*)\}\s*=\s*(?:await\s+)?require\s*\(\s*['"]([^'"]+)['"]\s*\)`)
)
