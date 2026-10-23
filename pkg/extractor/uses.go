package extractor

import (
	"regexp"
	"sort"
	"strings"
)

// Graph.Uses: for every function or component the project defines, how many
// times its name is used. 0 is the proof behind "nothing in the app ever calls
// this" — so a wrong 0 would tell someone their working button is dead, and
// everything here errs toward "used":
//
//   - words in comments and strings count;
//   - an import under another name (import { f as g }) and an export under
//     another name count as a use of f, because the real calls say g;
//   - a default export counts, since it can be imported under any name;
//   - HTML and template files count by plain words (onclick="f()", @click="f").
//
// What does not count: the declaration itself, an import of the name, and its
// place in an export list — none of those is a call.
var (
	// import a, { b as c } from 'x' (the specifiers may span lines)
	usesImportStmt = regexp.MustCompile(`(?m)^[ \t]*import\b([^;'"]*?)\bfrom\s*['"][^'"\n]*['"]`)
	// export { a, b as c }   export { a } from 'x'
	usesExportList = regexp.MustCompile(`(?m)^[ \t]*export\s*(?:type\s+)?\{([^}]*)\}(?:\s*from\s*['"][^'"\n]*['"])?`)
)

// countUses adds the uses found in one JS/TS file to counts.
func countUses(src string, cm codeMap, counts map[string]int) {
	var skip [][2]int // spans whose words are not uses
	for _, re := range []*regexp.Regexp{usesImportStmt, usesExportList} {
		for _, m := range re.FindAllStringSubmatchIndex(src, -1) {
			if !cm.codeAt(src, m[0]) {
				continue
			}
			skip = append(skip, [2]int{m[0], m[1]})
			aliasedNames(src[m[2]:m[3]], counts)
		}
	}
	for _, m := range defPatterns["js"].FindAllStringSubmatchIndex(src, -1) {
		// `export default function f` keeps its name as a use: it can be imported as anything.
		if !cm.codeAt(src, m[0]) || strings.Contains(src[m[0]:m[2]], "default") {
			continue
		}
		skip = append(skip, [2]int{m[2], m[3]})
	}
	sort.Slice(skip, func(i, j int) bool { return skip[i][0] < skip[j][0] })

	next, until := 0, 0
	for _, w := range wordPattern.FindAllStringIndex(src, -1) {
		for next < len(skip) && skip[next][0] <= w[0] {
			until = max(until, skip[next][1])
			next++
		}
		if w[0] < until {
			continue
		}
		counts[src[w[0]:w[1]]]++
	}
}

// aliasedNames counts the original name of every `a as b` in a specifier list.
func aliasedNames(specifiers string, counts map[string]int) {
	if open := strings.IndexByte(specifiers, '{'); open >= 0 {
		specifiers = specifiers[open:]
	}
	for _, s := range strings.Split(strings.Trim(specifiers, "{} \t\r\n"), ",") {
		parts := strings.Fields(strings.TrimPrefix(strings.TrimSpace(s), "type "))
		if len(parts) == 3 && parts[1] == "as" && parts[0] != parts[2] && parts[0] != "default" && parts[0] != "*" {
			counts[parts[0]]++
		}
	}
}
