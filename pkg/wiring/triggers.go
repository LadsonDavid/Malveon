package wiring

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/features"
	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// Does a button reach the request? (prevention-gate build plan, Step 1.)
//
// "I click the button and nothing happens" is the commonest way an agent's
// "done" is wrong, and it passed: a request that exists in a file read as
// CONNECTED whether or not anything on the screen runs it. When a plan line
// talks about a button, a click or a form, the request it points at must be
// reachable from one.
//
// The verdict rests on one fact malveon can prove from the code: the function
// that makes the request is never used, anywhere in the project (graph.Uses is
// 0), and no handler on a screen runs it. Anything weaker — the function is
// used somewhere, but not by a button; the handler is a prop, an alias, a
// hook's result — is left alone: a wrong "dead button" would block a working
// app, and the cost of a miss is only the old answer.

var triggerWordPattern = regexp.MustCompile(`(?i)\b(?:buttons?|click(?:s|ed|ing)?|taps?|press(?:es|ed)?|submit(?:s|ted)?|forms?)\b`)

// namesATrigger: the line talks about something a person clicks or submits.
// Words inside backticks are file and code names, not what the line says.
func namesATrigger(name string) bool {
	return triggerWordPattern.MatchString(backtickPattern.ReplaceAllString(name, " "))
}

// callReach is what is known about how a request gets made.
type callReach struct {
	trigger graph.Trigger // reached: the trigger whose handler runs the request
	reached bool
	unused  bool   // proven: nothing in the project uses the function that makes it
	fn      string // that function
}

// reaches: this trigger's handler runs the request. Its handler is written around
// the request, or names the function that makes it.
func reaches(t graph.Trigger, call graph.Node) bool {
	h := t.Handler
	if t.File == call.File && h.Kind == graph.HandlerInline && call.Line >= h.From && call.Line <= h.To {
		return true // the request is written inside the handler
	}
	return call.EnclosingFunc != "" && mentions(h.Mentions, call.EnclosingFunc)
}

// reachOf says whether a screen's trigger runs the call, and if not whether the
// function making it is provably unused. Both false means: can't tell.
func reachOf(g *graph.Graph, call graph.Node) callReach {
	if !isJSFile(call.File) {
		return callReach{}
	}
	// Only this screen's triggers: the call's own file, then the files that
	// import it. A same-named function in an unrelated file is not a connection.
	for _, t := range g.TriggersNear(call.File) {
		if reaches(t, call) {
			return callReach{trigger: t, reached: true}
		}
	}
	fn := call.EnclosingFunc
	if fn == "" {
		return callReach{}
	}
	// Only a name the project defines has a count: a method or an object member
	// the range scanner found has none, and is never called unused.
	if n, defined := g.Uses[fn]; defined && n == 0 {
		return callReach{unused: true, fn: fn}
	}
	return callReach{}
}

func isJSFile(file string) bool {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
		return true
	}
	return false
}

// mentions: name is in a sorted list of identifiers.
func mentions(sorted []string, name string) bool {
	i := sort.SearchStrings(sorted, name)
	return i < len(sorted) && sorted[i] == name
}

// checkTriggers turns a CONNECTED line that talks about a button into HALF
// BUILT when the request is provably never run by it. done=false leaves the
// result as it was. Two independent proofs, either is enough:
//   - the function that makes every request is used nowhere in the project, or
//   - the button the line names is itself dead (deadButtonFor).
//
// Anything weaker is left alone: a wrong "dead button" would block a working app.
func checkTriggers(g *graph.Graph, f features.Feature, res Result) (out Result, done bool) {
	if !namesATrigger(f.Name) || len(res.Pairs) == 0 {
		return res, false
	}
	first := res.Pairs[0]
	// Proof 1: the function that makes every request is used nowhere in the project.
	var fn string
	allUnused := true
	for i, p := range res.Pairs {
		rc := reachOf(g, p.Call)
		if !rc.unused {
			allUnused = false
			break
		}
		if i == 0 {
			fn = rc.fn
		}
	}
	if allUnused {
		reason := fmt.Sprintf("%s %s is requested from `%s` (%s), but nothing in the app calls `%s`", methodLabel(first.Call), first.Call.Path, fn, loc(first.Call), fn)
		if seen := describeTriggersIn(g, first.Call.File); len(seen) > 0 {
			reason += " — " + strings.Join(seen, "; ")
		}
		reason += ". Clicking it can't send the request"
		res.Verdict = HalfBuilt
		res.Reason = reason
		res.Evidence = dedupe(append(res.Evidence, loc(first.Call)))
		return res, true
	}
	// Proof 2: the button the line names does nothing, whatever else can send the request.
	if t, why, ok := deadButtonFor(g, f.Name, first.Call); ok {
		res.Verdict = HalfBuilt
		res.Reason = fmt.Sprintf("%s %s is requested from %s (%s), but %s (%s) %s. Clicking it can't send the request",
			methodLabel(first.Call), first.Call.Path, requestedFrom(first.Call), loc(first.Call), tagLabel(t), triggerLoc(t), why)
		res.Evidence = dedupe(append(res.Evidence, loc(first.Call), triggerLoc(t)))
		return res, true
	}
	return res, false
}

