package extractor

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// Imports are what the plan-items check follows to trace a flow end to
// end: backwards from a component to the page that shows it, and forwards
// from a route handler into the project code it uses. Only imports that
// resolve to a file in the project count; a package import is never
// followed.

var (
	// import X from 'm' · import { a, b as c } from 'm' · import * as ns from 'm'
	// export { a } from 'm' · import 'm'
	jsImportFromPattern = regexp.MustCompile(`(?m)^\s*(?:import|export)\s+(?:type\s+)?([^'";]*?)\s*from\s*['"]([^'"]+)['"]`)
	jsImportBarePattern = regexp.MustCompile(`(?m)^\s*import\s*['"]([^'"]+)['"]`)
	// import('m') · require('m') — the whole file; import('m').Name (a
	// TypeScript type reference) or require('m').name — just that name
	jsImportCallPattern = regexp.MustCompile(`\b(?:import|require)\s*\(\s*['"]([^'"]+)['"]\s*\)(?:\s*\.\s*([A-Za-z_$][\w$]*))?`)
)

type rawImport struct {
	typeOnly bool
	name     string // name as defined in the imported file; "" = the whole file
	spec     string // the module text as written: './x', '@/lib/y'
	line     int    // the line of the import statement
}

func scanJSImports(src string) []rawImport {
	var out []rawImport
	cm := newCodeMap(src, jsSyntax)
	// A commented-out import is not an import. The match starts at the
	// keyword, so that is what must be code.
	live := func(idx []int) []string {
		if !cm.codeAt(src, idx[0]) {
			return nil
		}
		groups := make([]string, len(idx)/2)
		for g := range groups {
			if idx[2*g] >= 0 {
				groups[g] = src[idx[2*g]:idx[2*g+1]]
			}
		}
		return groups
	}
	for _, idx := range jsImportFromPattern.FindAllStringSubmatchIndex(src, -1) {
		if m := live(idx); m != nil {
			ln := lines.lineOf(src, idx[0])
			for _, ri := range importedNames(m[1], m[2]) {
				ri.line = ln
				ri.typeOnly = ri.typeOnly || regexp.MustCompile(`^(?:import|export)\s+type\s`).MatchString(strings.TrimSpace(m[0]))
				out = append(out, ri)
			}
		}
	}
	for _, idx := range jsImportBarePattern.FindAllStringSubmatchIndex(src, -1) {
		if m := live(idx); m != nil {
			out = append(out, rawImport{spec: m[1], line: lines.lineOf(src, idx[0])})
		}
	}
	for _, idx := range jsImportCallPattern.FindAllStringSubmatchIndex(src, -1) {
		if m := live(idx); m != nil {
			name := m[2]
			switch name {
			case "then", "catch", "finally", "default":
				name = "" // a Promise method or the whole module, not a name the file exports
			}
			out = append(out, rawImport{spec: m[1], name: name, line: lines.lineOf(src, idx[0])})
		}
	}
	return out
}

// importedNames splits an import clause: `X, { a, b as c }` imports the
// default (the whole file) plus a and b by the names they're defined under.
func importedNames(clause, spec string) []rawImport {
	var out []rawImport
	named := ""
	if i := strings.Index(clause, "{"); i >= 0 {
		if j := strings.Index(clause[i:], "}"); j >= 0 {
			named = clause[i+1 : i+j]
			clause = clause[:i] + clause[i+j+1:]
		}
	}
	sawDefault := false
	for _, part := range strings.Split(named, ",") {
		fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(part), "type "))
		if len(fields) > 0 && fields[0] != "default" {
			out = append(out, rawImport{name: fields[0], spec: spec, typeOnly: strings.HasPrefix(strings.TrimSpace(part), "type ")})
		} else if len(fields) > 0 {
			sawDefault = true // `default as X`: the file's default export, the whole file
		}
	}
	if strings.TrimSpace(strings.Trim(clause, " ,")) != "" || named == "" || sawDefault {
		out = append(out, rawImport{spec: spec}) // a default or namespace import: the whole file
	}
	return out
}

var importExts = []string{"", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", "/index.ts", "/index.tsx", "/index.js", "/index.jsx"}

// resolveImports turns each file's raw imports into project files, once
// every file is known. `./x`/`../x` resolve from the importing file;
// `@/x` and `~/x` try `src/x` then `x` (the usual tsconfig alias). A
// package is dropped; a project path that matches no file is kept as
// Missing, so nothing downstream mistakes unread code for no code.
func resolveImports(g *graph.Graph, raw map[string][]rawImport, info *jsInfo) {
	bySlash := map[string]string{}
	for _, f := range g.Files {
		bySlash[filepath.ToSlash(f)] = f
	}
	g.Imports = map[string][]graph.Import{}
	for file, imps := range raw {
		for _, im := range imps {
			// A bare name is a package unless it matches a project file
			// (tsconfig baseUrl: `components/Button`).
			bases, project := specBases(file, im.spec)
			// A package that lives in this repo is project code, found by its name.
			if wsBases, isWorkspace := info.workspaceImport(im.spec); isWorkspace {
				bases, project = wsBases, true
			}
			target, ok := firstFile(bySlash, bases)
			switch {
			case ok && target != file:
				g.Imports[file] = append(g.Imports[file], graph.Import{TypeOnly: im.typeOnly, Name: im.name, File: target, Spec: im.spec, Line: im.line})
			case !ok && project:
				g.Imports[file] = append(g.Imports[file], graph.Import{TypeOnly: im.typeOnly, Name: im.name, Missing: im.spec, Spec: im.spec, Line: im.line})
			}
		}
	}
}

func firstFile(bySlash map[string]string, bases []string) (string, bool) {
	for _, b := range bases {
		for _, ext := range importExts {
			if f, ok := bySlash[b+ext]; ok {
				return f, true
			}
		}
	}
	return "", false
}
