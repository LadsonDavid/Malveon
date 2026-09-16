// Package graph holds the in-memory representation every check reads from.
// Nothing outside this package should know how nodes were extracted — it
// only sees Nodes and asks Find/PathMatch questions.
package graph

import "strings"

// Confidence records how sure the extractor is about a node's path.
// EXTRACTED means the path was read directly from a string literal in
// source. INFERRED means the extractor found the call/route but the path
// wasn't a plain literal (built from a variable, template, config, etc.) —
// never treat an Inferred node as proof of a match.
type Confidence int

const (
	Extracted Confidence = iota
	Inferred
)

func (c Confidence) String() string {
	if c == Extracted {
		return "EXTRACTED"
	}
	return "INFERRED"
}

// Kind is which side of a wire a node sits on.
type Kind int

const (
	RouteHandler Kind = iota // backend: something that registers a path
	NetworkCall              // frontend: something that calls a path
)

// Node is one extracted call or route registration.
type Node struct {
	Kind          Kind
	File          string
	Line          int
	Method        string     // HTTP verb if known, "" if not applicable/unknown
	Path          string     // the literal path string, "" if not resolvable statically
	Confidence    Confidence // Extracted only when Path is a real literal
	Words         []string   // tokens used for loose feature-name matching
	EnclosingFunc string     // name of the innermost named function this node's line falls inside; "" if top-level or unknown
	BodyFields    []string   // request fields sent (NetworkCall) or read via req.body (RouteHandler); nil if unresolved, non-nil (possibly empty) if resolved
}

// Graph is just the flat set of nodes a run produced.
type Graph struct {
	Nodes []Node
}

// FindNodesMatching returns nodes of the given kind whose Words overlap
// with any of the given feature words (case-insensitive, loose token
// match — not an exact string match).
func (g *Graph) FindNodesMatching(kind Kind, featureWords []string) []Node {
	wanted := normalizeWords(featureWords)
	var out []Node
	for _, n := range g.Nodes {
		if n.Kind != kind {
			continue
		}
		if wordsOverlap(normalizeWords(n.Words), wanted) {
			out = append(out, n)
		}
	}
	return out
}

func wordsOverlap(a, b []string) bool {
	set := make(map[string]bool, len(a))
	for _, w := range a {
		set[w] = true
	}
	for _, w := range b {
		if set[w] {
			return true
		}
	}
	return false
}

func normalizeWords(words []string) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" {
			continue
		}
		out = append(out, w)
	}
	return out
}

// Pair is one network call and one route handler whose literal paths
// agree — a connection the wiring check proved exists.
type Pair struct {
	Call  Node
	Route Node
}

// MatchedPairs finds every (call, route) pair among nodes matching
// featureWords whose literal paths agree. Both wiring and contract
// checks build on this — one shared definition of "these two are
// actually connected," never two that could quietly drift apart.
func (g *Graph) MatchedPairs(featureWords []string) []Pair {
	calls := g.FindNodesMatching(NetworkCall, featureWords)
	routes := g.FindNodesMatching(RouteHandler, featureWords)

	var pairs []Pair
	for _, c := range calls {
		if c.Confidence != Extracted {
			continue
		}
		for _, r := range routes {
			if r.Confidence != Extracted {
				continue
			}
			if PathsMatch(c.Path, r.Path) {
				pairs = append(pairs, Pair{Call: c, Route: r})
			}
		}
	}
	return pairs
}

// PathsMatch compares a network-call path against a route-handler path,
// treating a segment like :id or {id} as a wildcard so "/refund/:id" and
// "/refund/123" count as the same route.
func PathsMatch(callPath, routePath string) bool {
	a := splitPath(callPath)
	b := splitPath(routePath)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if isWildcardSegment(b[i]) || isWildcardSegment(a[i]) {
			continue
		}
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return []string{}
	}
	return strings.Split(p, "/")
}

func isWildcardSegment(seg string) bool {
	if seg == "" {
		return false
	}
	if strings.HasPrefix(seg, ":") {
		return true
	}
	if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
		return true
	}
	return false
}
