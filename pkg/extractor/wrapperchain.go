package extractor

import (
	"regexp"
	"sort"
	"strings"
)

// A request helper often wraps another one: jpost(path, body) calls api(path, {method:"POST"}),
// and api calls fetch. Only a function that calls fetch itself was recognised, so every call
// through jpost looked like a request to an address nobody could read. Found by scanning a
// lead's repo (steltic_india, 2026-11-10): 8 routes its screen really calls were listed unused.
//
// Candidates are the small functions the first pass did not take for a request helper. After
// every file has been read, a candidate that hands one of its parameters to a known helper
// becomes a helper too, up to four levels deep.

const (
	maxChainBody       = 1500   // a wrapper is a few lines; longer bodies are not looked at
	maxChainCandidates = 20_000 // memory bound on very large projects
	maxChainRounds     = 4
)

type fnCandidate struct {
	name   string
	params []string
	body   string
}

// noteCandidate remembers a small function that might wrap a request helper.
func (in *jsInfo) noteCandidate(name, params, body string) {
	if name == "" || len(body) > maxChainBody || len(in.candidates) >= maxChainCandidates {
		return
	}
	var names []string
	for _, p := range strings.Split(params, ",") {
		p = strings.TrimSpace(p)
		if i := strings.IndexAny(p, ":=?"); i >= 0 {
			p = strings.TrimSpace(p[:i])
		}
		names = append(names, p)
	}
	if len(names) == 0 || !strings.Contains(body, "(") {
		return
	}
	in.candidates = append(in.candidates, fnCandidate{name: name, params: names, body: body})
}

// chainWrappers turns candidates that pass a parameter to a known helper into helpers.
func (in *jsInfo) chainWrappers() {
	if len(in.wrappers) == 0 || len(in.candidates) == 0 {
		return
	}
	for round := 0; round < maxChainRounds; round++ {
		changed := false
		for _, c := range in.candidates {
			if _, known := in.wrappers[c.name]; known {
				continue
			}
			if w, ok := wrapsHelper(c, in.wrappers); ok {
				in.wrappers[c.name] = w
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	in.candidates = nil
}

// callName: a name followed by "(" (a generic call f<T>(x) too), found once per body.
var callName = regexp.MustCompile(`([A-Za-z_$][\w$]*)\s*(?:<[^()]*>)?\s*\(`)

func wrapsHelper(c fnCandidate, known map[string]wrapper) (wrapper, bool) {
	for _, m := range callName.FindAllStringSubmatchIndex(c.body, -1) {
		inner := c.body[m[2]:m[3]]
		w, isHelper := known[inner]
		if !isHelper {
			continue
		}
		if inner == c.name {
			continue
		}
		argsList := readArgsList(c.body, m[1]-1)
		args := splitTopLevelArgs(argsList)
		if w.param >= len(args) {
			continue
		}
		arg := strings.TrimSpace(args[w.param])
		if _, literal := literalPath(arg); literal {
			continue // a fixed address: this function does not pass its parameter on
		}
		for i, pn := range c.params {
			if pn == "" || !containsWord(arg, pn) {
				continue
			}
			method := w.method
			if mm := fetchMethodFieldPattern.FindStringSubmatch(argsList); mm != nil {
				method = strings.ToUpper(mm[1])
			}
			return wrapper{param: i, method: method, prefix: w.prefix + literalPrefix(arg, pn)}, true
		}
	}
	return wrapper{}, false
}

// wrapperCall is the pattern that finds a call to any known request helper, compiled the first
// time it is needed (after every file has been read, so the helper list is final). Compiling it
// for every file made a project with hundreds of helpers several times slower.
func (in *jsInfo) wrapperCall() *regexp.Regexp {
	in.callOnce.Do(func() {
		names := make([]string, 0, len(in.wrappers))
		for n := range in.wrappers {
			names = append(names, regexp.QuoteMeta(n))
		}
		sort.Strings(names)
		in.callRe = regexp.MustCompile(`\b(` + strings.Join(names, "|") + `)\s*(?:<[^()]*>)?\s*\(`)
	})
	return in.callRe
}
