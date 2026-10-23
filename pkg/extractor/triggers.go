package extractor

import (
	"regexp"
	"sort"
	"strings"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// Triggers: the things on a screen a person acts on — a button, a form, a
// link, any element given an event handler — and what each one runs. Read
// from JSX with a small scanner, never by running anything. The point is the
// complaint "I click the button and nothing happens": a call that exists in
// a file proves nothing if no button reaches it.
//
// A trigger's handler is always one of graph.HandlerKind. The two that matter
// for a verdict are None (nothing is attached) and Unknown (malveon can't read
// it): a dead-button verdict must never rest on something that might be wired.

// maxTriggerFile skips files nobody writes by hand (a bundle): a component is
// never a megabyte. maxAttrExpr and maxAttrString bound how far one attribute
// may reach, so a broken file costs time in proportion to its size, not its
// size squared.
const (
	maxTriggerFile = 1 << 20
	maxAttrExpr    = 1 << 14
	maxAttrString  = 1 << 12
)

var (
	eventProp = regexp.MustCompile(`^on[A-Z]\w*$`)
	// Attributes that wire an element somewhere malveon doesn't read (htmx,
	// Alpine, Vue and Angular directives, data attributes a script listens for).
	frameworkAttr = regexp.MustCompile(`^(?:hx-|x-on|@|v-on|ng-|\(click|data-action$|data-click$|data-toggle$|data-target$|data-bs-|data-controller$)`)
	// () => x, async (a, b): T => x, a => x, function () {}
	inlineFunc = regexp.MustCompile(`^(?:async\s+)?(?:function\b|(?:\([^)]*\)(?:\s*:\s*[^=>{]+)?|[A-Za-z_$][\w$]*)\s*=>)`)
	// a, a.b.c, a?.b
	nameChain = regexp.MustCompile(`^[A-Za-z_$][\w$]*(?:\??\.[A-Za-z_$][\w$]*)*$`)
	identWord = regexp.MustCompile(`[A-Za-z_$][\w$]*`)
)

// jsKeywords are never "the function a handler reaches".
var jsKeywords = map[string]bool{
	"async": true, "await": true, "return": true, "const": true, "let": true, "var": true, "function": true,
	"if": true, "else": true, "true": true, "false": true, "null": true, "undefined": true, "new": true,
	"typeof": true, "void": true, "this": true, "in": true, "of": true, "for": true, "while": true, "do": true,
	"switch": true, "case": true, "break": true, "continue": true, "throw": true, "try": true, "catch": true,
	"finally": true, "class": true, "extends": true, "import": true, "export": true, "default": true,
	"delete": true, "instanceof": true, "yield": true,
}

type attrKind int

const (
	attrBare   attrKind = iota // disabled
	attrString                 // title="x"
	attrExpr                   // onClick={...}
	attrSpread                 // {...props}
)

type jsxAttr struct {
	name       string
	kind       attrKind
	value      string // string: the text; expr: the text between the braces
	start, end int    // expr: where that text sits in the source
}

type jsxOpen struct {
	tag         string
	attrs       []jsxAttr
	selfClosing bool
	end         int // offset just after the closing >
}

func (e jsxOpen) attr(name string) (jsxAttr, bool) {
	for _, a := range e.attrs {
		if a.name == name {
			return a, true
		}
	}
	return jsxAttr{}, false
}

// openEl is an element still open: only forms matter, because a button takes
// the handler of the form it sits in.
type openEl struct {
	tag     string
	handled bool // forms: something is attached
}

// scanTriggers finds the triggers in one JSX/TSX file. File is left for the
// caller to fill in.
func scanTriggers(src string) []graph.Trigger {
	out, _ := scanTriggersAndOpaque(src)
	return out
}

// scanTriggersAndOpaque also says whether any element in the file is wired in a way
// malveon cannot read: spread props ({...props}) or a framework attribute (hx-post,
// @click). Such an element may hold a handler for a button that has none of its own.
func scanTriggersAndOpaque(src string) (out []graph.Trigger, opaque bool) {
	if len(src) > maxTriggerFile || !strings.Contains(src, "<") {
		return nil, false
	}
	cm := newCodeMap(src, jsSyntax)
	var stack []openEl

	for i := 0; i < len(src); i++ {
		j := strings.IndexByte(src[i:], '<')
		if j < 0 {
			break
		}
		i += j
		if !cm.codeAt(src, i) {
			continue
		}
		if i+1 < len(src) && src[i+1] == '/' {
			stack = closeElement(src, i, stack)
			continue
		}
		if !jsxStart(src, i) {
			continue
		}
		el, ok := parseOpenTag(src, cm, i)
		if !ok {
			continue
		}
		for _, a := range el.attrs {
			opaque = opaque || a.kind == attrSpread || frameworkAttr.MatchString(a.name)
		}
		made, handled := triggersOf(src, cm, i, el, stack)
		out = append(out, made...)
		if !el.selfClosing {
			stack = append(stack, openEl{tag: el.tag, handled: handled})
		}
	}
	return out, opaque
}

// jsxStart: a "<" followed by a letter that isn't a generic (useState<string>)
// or a comparison (a<b): those have a name, ")" or "]" right before the "<".
func jsxStart(src string, i int) bool {
	if i+1 >= len(src) {
		return false
	}
	if c := src[i+1]; !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
		return false
	}
	if i == 0 {
		return true
	}
	p := src[i-1]
	return !(isIdent(p) || p == ')' || p == ']' || p == '"' || p == '\'' || p == '`' || p == '.')
}

