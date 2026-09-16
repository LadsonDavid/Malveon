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
)

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, ".malveon": true,
}

// Extract walks root and returns the code graph for every supported
// source file found.
func Extract(root string) (*graph.Graph, error) {
	g := &graph.Graph{}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		var sites []callSite
		ext := strings.ToLower(filepath.Ext(path))

		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // unreadable file: skip, don't fail the whole run
		}
		src := string(raw)

		switch ext {
		case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
			sites = scanJSLike(src)
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

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
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

// wordsFor turns a file path and a route path into a flat set of lowercase
// tokens used for loose feature-name matching (e.g. "refund button"
// matching a "/refund" route defined in "RefundButton.jsx").
func wordsFor(file, path string) []string {
	var words []string
	words = append(words, splitWords(filepath.Base(file))...)
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
