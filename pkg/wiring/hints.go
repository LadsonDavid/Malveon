package wiring

import (
	"fmt"
	"sort"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/features"
	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// A plan line that names no URL, file or code name ("Members can bookmark a
// post") can't be checked: malveon looks up what a line names and only trusts
// a word match when it's strong. Someone writing plain sentences (a vibe
// coder, or anyone typing to an agent) was told "add the URL or file it's
// about" with no hint what that could be. closestInCode lists the routes and
// pages in the code that share a word with the line, so the fix is "name one
// of these". It only ever adds a hint: the line stays unchecked (SKIPPED)
// until it names something, so a guess can't become CONNECTED.

const maxHints = 3

// actorStems say who does something, not what it is ("Members can ...").
var actorStems = map[string]bool{
	"member": true, "admin": true, "everyone": true, "someone": true, "anyone": true,
	"owner": true, "person": true, "people": true, "visitor": true,
}

func closestInCode(g *graph.Graph, f features.Feature) []string {
	want := map[string]bool{}
	for _, w := range specificWords(g, f) {
		if st := stem(w); !actorStems[st] {
			want[st] = true
		}
	}
	if len(want) == 0 {
		return nil
	}

	type hit struct {
		label string
		key   string
		where string
		score float64
		depth int  // path segments: the bare collection URL is likelier what a plain sentence means
		route bool // prefer the route's own location over a call's
	}
	// Collect every URL whose own path shares a word with the line (file names
	// are left out: "PostJobModal" is how "post" once leaked in), and how many
	// distinct URLs each shared word appears in: "report" in two routes says
	// far more than "post" in a dozen, so a rare word counts for more.
	type cand struct {
		label string
		key   string // label with every placeholder spelled the same (:id, :param, [id])
		where string
		stems []string
		depth int
		route bool // prefer the route's own location over a call's
	}
	var cands []cand
	urls := map[string]map[string]bool{} // word -> the distinct URLs it appears in
	for _, n := range g.Nodes {
		if n.Confidence != graph.Extracted || n.Path == "" || n.InTest {
			continue
		}
		var label string
		switch n.Kind {
		case graph.RouteHandler, graph.NetworkCall:
			label = n.Path
			if n.Method != "" {
				label = n.Method + " " + n.Path
			}
		case graph.Page:
			label = n.Path
		default:
			continue
		}
		seen := map[string]bool{}
		var stems []string
		for _, w := range g.ContentWords(pathWordsOf(n.Path)) {
			if st := stem(w); want[st] && !seen[st] {
				seen[st] = true
				stems = append(stems, st)
				if urls[st] == nil {
					urls[st] = map[string]bool{}
				}
				urls[st][foldPlaceholders(label)] = true
			}
		}
		if len(stems) == 0 {
			continue
		}
		cands = append(cands, cand{label: label, key: foldPlaceholders(label), where: fmt.Sprintf("%s:%d", n.File, n.Line), stems: stems,
			depth: strings.Count(n.Path, "/"), route: n.Kind != graph.NetworkCall})
	}

	best := map[string]hit{}
	for _, c := range cands {
		score := 0.0
		for _, st := range c.stems {
			score += 1 / float64(len(urls[st]))
		}
		h := hit{label: c.label, key: c.key, where: c.where, score: score, depth: c.depth, route: c.route}
		if old, ok := best[c.key]; !ok || (h.route && !old.route) || h.score > old.score {
			best[c.key] = h
		}
	}

	hits := make([]hit, 0, len(best))
	for _, h := range best {
		hits = append(hits, h)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		if hits[i].depth != hits[j].depth {
			return hits[i].depth < hits[j].depth
		}
		return hits[i].label < hits[j].label
	})
	if len(hits) > maxHints {
		hits = hits[:maxHints]
	}
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = fmt.Sprintf("`%s` (%s)", h.label, h.where)
	}
	return out
}

// stem folds a simple plural into its singular so "bookmarks" and "bookmark"
// compare equal. Deliberately small: it never needs to be a real stemmer, only
// to make the two spellings of one word meet.
func stem(w string) string {
	w = strings.ToLower(w)
	switch {
	case len(w) > 4 && strings.HasSuffix(w, "ies"):
		return w[:len(w)-3] + "y"
	case len(w) > 4 && (strings.HasSuffix(w, "sses") || strings.HasSuffix(w, "xes") ||
		strings.HasSuffix(w, "ches") || strings.HasSuffix(w, "shes") || strings.HasSuffix(w, "zes")):
		return w[:len(w)-2]
	case len(w) > 3 && strings.HasSuffix(w, "s") &&
		!strings.HasSuffix(w, "ss") && !strings.HasSuffix(w, "us") && !strings.HasSuffix(w, "is"):
		return w[:len(w)-1]
	}
	return w
}

// foldPlaceholders spells every placeholder segment the same, so the route
// /api/x/:id and a call to /api/x/:param count as one URL.
func foldPlaceholders(label string) string {
	segs := strings.Split(label, "/")
	for i, seg := range segs {
		if graph.IsWildcardSegment(seg) {
			segs[i] = ":x"
		}
	}
	return strings.Join(segs, "/")
}
