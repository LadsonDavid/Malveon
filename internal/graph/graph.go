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

	commonWords map[string]bool // lazily computed, see commonWordSet
}

// commonWordMinCount and commonWordFraction jointly decide when a word
// has stopped meaning anything in *this* codebase. Chosen from real data
// (2026-09-16, a 292-node real project): "api" sat at 140 nodes (47.9%)
// and "admin" at 19 (6.5%), both causing real false-positive matches
// (a "Circles" feature matching an unrelated "announcements" route
// because both happened to be under /admin/*); a genuinely specific
// feature word like "circles" itself sat at 10 nodes (3.4%). The
// min-count floor exists specifically so this never fires on a small
// graph (a test fixture, a young project) where a legitimate shared
// word between a real call/route pair can easily be 100% of a tiny
// node count — the fraction alone can't tell "too common" apart from
// "this pair is exactly what it looks like" without enough nodes to
// make the statistic mean something.
const (
	commonWordMinCount = 15
	commonWordFraction = 0.05
)

// stopWords are ordinary English function words that carry no domain
// meaning at all — unlike commonWordFraction/commonWordMinCount (which
// catch a word specific to *this* codebase becoming too common to mean
// anything), these can coincidentally collide with a URL path fragment
// regardless of how rare that collision is. Confirmed 2026-09-16: a
// feature description starting "All 6 Settings tabs..." matched an
// unrelated /api/notifications/read-all route — "all" appeared on only
// one node in the whole graph, far too rare to trip the frequency
// filter, but it's still not a real signal. A small, fixed, standard
// stopword list — not per-codebase, never needs tuning — closes that
// gap the frequency filter structurally can't.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true,
	"and": true, "or": true, "of": true, "to": true, "in": true, "on": true,
	"for": true, "with": true, "from": true, "by": true, "at": true,
	"is": true, "are": true, "be": true, "as": true, "it": true,
	"this": true, "that": true, "all": true, "any": true,
	"your": true, "you": true, "we": true, "our": true,
}

// commonWordSet returns the words that carry no identifying signal in
// this graph — either because they're generic English (stopWords) or
// because they're specific to this codebase but appear in too many
// nodes to mean anything (commonWordFraction/commonWordMinCount).
// Computed once and cached — every check that resolves features against
// this graph calls FindNodesMatching repeatedly, and the set doesn't
// change mid-run.
func (g *Graph) commonWordSet() map[string]bool {
	if g.commonWords != nil {
		return g.commonWords
	}
	counts := map[string]int{}
	for _, n := range g.Nodes {
		for _, w := range dedupe(normalizeWords(n.Words)) {
			counts[w]++
		}
	}
	floor := float64(len(g.Nodes)) * commonWordFraction
	common := map[string]bool{}
	for w := range stopWords {
		common[w] = true
	}
	for w, c := range counts {
		if c > commonWordMinCount && float64(c) > floor {
			common[w] = true
		}
	}
	g.commonWords = common
	return common
}

func dedupe(words []string) []string {
	seen := make(map[string]bool, len(words))
	out := make([]string, 0, len(words))
	for _, w := range words {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

// FindNodesMatching returns nodes of the given kind whose Words overlap
// with any of the given feature words (case-insensitive, loose token
// match — not an exact string match), excluding words too common across
// the whole graph to mean anything (see commonWordSet).
func (g *Graph) FindNodesMatching(kind Kind, featureWords []string) []Node {
	common := g.commonWordSet()
	wanted := excludeCommon(normalizeWords(featureWords), common)
	var out []Node
	for _, n := range g.Nodes {
		if n.Kind != kind {
			continue
		}
		if wordsOverlap(excludeCommon(normalizeWords(n.Words), common), wanted) {
			out = append(out, n)
		}
	}
	return out
}

func excludeCommon(words []string, common map[string]bool) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		if !common[w] {
			out = append(out, w)
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
		if IsWildcardSegment(b[i]) || IsWildcardSegment(a[i]) {
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

func IsWildcardSegment(seg string) bool {
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

// IsFullyStatic reports whether path has no wildcard segments at all —
// used by the overlap check to tell a genuinely static route ("/users/
// mine") apart from a parameterized one ("/users/:id") that happens to
// match it, since several routers (see internal/checks/overlap's
// reachability/ordering notes) resolve the two very differently.
func IsFullyStatic(path string) bool {
	for _, seg := range splitPath(path) {
		if IsWildcardSegment(seg) {
			return false
		}
	}
	return true
}
