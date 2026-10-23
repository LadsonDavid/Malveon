package extractor

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// wantTrigger is what one trigger must look like. Empty fields aren't checked,
// except mentions: nil isn't checked, an empty non-nil slice must be empty.
type wantTrigger struct {
	tag, event string
	kind       graph.HandlerKind
	name       string
	mentions   []string
	label      string
	line       int
	from, to   int // Inline only
}

// inComponent puts JSX on line 3 of a small component, so a test can say which
// line a trigger is on.
func inComponent(jsx string) string {
	return "export default function C() {\n  return (\n" + jsx + "\n  );\n}\n"
}

func describeTriggers(ts []graph.Trigger) string {
	var b strings.Builder
	for _, t := range ts {
		fmt.Fprintf(&b, "  <%s> line %d event=%q kind=%s name=%q mentions=%v label=%q span=%d-%d\n",
			t.Tag, t.Line, t.Event, t.Handler.Kind, t.Handler.Name, t.Handler.Mentions, t.Label, t.Handler.From, t.Handler.To)
	}
	return b.String()
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestScanTriggers(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []wantTrigger
	}{
		// --- a button and what it is wired to ---
		{"a button with nothing attached", inComponent(`<button className="btn">Refund</button>`),
			[]wantTrigger{{tag: "button", kind: graph.HandlerNone, label: "Refund", line: 3}}},
		{"onClick names a function", inComponent(`<button onClick={doRefund}>Refund</button>`),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerNamed, name: "doRefund", mentions: []string{"doRefund"}, label: "Refund"}}},
		{"onClick is a function written in place", inComponent(`<button onClick={() => doRefund('1')}>Refund</button>`),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerInline, mentions: []string{"doRefund"}, from: 3, to: 3}}},
		{"an inline handler over several lines, with > inside it",
			inComponent("<button\n  onClick={async () => {\n    if (a > b) await doRefund(id);\n  }}\n>\n  Refund\n</button>"),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerInline, mentions: []string{"doRefund", "id"}, label: "Refund", line: 3, from: 4, to: 6}}},
		{"a member: onClick={handlers.save}", inComponent(`<button onClick={handlers.save}>x</button>`),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerNamed, name: "save", mentions: []string{"handlers", "save"}}}},
		{"a handler passed in as a prop", inComponent(`<button onClick={props.onClick}>x</button>`),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerNamed, name: "onClick", mentions: []string{"onClick", "props"}}}},
		{"onClick={undefined} attaches nothing", inComponent(`<button onClick={undefined}>x</button>`),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerNone}}},
		{"onClick={null} attaches nothing", inComponent(`<button onClick={null}>x</button>`),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerNone}}},
		{"an empty function does nothing but is a handler", inComponent(`<button onClick={() => {}}>x</button>`),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerInline, mentions: []string{}}}},
		{"spread props could carry a handler", inComponent(`<button {...props}>Go</button>`),
			[]wantTrigger{{tag: "button", kind: graph.HandlerUnknown, label: "Go"}}},
		{"a ternary can't be read as one function", inComponent(`<button onClick={ok ? yes : no}>x</button>`),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerUnknown, mentions: []string{"no", "ok", "yes"}}}},
		{"a call that makes the handler can't be read", inComponent(`<button onClick={makeHandler(id)}>x</button>`),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerUnknown, mentions: []string{"id", "makeHandler"}}}},
		{"htmx attribute", inComponent(`<button hx-post="/x">x</button>`),
			[]wantTrigger{{tag: "button", kind: graph.HandlerUnknown}}},
		{"data-action may be wired by a listener elsewhere", inComponent(`<button data-action="refund">x</button>`),
			[]wantTrigger{{tag: "button", kind: graph.HandlerUnknown}}},
		{"a disabled button is inert on purpose", inComponent(`<button disabled>Soon</button>`), nil},
		{"the label can come from aria-label", inComponent(`<button aria-label="Close" onClick={close}><XIcon /></button>`),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerNamed, name: "close", label: "Close"}}},

		// --- forms: the button inside takes the form's handler ---
		{"a submit button outside any form", inComponent(`<button type="submit">Save</button>`),
			[]wantTrigger{{tag: "button", kind: graph.HandlerNone, label: "Save"}}},
		{"form onSubmit", inComponent(`<form onSubmit={handleSubmit}><button type="submit">Save</button></form>`),
			[]wantTrigger{
				{tag: "form", event: "onSubmit", kind: graph.HandlerNamed, name: "handleSubmit"},
				{tag: "button", kind: graph.HandlerForm, label: "Save"}}},
		{"form action is a function", inComponent(`<form action={saveAction}><button>Save</button></form>`),
			[]wantTrigger{
				{tag: "form", event: "action", kind: graph.HandlerNamed, name: "saveAction"},
				{tag: "button", kind: graph.HandlerForm}}},
		{"form action is a URL", inComponent(`<form action="/api/save" method="post"><button>Save</button></form>`),
			[]wantTrigger{
				{tag: "form", event: "action", kind: graph.HandlerNavigates},
				{tag: "button", kind: graph.HandlerForm}}},
		{"a form with nothing attached", inComponent(`<form><button>Save</button></form>`),
			[]wantTrigger{
				{tag: "form", kind: graph.HandlerNone},
				{tag: "button", kind: graph.HandlerNone}}},
		{"type=button inside a form doesn't submit it", inComponent(`<form onSubmit={go}><button type="button">Cancel</button></form>`),
			[]wantTrigger{
				{tag: "form", event: "onSubmit", kind: graph.HandlerNamed, name: "go"},
				{tag: "button", kind: graph.HandlerNone, label: "Cancel"}}},
		{"a button after a form closed isn't in it", inComponent(`<form onSubmit={go}></form><button>Save</button>`),
			[]wantTrigger{
				{tag: "form", event: "onSubmit", kind: graph.HandlerNamed, name: "go"},
				{tag: "button", kind: graph.HandlerNone}}},

		// --- links, inputs, and anything given an event handler ---
		{"a link with an href goes somewhere", inComponent(`<a href="/about">About</a>`),
			[]wantTrigger{{tag: "a", kind: graph.HandlerNavigates, label: "About"}}},
		{"Link with an href", inComponent(`<Link href="/x">X</Link>`),
			[]wantTrigger{{tag: "Link", kind: graph.HandlerNavigates, label: "X"}}},
		{"an anchor with onClick and no href", inComponent(`<a onClick={go}>x</a>`),
			[]wantTrigger{{tag: "a", event: "onClick", kind: graph.HandlerNamed, name: "go"}}},
		{"a div with onClick", inComponent(`<div onClick={toggle}>x</div>`),
			[]wantTrigger{{tag: "div", event: "onClick", kind: graph.HandlerNamed, name: "toggle"}}},
		{"two handlers on one element make two triggers", inComponent(`<div onClick={aa} onKeyDown={bb}>x</div>`),
			[]wantTrigger{
				{tag: "div", event: "onClick", kind: graph.HandlerNamed, name: "aa"},
				{tag: "div", event: "onKeyDown", kind: graph.HandlerNamed, name: "bb"}}},
		{"a component given a callback prop", inComponent(`<Modal onClose={close} title="x" />`),
			[]wantTrigger{{tag: "Modal", event: "onClose", kind: graph.HandlerNamed, name: "close"}}},
		{"a component with an inline onClick", inComponent(`<Button onClick={() => save()}>Go</Button>`),
			[]wantTrigger{{tag: "Button", event: "onClick", kind: graph.HandlerInline, mentions: []string{"save"}, label: "Go"}}},
		{"a dotted component name", inComponent(`<Foo.Bar onClick={go} />`),
			[]wantTrigger{{tag: "Foo.Bar", event: "onClick", kind: graph.HandlerNamed, name: "go"}}},
		{"input type=submit outside a form", inComponent(`<input type="submit" value="Go" />`),
			[]wantTrigger{{tag: "input", kind: graph.HandlerNone}}},
		{"an input with onChange", inComponent(`<input type="text" onChange={e => setQ(e.target.value)} />`),
			[]wantTrigger{{tag: "input", event: "onChange", kind: graph.HandlerInline, mentions: []string{"setQ", "target", "value"}}}},

		// --- where JSX can sit ---
		{"after &&", inComponent(`{ok && <button onClick={go}>x</button>}`),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerNamed, name: "go"}}},
		{"after return, on the same line", "function C() {\n  return <button onClick={go}>x</button>;\n}\n",
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerNamed, name: "go", line: 2}}},
		{"inside a map callback", inComponent(`{items.map(i => <li key={i}><button onClick={() => del(i)}>x</button></li>)}`),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerInline, mentions: []string{"del"}}}},
		{"a template literal and a > inside other attributes",
			inComponent("<button className={`btn ${on ? \"a\" : \"b\"}`} title=\"a > b\" onClick={go}>x</button>"),
			[]wantTrigger{{tag: "button", event: "onClick", kind: graph.HandlerNamed, name: "go"}}},
		{"line numbers follow the element", inComponent("<div>\n<button onClick={one}>1</button>\n<button onClick={two}>2</button>\n</div>"),
			[]wantTrigger{
				{tag: "button", event: "onClick", kind: graph.HandlerNamed, name: "one", line: 4},
				{tag: "button", event: "onClick", kind: graph.HandlerNamed, name: "two", line: 5}}},

		// --- not JSX at all ---
		{"comparisons and generics are not elements",
			"const n = a < b && c > d;\nconst [s] = useState<string>('');\nconst xs: Array<Item> = [];\nconst f = <T,>(x: T) => x;\n", nil},
		{"elements in comments and strings are not elements",
			"// <button onClick={x}>\nconst s = \"<button onClick={y}>\";\n/* <form onSubmit={z}> */\n", nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := scanTriggers(c.src)
			if len(got) != len(c.want) {
				t.Fatalf("got %d triggers, want %d:\n%s", len(got), len(c.want), describeTriggers(got))
			}
			for i, w := range c.want {
				g := got[i]
				var bad []string
				if g.Tag != w.tag {
					bad = append(bad, fmt.Sprintf("tag %q, want %q", g.Tag, w.tag))
				}
				if g.Event != w.event {
					bad = append(bad, fmt.Sprintf("event %q, want %q", g.Event, w.event))
				}
				if g.Handler.Kind != w.kind {
					bad = append(bad, fmt.Sprintf("kind %q, want %q", g.Handler.Kind, w.kind))
				}
				if w.name != "" && g.Handler.Name != w.name {
					bad = append(bad, fmt.Sprintf("name %q, want %q", g.Handler.Name, w.name))
				}
				if w.mentions != nil && !sameStrings(g.Handler.Mentions, w.mentions) {
					bad = append(bad, fmt.Sprintf("mentions %v, want %v", g.Handler.Mentions, w.mentions))
				}
				if w.label != "" && g.Label != w.label {
					bad = append(bad, fmt.Sprintf("label %q, want %q", g.Label, w.label))
				}
				if w.line != 0 && g.Line != w.line {
					bad = append(bad, fmt.Sprintf("line %d, want %d", g.Line, w.line))
				}
				if w.from != 0 && (g.Handler.From != w.from || g.Handler.To != w.to) {
					bad = append(bad, fmt.Sprintf("span %d-%d, want %d-%d", g.Handler.From, g.Handler.To, w.from, w.to))
				}
				if len(bad) > 0 {
					t.Errorf("trigger %d: %s\n%s", i, strings.Join(bad, "; "), describeTriggers(got))
				}
			}
		})
	}
}