// closeElement pops the stack through the matching open element.
func closeElement(src string, i int, stack []openEl) []openEl {
	k := i + 2
	for k < len(src) && (isIdent(src[k]) || src[k] == '.' || src[k] == '-' || src[k] == ':') {
		k++
	}
	name := src[i+2 : k]
	for n := len(stack) - 1; n >= 0; n-- {
		if stack[n].tag == name {
			return stack[:n]
		}
	}
	return stack
}

// parseOpenTag reads <Tag attr="x" attr={expr} {...spread}> or the self-closing
// form. It fails (ok=false) on anything that isn't an element, so a stray "<"
// costs nothing.
func parseOpenTag(src string, cm codeMap, i int) (el jsxOpen, ok bool) {
	k := i + 1
	for k < len(src) && (isIdent(src[k]) || src[k] == '.' || src[k] == '-' || src[k] == ':') {
		k++
	}
	el.tag = src[i+1 : k]
	for {
		for k < len(src) && isSpaceByte(src[k]) {
			k++
		}
		if k >= len(src) {
			return el, false
		}
		c := src[k]
		switch {
		case c == '>':
			el.end = k + 1
			return el, true
		case c == '/' && k+1 < len(src) && src[k+1] == '>':
			el.selfClosing, el.end = true, k+2
			return el, true
		case c == '{': // {...props}
			end, found := braceEnd(src, cm, k)
			if !found || !strings.HasPrefix(strings.TrimSpace(src[k+1:end]), "...") {
				return el, false
			}
			el.attrs = append(el.attrs, jsxAttr{kind: attrSpread, value: strings.TrimSpace(src[k+1 : end])})
			k = end + 1
		case isIdent(c):
			s := k
			for k < len(src) && (isIdent(src[k]) || src[k] == '-' || src[k] == ':' || src[k] == '.') {
				k++
			}
			a := jsxAttr{name: src[s:k]}
			m := k
			for m < len(src) && isSpaceByte(src[m]) {
				m++
			}
			if m < len(src) && src[m] == '=' {
				m++
				for m < len(src) && isSpaceByte(src[m]) {
					m++
				}
				if m >= len(src) {
					return el, false
				}
				switch src[m] {
				case '"', '\'':
					e := strings.IndexByte(src[m+1:min(len(src), m+1+maxAttrString)], src[m])
					if e < 0 {
						return el, false
					}
					a.kind, a.value = attrString, src[m+1:m+1+e]
					k = m + 1 + e + 1
				case '{':
					end, found := braceEnd(src, cm, m)
					if !found {
						return el, false
					}
					a.kind, a.value, a.start, a.end = attrExpr, src[m+1:end], m+1, end
					k = end + 1
				default:
					return el, false
				}
			}
			el.attrs = append(el.attrs, a)
		default:
			return el, false
		}
	}
}

func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// braceEnd finds the "}" that closes the "{" at open, counting braces that are
// code only: the mask has already blanked strings, templates and comments.
func braceEnd(src string, cm codeMap, open int) (int, bool) {
	depth := 0
	// An attribute is never this long. Without a limit, thousands of unclosed
	// "<a b={" in one file would each be re-scanned to the end of it.
	for k := open; k < len(src) && k < open+maxAttrExpr; k++ {
		if k < len(cm) && !cm[k] {
			continue
		}
		switch src[k] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return k, true
			}
		}
	}
	return 0, false
}

