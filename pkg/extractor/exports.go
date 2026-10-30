package extractor

import (
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// What a JavaScript or TypeScript file exports, so a named import can be checked
// against it. The aim is never to be wrong in the accusing direction: anything that
// makes the list of names unknowable (a re-export of everything, CommonJS, a
// destructured or multi-variable declaration, `export =`) sets Open, and an open file
// has every name.

var (
	// export [declare] [default] [async] [abstract] function|class|interface|enum|type|const|let|var Name
	exportDeclPattern = regexp.MustCompile(`(?m)^[ \t]*export[ \t]+(?:declare[ \t]+)?(default[ \t]+)?(?:async[ \t]+)?(?:abstract[ \t]+)?(function\*?|class|interface|enum|namespace|module|type|const[ \t]+enum|const|let|var)[ \t]+([A-Za-z_$][\w$]*)`)
	// export default ...  (named or not)
	exportDefaultPattern = regexp.MustCompile(`(?m)^[ \t]*export[ \t]+default\b`)
	// export { a, b as c } [from 'm'] · export type { T } [from 'm']
	exportListPattern = regexp.MustCompile(`(?m)^[ \t]*export[ \t]+(?:type[ \t]+)?\{([^}]*)\}`)
	// export * from 'm' (everything: unknowable) · export * as ns from 'm'
	exportStarPattern = regexp.MustCompile(`(?m)^[ \t]*export[ \t]+(?:type[ \t]+)?\*[ \t]*(?:as[ \t]+([A-Za-z_$][\w$]*)[ \t]*)?from`)
	// export const { a } = x · export const [a] = x
	exportDestructurePattern = regexp.MustCompile(`(?m)^[ \t]*export[ \t]+(?:const|let|var)[ \t]*[{\[]`)
	// export const|let|var name ... (the rest of the line is read for more declarators)
	exportVarPattern = regexp.MustCompile(`(?m)^[ \t]*export[ \t]+(?:declare[ \t]+)?(?:const|let|var)[ \t]+[A-Za-z_$][\w$]*`)
	// export = x · export import x = y
	exportEqualsPattern = regexp.MustCompile(`(?m)^[ \t]*export[ \t]*=|^[ \t]*export[ \t]+import\b`)
	// CommonJS: module.exports · exports.x · exports[x] · Object.defineProperty(exports
	commonJSExportPattern = regexp.MustCompile(`\bmodule\.exports\b|(?:^|[^.\w$])exports\.|(?:^|[^.\w$])exports\[|\bObject\.defineProperty\(\s*exports\b`)
)

// listComments are the // and /* */ comments people put between the names of an export list.
var listComments = regexp.MustCompile(`//[^\n]*|/\*(?s:.*?)\*/`)

// scanJSExports lists the names src exports. It reads the text, not the code map: the
// code map is not a parser, and a regex literal holding a quote (`/"/g`) makes it take
// every later line for a string. Missing an export invents a broken import; counting an
// export that sits in a comment only hides one. So every match counts.
func scanJSExports(src string) graph.ExportSet {
	set := graph.ExportSet{Names: map[string]bool{}}

	for _, m := range exportDeclPattern.FindAllStringSubmatchIndex(src, -1) {
		if m[2] >= 0 { // export default function name: the name is local, only "default" leaves the file
			set.Names["default"] = true
			continue
		}
		set.Names[src[m[6]:m[7]]] = true
	}
	if exportDefaultPattern.MatchString(src) {
		set.Names["default"] = true
	}
	for _, m := range exportListPattern.FindAllStringSubmatchIndex(src, -1) {
		for _, part := range strings.Split(listComments.ReplaceAllString(src[m[2]:m[3]], " "), ",") {
			fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(part), "type "))
			switch {
			case len(fields) == 1:
				set.Names[fields[0]] = true
			case len(fields) == 3 && fields[1] == "as":
				set.Names[fields[2]] = true
			}
		}
	}
	for _, m := range exportStarPattern.FindAllStringSubmatchIndex(src, -1) {
		if m[2] >= 0 {
			set.Names[src[m[2]:m[3]]] = true
		} else {
			set.Open = true
		}
	}
	if exportDestructurePattern.MatchString(src) || exportEqualsPattern.MatchString(src) || commonJSExportPattern.MatchString(src) {
		set.Open = true
	}
	for _, m := range exportVarPattern.FindAllStringIndex(src, -1) {
		if hasTopLevelComma(restOfLine(src, m[1])) {
			set.Open = true
		}
	}
	return set
}

func restOfLine(src string, from int) string {
	if i := strings.IndexByte(src[from:], '\n'); i >= 0 {
		return src[from : from+i]
	}
	return src[from:]
}

// hasTopLevelComma: a comma outside brackets and strings, which in a declaration's
// first line means another declarator (`export const a = 1, b = 2`).
func hasTopLevelComma(s string) bool {
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == ',' && depth == 0:
			return true
		}
	}
	return false
}