// A file with no JSX costs nothing, and a file that only looks like JSX in
// places must not hang the scan.
func TestScanTriggersTerminatesOnBrokenInput(t *testing.T) {
	for _, src := range []string{
		"",
		"<",
		"<button",
		"<button onClick={",
		"<button onClick={() => {",
		"<button onClick=\"unterminated",
		"<<<>>>",
		"x < y > z < w",
		strings.Repeat("<div ", 5000),
	} {
		scanTriggers(src) // must return
	}
}

// Broken or hostile input must not make the scan quadratic: an "<a b={" that
// never closes used to be re-scanned to the end of the file by every one after it.
func TestScanTriggersIsNotQuadratic(t *testing.T) {
	if testing.Short() {
		t.Skip("wall-clock limit; slow on loaded machines. Runs in the full test run.")
	}
	for name, src := range map[string]string{
		"thousands of unclosed expressions": strings.Repeat("<a b={ ", 40000),
		"thousands of unclosed strings":     strings.Repeat("<a b=\" ", 40000),
		"thousands of real elements":        strings.Repeat("<div className=\"x\" onClick={go}>y</div>\n", 20000),
	} {
		done := make(chan struct{})
		start := time.Now()
		go func() { scanTriggers(src); close(done) }()
		select {
		case <-done:
			t.Logf("%s: %d bytes in %s", name, len(src), time.Since(start).Round(time.Millisecond))
		case <-time.After(time.Second):
			t.Fatalf("%s: still scanning after 1s (%d bytes)", name, len(src))
		}
	}
}
