// Package extractor builds a graph.Graph by scanning source files for
// route registrations and outbound network calls. It supports Python,
// JavaScript, TypeScript, and Go (the four languages confirmed in scope).
// It never runs the code, never imports another package's parser, and
// never guesses a path it can't read as a literal — see literalPath.
package extractor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/LadsonDavid/beta-test/internal/graph"
	"github.com/LadsonDavid/beta-test/internal/skipdirs"
)

// Extract walks root and returns the code graph for every supported
// source file found.
func Extract(root string) (*graph.Graph, error) {
	g := &graph.Graph{}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipdirs.Names[d.Name()] {
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

		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // unreadable file: skip, don't fail the whole run
		}
		src := string(raw)

		switch ext {
		case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
			sites = scanJSLike(src)
			if IsNextRouteFile(rel) {
				sites = append(sites, scanNextRouteHandlers(rel, src)...)
			}
		case ".py":
			sites = scanPython(src)
		case ".go":
			fset, file, perr := parseGoFile(path, src)
			if perr != nil {
				return nil // syntax error in one file shouldn't kill the run
			}
			sites = scanGo(src, fset, file)
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
	return g, nil
}

func toNode(file string, s callSite) graph.Node {
	n := graph.Node{
		Kind:          s.kind,
		File:          file,
		Line:          s.line,
		Method:        s.method,
		EnclosingFunc: s.enclosingFunc,
		BodyFields:    s.bodyFields,
	}
	if path, ok := literalPath(s.argRaw); ok {
		n.Path = path
		n.Confidence = graph.Extracted
	} else {
		n.Confidence = graph.Inferred
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
	s = strings.TrimSuffix(s, filepath.Ext(s))
	// insert a boundary before an uppercase letter that follows a
	// lowercase/digit (camelCase -> camel Case), then split on any
	// remaining non-alphanumeric run (-, _, /, .).
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && isUpper(r) && !isUpper(runes[i-1]) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	fields := strings.FieldsFunc(b.String(), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, strings.ToLower(f))
	}
	return out
}

func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}
