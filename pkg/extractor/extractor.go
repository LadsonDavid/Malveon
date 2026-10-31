// Package extractor builds a graph.Graph by scanning source files for
// route registrations and outbound network calls. It supports Python,
// JavaScript, TypeScript, and Go, plus backend routes only for Rust,
// Java, Ruby, PHP, and C# (see rust.go/java.go/ruby.go/php.go/dotnet.go).
// It never runs the code, never imports another package's parser, and
// never guesses a path it can't read as a literal — see literalPath.
package extractor

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
	"github.com/LadsonDavid/Malveon/pkg/skipdirs"
)

// Extract walks root and returns the code graph for every supported
// source file found.
func Extract(root string) (*graph.Graph, error) {
	g := &graph.Graph{}
	imports := map[string][]rawImport{}
	exports := map[string]graph.ExportSet{}
	info := collectJSInfo(root)
	mounts := map[string][]jsMount{}
	routerFiles := map[string]int{}
	pyMounts := map[string][]pyMountRef{}
	dj := newDjangoProject()
	tp := newTRPCProject()
	aa := newAstroActions()
	rp := newRemixProject()
	var tplCalls []tplCall

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skipdirs.Names[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		var sites []callSite
		ext := strings.ToLower(filepath.Ext(path))

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		g.Files = append(g.Files, rel)
		if ext == ".sql" || ext == ".prisma" {
			if raw, err := os.ReadFile(path); err == nil {
				readSchema(g, rel, ext, string(raw))
			}
			return nil
		}
		if ext == ".md" || ext == ".mdx" {
			for _, site := range astroPageSite(rel, info) {
				g.Nodes = append(g.Nodes, toNode(rel, site))
			}
			return nil
		}
		if tplExts[ext] {
			if raw, err := os.ReadFile(path); err == nil {
				tplCalls = append(tplCalls, scanTemplateCalls(rel, string(raw))...)
			}
			return nil
		}
		if !scannedExts[ext] {
			return nil
		}

		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // unreadable file: skip, don't fail the whole run
		}
		src := string(raw)
		recordDefs(g, rel, ext, src)

		switch ext {
		case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
			sites = scanJSLikeWith(src, info)
			if ext != ".ts" && ext != ".cjs" { // JSX can't be in these
				trigs, opaque := scanTriggersAndOpaque(src)
				for _, t := range trigs {
					t.File = rel
					g.Triggers = append(g.Triggers, t)
				}
				if opaque {
					if g.OpaqueWiring == nil {
						g.OpaqueWiring = map[string]bool{}
					}
					g.OpaqueWiring[rel] = true
				}
			}
			imports[rel] = scanJSImports(src)
			exports[rel] = scanJSExports(src)
			mounts[rel] = scanJSMounts(src)
			routerFiles[rel] = jsRouterCount(src)
			if IsNextRouteFile(rel) {
				sites = append(sites, scanNextRouteHandlers(rel, src)...)
			}
			if site, ok := nextPageSite(rel); ok {
				sites = append(sites, site)
			}
			sites = append(sites, scanNextPagesRouter(rel, src, info)...)
			sites = append(sites, scanNuxtServerRoute(rel, src, info)...)
			sites = append(sites, scanNest(rel, src, info)...)
			sites = append(sites, scanSvelteKitServer(rel, src, info)...)
			sites = append(sites, scanAstroEndpoint(rel, src, info)...)
			aa.readFile(rel, src, info)
			if info.projectHasDep("@trpc/") {
				tp.readFile(rel, src)
			}
			sites = append(sites, rp.readFile(rel, src, info)...)
		case ".vue", ".svelte", ".astro":
			// A single-file component makes requests; it never registers server routes.
			body := blankStyleBlocks(src)
			for _, s := range scanJSLikeWith(body, info) {
				if s.kind != graph.RouteHandler {
					sites = append(sites, s)
				}
			}
			imports[rel] = scanJSImports(body)
			sites = append(sites, nuxtPageSite(rel, info)...)
			sites = append(sites, svelteKitPage(rel, src, info)...)
			sites = append(sites, astroPageSite(rel, info)...)
			aa.readFile(rel, src, info)
		case ".py":
			sites = scanPython(src)
			pyMounts[rel] = scanPyMounts(src)
			dj.readPython(rel, src)
		case ".go":
			fset, file, perr := parseGoFile(path, src)
			if perr != nil {
				return nil // syntax error in one file shouldn't kill the run
			}
			sites = scanGo(src, fset, file)
		case ".rs":
			sites = scanRust(src)
		case ".java":
			sites = scanJava(src)
		case ".rb":
			sites = scanRuby(src)
		case ".php":
			sites = scanPHP(src)
		case ".cs":
			sites = scanDotnet(src)
		default:
			return nil
		}

		for _, s := range sites {
			g.Nodes = append(g.Nodes, toNode(rel, s))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	resolveImports(g, imports, info)
	g.Exports = exports
	g.Uses = make(map[string]int, len(g.Defs))
	for name := range g.Defs {
		g.Uses[name] = info.uses[name]
	}
	registerDrizzleTables(g, info)
	applyMounts(g, mounts, routerFiles)
	applyPyMounts(g, pyMounts)
	applyTemplateCalls(g, tplCalls, dj.apply(g))
	tp.apply(g)
	aa.apply(g)
	rp.apply(g)
	// Last, so every kind of call node (fetch, tRPC callers, templates, Go) is covered.
	for i := range g.Nodes {
		if isTestFile(g.Nodes[i].File) {
			g.Nodes[i].InTest = true
		}
	}
	return g, nil
}

// isTestFile: the path follows a test naming convention no app page would —
// foo.test.ts, foo.spec.tsx, foo.cy.ts, *_test.go, test_*.py, *_test.py,
// conftest.py, or a __tests__, cypress or e2e folder. A bare test/ or tests/
// folder is not enough: a quiz app can have a real /tests page.
func isTestFile(rel string) bool { return IsTestFile(rel) }

// IsTestFile reports whether a file follows a test naming convention (see above).
func IsTestFile(rel string) bool {
	if graph.SourceRole(rel) != "production" {
		return true
	}
	parts := strings.Split(strings.ToLower(filepath.ToSlash(rel)), "/")
	for _, dir := range parts[:len(parts)-1] {
		if dir == "__tests__" || dir == "cypress" || dir == "e2e" {
			return true
		}
	}
	base := parts[len(parts)-1]
	switch {
	case strings.HasSuffix(base, "_test.go"), strings.HasSuffix(base, "_test.py"),
		strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py"), base == "conftest.py", base == "tests.rs", strings.HasSuffix(base, ".rs") && inDir(parts, "tests"):
		return true
	}
	for _, mark := range []string{".test.", ".spec.", ".cy."} {
		if strings.Contains(base, mark) {
			return true
		}
	}
	return false
}

// inDir: one of the folders above the file is named dir (Rust integration tests live in tests/).
func inDir(parts []string, dir string) bool {
	for _, p := range parts[:len(parts)-1] {
		if p == dir {
			return true
		}
	}
	return false
}

var scannedExts = map[string]bool{
	".vue": true, ".svelte": true, ".astro": true,
	".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".mjs": true, ".cjs": true,
	".py": true, ".go": true, ".rs": true, ".java": true, ".rb": true, ".php": true, ".cs": true,
}

// defPatterns find top-level names a plan line might mention in
// backticks (`InvitesTab`, `sendEmailNotification()`), so "is this
// component/function actually in the code" is a lookup, not a guess.
var defPatterns = map[string]*regexp.Regexp{
	"js": regexp.MustCompile(`(?m)(?:^|[\s;])(?:export\s+)?(?:default\s+)?(?:async\s+)?(?:function\*?|class|const|let|var|interface|type|enum)\s+([A-Za-z_$][\w$]*)`),
	"py": regexp.MustCompile(`(?m)^\s*(?:async\s+)?(?:def|class)\s+(\w+)`),
	"go": regexp.MustCompile(`(?m)^(?:func(?:\s*\([^)]*\))?|type)\s+(\w+)`),
}

var identPattern = regexp.MustCompile(`[A-Za-z_$][\w$]*`)

// recordDefs indexes where names are defined (functions, classes,
// components) and where every identifier first appears.
func recordDefs(g *graph.Graph, rel, ext, src string) {
	var pat *regexp.Regexp
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".vue", ".svelte", ".astro":
		pat = defPatterns["js"]
	case ".py":
		pat = defPatterns["py"]
	case ".go":
		pat = defPatterns["go"]
	}
	if g.Defs == nil {
		g.Defs = map[string][]graph.Loc{}
		g.Names = map[string]map[string]bool{}
	}
	names := map[string]bool{}
	for _, m := range identPattern.FindAllStringIndex(src, -1) {
		if name := src[m[0]:m[1]]; len(name) >= 3 {
			names[name] = true
		}
	}
	g.Names[rel] = names
	if pat == nil {
		return
	}
	ends := map[string]int{}
	if pat == defPatterns["js"] {
		for _, r := range jsFuncRanges(src) {
			if _, ok := ends[r.name]; !ok {
				ends[r.name] = r.endLine
			}
		}
	}
	cm := newCodeMap(src, definitionSyntax(ext))
	for _, m := range pat.FindAllStringSubmatchIndex(src, -1) {
		if !cm.codeAt(src, m[0]) {
			continue // a function named in a comment or a string isn't defined
		}
		name := src[m[2]:m[3]]
		g.Defs[name] = append(g.Defs[name], graph.Loc{File: rel, Line: lineOf(src, m[2]), End: ends[name]})
	}
}

