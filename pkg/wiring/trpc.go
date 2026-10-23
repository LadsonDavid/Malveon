package wiring

import (
	"fmt"
	"sort"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/features"
	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// A tRPC procedure (or an Astro action) is named in a plan line as `post.create`,
// not by URL. Each one is a route at /<base>/post.create (graph.Node.RPC), so a
// name that matches a procedure is checked as that URL, and a name that doesn't,
// inside a router that exists, is a procedure that isn't built.

var clientPrefixes = []string{"trpc.", "api.", "caller.", "client.", "utils.", "trpcClient.", "actions."}

// rpcProcedures lists the dotted names of the project's tRPC procedures with
// their full URLs.
func rpcProcedures(g *graph.Graph) map[string]string {
	out := map[string]string{}
	for _, n := range g.Nodes {
		if n.Kind != graph.RouteHandler || !n.RPC || n.Confidence != graph.Extracted {
			continue
		}
		if i := strings.LastIndexByte(n.Path, '/'); i >= 0 {
			out[n.Path[i+1:]] = n.Path
		}
	}
	return out
}

// hasUnreadableRPC: some tRPC router or procedure is defined where malveon can't
// read it (a package, a spread), so "no such procedure" can't be claimed.
func hasUnreadableRPC(g *graph.Graph) bool {
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && n.RPC && n.Confidence != graph.Extracted {
			return true
		}
	}
	return false
}

// checkRPCName answers a dotted code name in a plan line when it is a tRPC
// procedure (or one missing from a router that exists).
func checkRPCName(g *graph.Graph, name string, f features.Feature) (finding, bool) {
	if !strings.Contains(name, ".") {
		return finding{}, false
	}
	procs := rpcProcedures(g)
	if len(procs) == 0 {
		return finding{}, false
	}
	candidates := []string{name}
	for _, p := range clientPrefixes {
		if strings.HasPrefix(name, p) {
			candidates = append(candidates, strings.TrimPrefix(name, p))
		}
	}
	for _, c := range candidates {
		if url, ok := procs[c]; ok {
			return checkPath(g, anchor{kind: pathAnchor, text: url}, !features.ExpectsOutsideCaller(f))
		}
	}
	// The router exists but has no such procedure.
	for _, c := range candidates {
		segs := strings.Split(c, ".")
		if len(segs) < 2 {
			continue
		}
		router := strings.Join(segs[:len(segs)-1], ".")
		var have []string
		for p := range procs {
			if strings.HasPrefix(p, router+".") && !strings.Contains(strings.TrimPrefix(p, router+"."), ".") {
				have = append(have, strings.TrimPrefix(p, router+"."))
			}
		}
		if len(have) > 0 && !hasUnreadableRPC(g) {
			sort.Strings(have)
			return finding{verdict: NotBuilt,
				reason: fmt.Sprintf("`%s` has no `%s` procedure (it has: %s)", router, segs[len(segs)-1], strings.Join(have, ", "))}, true
		}
	}
	return finding{}, false
}