func requestedFrom(call graph.Node) string {
	if call.EnclosingFunc != "" {
		return "`" + call.EnclosingFunc + "`"
	}
	return "the file"
}

var wordPattern = regexp.MustCompile(`[A-Za-z0-9]+`)

// deadButtonFor proves that the button a plan line names does nothing: every button
// near the request whose visible text is words of the line has no handler, or one
// that runs nothing (onClick={() => {}}). It says no whenever the click could be
// handled somewhere malveon can't see — a handler on a wrapper or the form around
// it, spread props, a framework attribute, a listener added in code, or another
// button with the same words that does work.
func deadButtonFor(g *graph.Graph, line string, call graph.Node) (graph.Trigger, string, bool) {
	if !isJSFile(call.File) {
		return graph.Trigger{}, "", false
	}
	lineWords := map[string]bool{}
	for _, w := range wordPattern.FindAllString(strings.ToLower(backtickPattern.ReplaceAllString(line, " ")), -1) {
		lineWords[w] = true
	}
	near := g.TriggersNear(call.File)
	var dead []graph.Trigger
	for _, t := range near {
		if t.Tag != "button" || t.Label == "" || !labelIn(t.Label, lineWords) {
			continue
		}
		if !handlesNothing(t.Handler) {
			return graph.Trigger{}, "", false // a Refund button that is wired (or might be)
		}
		dead = append(dead, t)
	}
	if len(dead) == 0 {
		return graph.Trigger{}, "", false
	}
	for _, t := range near {
		switch {
		case t.Handler.Kind == graph.HandlerUnknown:
			return graph.Trigger{}, "", false // spread props or a framework attribute somewhere near
		case reaches(t, call) && t.Label == "":
			return graph.Trigger{}, "", false // a wrapper or form around the button runs it; the click bubbles up
		}
	}
	for _, t := range dead {
		if g.OpaqueWiring[t.File] || g.Names[t.File]["addEventListener"] {
			return graph.Trigger{}, "", false
		}
	}
	t := dead[0]
	why := "has no click handler"
	if t.Handler.Kind == graph.HandlerInline {
		why = "has a click handler that does nothing"
	}
	return t, why, true
}

// labelIn: every word of the button's visible text is a word of the plan line.
func labelIn(label string, lineWords map[string]bool) bool {
	words := wordPattern.FindAllString(strings.ToLower(label), -1)
	for _, w := range words {
		if !lineWords[w] {
			return false
		}
	}
	return len(words) > 0
}

// handlesNothing: no handler is attached, or the one written in place uses no
// name at all (an empty function), so clicking cannot run anything.
func handlesNothing(h graph.Handler) bool {
	return h.Kind == graph.HandlerNone || h.Kind == graph.HandlerInline && len(h.Mentions) == 0
}

// describeTriggersIn says what the buttons and forms in a file are wired to,
// so the person can find the one that does nothing.
func describeTriggersIn(g *graph.Graph, file string) []string {
	var out []string
	for _, t := range g.Triggers {
		if t.File != file {
			continue
		}
		what := ""
		switch t.Handler.Kind {
		case graph.HandlerNone:
			what = "has no " + noun(t) + " handler"
		case graph.HandlerNamed:
			what = fmt.Sprintf("runs `%s`", t.Handler.Name)
		case graph.HandlerInline:
			what = "runs a function written in place"
		default:
			continue // a link, a form's button, something unreadable: not worth saying
		}
		out = append(out, fmt.Sprintf("%s %s %s", tagLabel(t), "("+triggerLoc(t)+")", what))
		if len(out) == 2 {
			break
		}
	}
	return out
}

func noun(t graph.Trigger) string {
	switch t.Tag {
	case "form":
		return "submit"
	}
	return "click"
}

// tagLabel: `<button>` "Refund"
func tagLabel(t graph.Trigger) string {
	s := "`<" + t.Tag + ">`"
	if t.Label != "" {
		s += fmt.Sprintf(" %q", t.Label)
	}
	return s
}

func triggerLoc(t graph.Trigger) string { return fmt.Sprintf("%s:%d", t.File, t.Line) }

// triggerHop is the part of the path that says what a person does: onClick on
// <button> "Refund". It goes in front of the Call hop; the path keeps its four
// hops because the editor labels them by position.
func triggerHop(t graph.Trigger) string {
	s := t.Event + " on <" + t.Tag + ">"
	if t.Event == "" {
		s = "<" + t.Tag + ">"
	}
	if t.Label != "" {
		s += fmt.Sprintf(" %q", t.Label)
	}
	return s + " (" + triggerLoc(t) + ")"
}