// triggersOf makes the triggers for one element. handled reports, for a form,
// whether anything is attached to it (a button inside takes that).
func triggersOf(src string, cm codeMap, at int, el jsxOpen, stack []openEl) (made []graph.Trigger, handled bool) {
	line := lineOf(src, at)
	label := labelOf(src, el)
	mk := func(event string, h graph.Handler) graph.Trigger {
		return graph.Trigger{Line: line, Tag: el.tag, Label: label, Event: event, Handler: h}
	}
	byEvent := func() (ts []graph.Trigger) {
		for _, a := range el.attrs {
			if a.kind == attrExpr && eventProp.MatchString(a.name) {
				ts = append(ts, mk(a.name, handlerOf(src, cm, a)))
			}
		}
		return ts
	}
	spread, framework := false, false
	for _, a := range el.attrs {
		spread = spread || a.kind == attrSpread
		framework = framework || frameworkAttr.MatchString(a.name)
	}
	inForm, formHandled := nearestForm(stack)

	switch {
	case el.tag == "button" || el.tag == "input" && inputIsButton(el):
		if a, ok := el.attr("disabled"); ok && a.kind == attrBare {
			return nil, false // inert on purpose
		}
		if ts := byEvent(); len(ts) > 0 {
			return ts, false
		}
		if a, ok := el.attr("formAction"); ok && a.kind == attrExpr {
			return []graph.Trigger{mk("formAction", handlerOf(src, cm, a))}, false
		}
		switch {
		case spread || framework:
			return []graph.Trigger{mk("", graph.Handler{Kind: graph.HandlerUnknown})}, false
		case inForm && formHandled && !isTypeButton(el):
			return []graph.Trigger{mk("", graph.Handler{Kind: graph.HandlerForm})}, false
		}
		return []graph.Trigger{mk("", graph.Handler{Kind: graph.HandlerNone})}, false

	case el.tag == "form":
		made = byEvent()
		if a, ok := el.attr("action"); ok {
			switch a.kind {
			case attrExpr:
				made = append(made, mk("action", handlerOf(src, cm, a)))
			case attrString:
				made = append(made, mk("action", graph.Handler{Kind: graph.HandlerNavigates}))
			}
		}
		if len(made) == 0 {
			kind := graph.HandlerNone
			if _, ok := el.attr("ref"); ok || spread || framework {
				kind = graph.HandlerUnknown
			} else if _, ok := el.attr("method"); ok {
				kind = graph.HandlerNavigates
			}
			made = []graph.Trigger{mk("", graph.Handler{Kind: kind})}
		}
		for _, t := range made {
			handled = handled || t.Handler.Kind != graph.HandlerNone
		}
		return made, handled

	case isLink(el.tag):
		if ts := byEvent(); len(ts) > 0 {
			return ts, false
		}
		if _, ok := el.attr("href"); ok {
			return []graph.Trigger{mk("", graph.Handler{Kind: graph.HandlerNavigates})}, false
		}
		if _, ok := el.attr("to"); ok {
			return []graph.Trigger{mk("", graph.Handler{Kind: graph.HandlerNavigates})}, false
		}
		return nil, false
	}
	return byEvent(), false
}

func isLink(tag string) bool { return tag == "a" || tag == "Link" || tag == "NavLink" }

// inputIsButton: <input type="submit"> and friends act like a button; a text box doesn't.
func inputIsButton(el jsxOpen) bool {
	if a, ok := el.attr("type"); ok && a.kind == attrString {
		switch a.value {
		case "submit", "button", "image":
			return true
		}
	}
	return false
}

// isTypeButton: type="button" never submits the form it sits in.
func isTypeButton(el jsxOpen) bool {
	a, ok := el.attr("type")
	return ok && a.kind == attrString && a.value == "button"
}

func nearestForm(stack []openEl) (in, handled bool) {
	for n := len(stack) - 1; n >= 0; n-- {
		if stack[n].tag == "form" {
			return true, stack[n].handled
		}
	}
	return false, false
}

// handlerOf reads what an attribute's expression runs.
func handlerOf(src string, cm codeMap, a jsxAttr) graph.Handler {
	expr := strings.TrimSpace(a.value)
	h := graph.Handler{Mentions: mentionsIn(src, cm, a.start, a.end)}
	switch {
	case expr == "" || expr == "undefined" || expr == "null" || expr == "false":
		h.Kind, h.Mentions = graph.HandlerNone, nil
	case inlineFunc.MatchString(expr):
		h.Kind = graph.HandlerInline
		lead := len(a.value) - len(strings.TrimLeft(a.value, " \t\r\n"))
		h.From = lineOf(src, a.start+lead)
		h.To = lineOf(src, a.start+len(strings.TrimRight(a.value, " \t\r\n"))-1)
	case nameChain.MatchString(expr):
		h.Kind = graph.HandlerNamed
		h.Name = expr[strings.LastIndexAny(expr, ".")+1:]
	default:
		h.Kind = graph.HandlerUnknown
	}
	return h
}

// mentionsIn lists the identifiers used in src[from:to], in code only, sorted.
func mentionsIn(src string, cm codeMap, from, to int) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range identWord.FindAllStringIndex(src[from:to], -1) {
		w := src[from+m[0] : from+m[1]]
		if len(w) < 2 || jsKeywords[w] || seen[w] || (from+m[0] < len(cm) && !cm[from+m[0]]) {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// labelOf is the words a person sees on an element: its aria-label, else its
// plain text up to the next tag or expression.
func labelOf(src string, el jsxOpen) string {
	if a, ok := el.attr("aria-label"); ok && a.kind == attrString {
		return clip(a.value)
	}
	if el.selfClosing {
		return ""
	}
	k := el.end
	for k < len(src) && src[k] != '<' && src[k] != '{' && k-el.end < 200 {
		k++
	}
	return clip(strings.Join(strings.Fields(src[el.end:k]), " "))
}

func clip(s string) string {
	if r := []rune(s); len(r) > 40 {
		return string(r[:40])
	}
	return s
}