func toNode(file string, s callSite) graph.Node {
	n := graph.Node{
		Kind:           s.kind,
		File:           file,
		Line:           s.line,
		Method:         s.method,
		EnclosingFunc:  s.enclosingFunc,
		BodyFields:     s.bodyFields,
		RequiredFields: s.requiredFields,
		Tail:           s.tail,
		Op:             s.op,
		Unreferenced:   s.unreferenced,
		Stub:           s.stub,
		Receiver:       s.receiver,
		Unmounted:      s.unmounted,
		FileRouted:     s.fileRouted,
		RPC:            s.rpc,
	}
	if s.kind == graph.DataCall {
		n.Path = s.argRaw // a table/function name, read straight from a string literal
		n.Confidence = graph.Extracted
	} else if path, ok := literalPath(s.argRaw); ok {
		n.Path = path
		n.Confidence = graph.Extracted
	} else {
		n.Confidence = graph.Inferred
		n.External = s.kind == graph.NetworkCall && isAbsoluteURLArg(s.argRaw)
	}
	n.Words = wordsFor(file, n.Path)
	return n
}

// reservedBaseNames are filenames so common across a codebase that they
// carry no identifying signal on their own — Next.js's App Router
// reserves "page"/"layout"/etc. for a role, not a topic, and nearly
// every route segment has one; "index"/"main"/"__init__" are the same
// story in plain JS/TS/Go/Python. Confirmed 2026-09-16 against a real
// project: a feature description that happened to contain the ordinary
// word "page" matched 75 unrelated page.tsx files across the whole app,
// because that word was the *only* thing distinguishing those nodes.
// Excluding these from a file's own name-derived words means such a
// node is only ever matched by something real — a resolved path segment
// — never by riding along on its reserved role.
var reservedBaseNames = map[string]bool{
	// Next.js App Router special files
	"page": true, "layout": true, "loading": true, "error": true,
	"template": true, "default": true, "not-found": true, "route": true,
	"global-error": true, "middleware": true,
	// General JS/TS entry point
	"index": true,
	// Go
	"main": true,
	// Python
	"__init__": true,
}

// wordsFor turns a file path and a route path into a flat set of lowercase
// tokens used for loose feature-name matching (e.g. "refund button"
// matching a "/refund" route defined in "RefundButton.jsx").
func wordsFor(file, path string) []string {
	var words []string
	base := filepath.Base(file)
	stem := strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
	if !reservedBaseNames[stem] {
		words = append(words, splitWords(base)...)
	}
	if path != "" {
		for _, seg := range strings.Split(strings.Trim(path, "/"), "/") {
			if seg == "" || strings.HasPrefix(seg, ":") || strings.HasPrefix(seg, "{") {
				continue
			}
			words = append(words, splitWords(seg)...)
		}
	}
	return words
}

func splitWords(s string) []string {
	return graph.SplitWords(strings.TrimSuffix(s, filepath.Ext(s)))
}

// definitionSyntax is the lexical rules of the file type recordDefs reads.
func definitionSyntax(ext string) syntax {
	switch ext {
	case ".py":
		return pySyntax
	case ".go":
		return cLikeSyn
	}
	return jsSyntax
}
