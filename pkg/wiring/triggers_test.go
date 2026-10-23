package wiring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
	"github.com/LadsonDavid/Malveon/pkg/graph"
)

// "I click the button and nothing happens." A request that exists in a file
// proves nothing if no button reaches it. These tests write small Next.js
// projects to disk, run the real extractor, and check the verdict for a plan
// line that talks about a button.

const refundLine = "The Refund button in `src/components/RefundButton.tsx` calls `POST /api/refund`"

const refundRoute = "export async function POST(req: Request) {\n  const { orderId } = await req.json();\n  return Response.json({ ok: true, orderId });\n}\n"

const refundPage = "import RefundButton from '../components/RefundButton';\nexport default function Home() { return <main><RefundButton /></main>; }\n"

func graphOf(t *testing.T, files map[string]string) *graph.Graph {
	t.Helper()
	root := t.TempDir()
	for rel, src := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g, err := extractor.Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// refundApp is the app with only RefundButton.tsx's content to choose.
func refundApp(button string, more map[string]string) map[string]string {
	files := map[string]string{
		"src/app/page.tsx":                refundPage,
		"src/app/api/refund/route.ts":     refundRoute,
		"src/components/RefundButton.tsx": button,
	}
	for k, v := range more {
		files[k] = v
	}
	return files
}

func TestButtonReachesTheCall(t *testing.T) {
	post := "await fetch('/api/refund', { method: 'POST', body: JSON.stringify({ orderId }) });"
	cases := []struct {
		name    string
		files   map[string]string
		line    string
		want    Verdict
		reason  string // must appear in the reason
		pathHas string // must appear in the path
	}{
		// --- provably dead: nothing in the app calls the function that makes the request ---
		{"an exported helper nothing calls, and a button with no handler",
			refundApp("export async function doRefund(orderId: string) {\n  "+post+"\n}\nexport default function RefundButton() {\n  return <button className=\"btn\">Refund</button>;\n}\n", nil),
			refundLine, HalfBuilt, "nothing in the app calls `doRefund`", ""},
		{"a handler defined inside the component and never attached",
			refundApp("export default function RefundButton() {\n  const handleRefund = async () => {\n    "+post+"\n  };\n  return <button className=\"btn\">Refund</button>;\n}\n", nil),
			refundLine, HalfBuilt, "a function nothing in the project calls", ""},
		{"the button runs something else",
			refundApp("export async function doRefund(orderId: string) {\n  "+post+"\n}\nexport default function RefundButton() {\n  return <button onClick={() => console.log('refund')}>Refund</button>;\n}\n", nil),
			refundLine, HalfBuilt, "nothing in the app calls `doRefund`", ""},
		{"the helper is in another file that nobody uses",
			refundApp("export default function RefundButton() {\n  return <button>Refund</button>;\n}\n",
				map[string]string{"src/lib/api.ts": "export async function doRefund(orderId: string) {\n  " + post + "\n}\n"}),
			"The Refund button calls `POST /api/refund`", HalfBuilt, "nothing in the app calls `doRefund`", ""},
		{"imported but never used",
			refundApp("import { doRefund } from '../lib/api';\nexport default function RefundButton() {\n  return <button>Refund</button>;\n}\n",
				map[string]string{"src/lib/api.ts": "export async function doRefund(orderId: string) {\n  " + post + "\n}\n"}),
			refundLine, HalfBuilt, "nothing in the app calls `doRefund`", ""},

		// --- reached by a button: stays CONNECTED, and the path says how ---
		{"onClick calls the helper",
			refundApp("export async function doRefund(orderId: string) {\n  "+post+"\n}\nexport default function RefundButton() {\n  return <button onClick={() => doRefund('1')}>Refund</button>;\n}\n", nil),
			refundLine, Working, "", "onClick"},
		{"onClick is the function that makes the request",
			refundApp("export default function RefundButton() {\n  const handleRefund = async () => {\n    "+post+"\n  };\n  return <button onClick={handleRefund}>Refund</button>;\n}\n", nil),
			refundLine, Working, "", "onClick"},
		{"the request is written inside onClick",
			refundApp("export default function RefundButton() {\n  return <button onClick={() => fetch('/api/refund', { method: 'POST' })}>Refund</button>;\n}\n", nil),
			refundLine, Working, "", "onClick"},
		{"the page hands the helper to the button as a prop",
			map[string]string{
				"src/app/page.tsx": "import RefundButton from '../components/RefundButton';\nimport { doRefund } from '../lib/api';\n" +
					"export default function Home() { return <main><RefundButton onRefund={doRefund} /></main>; }\n",
				"src/components/RefundButton.tsx": "export default function RefundButton({ onRefund }) {\n  return <button onClick={onRefund}>Refund</button>;\n}\n",
				"src/lib/api.ts":                  "export async function doRefund(orderId: string) {\n  " + post + "\n}\n",
				"src/app/api/refund/route.ts":     refundRoute,
			},
			// The line doesn't name RefundButton.tsx: naming it brings in an older rule
			// ("two named things must be connected") that reads a prop-passed handler as
			// HALF BUILT. That is a separate, existing false positive; this case is about
			// whether the button reaches the call.
			"The Refund button calls `POST /api/refund`", Working, "", ""},
		{"a form's onSubmit reaches the request",
			map[string]string{
				"src/app/page.tsx": "import Signup from '../components/Signup';\nexport default function Home() { return <Signup />; }\n",
				"src/components/Signup.tsx": "export default function Signup() {\n  const submit = async () => { await fetch('/api/signup', { method: 'POST' }); };\n" +
					"  return <form onSubmit={submit}><button type=\"submit\">Join</button></form>;\n}\n",
				"src/app/api/signup/route.ts": "export async function POST() { return Response.json({ ok: true }); }\n",
			},
			"The signup form posts to `POST /api/signup`", Working, "", "onSubmit"},

		// --- not provable: never flagged ---
		{"the function is used somewhere that isn't a button, and the named button has no handler",
			refundApp("import { useEffect } from 'react';\nexport async function doRefund(orderId: string) {\n  "+post+"\n}\nexport default function RefundButton() {\n  useEffect(() => { doRefund('1'); }, []);\n  return <button>Refund</button>;\n}\n", nil),
			refundLine, HalfBuilt, "has no click handler", ""},
		{"the helper is imported under another name",
			refundApp("import { doRefund as refund } from '../lib/api';\nexport default function RefundButton() {\n  return <button onClick={() => refund('1')}>Refund</button>;\n}\n",
				map[string]string{"src/lib/api.ts": "export async function doRefund(orderId: string) {\n  " + post + "\n}\n"}),
			refundLine, Working, "", ""},
		{"a line that doesn't talk about a button isn't judged by its buttons",
			refundApp("export async function doRefund(orderId: string) {\n  "+post+"\n}\nexport default function RefundButton() {\n  return <button>Refund</button>;\n}\n", nil),
			"`POST /api/refund` is served and called from `src/components/RefundButton.tsx`", Working, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := graphOf(t, c.files)
			r := Run(g, []features.Feature{{ID: "x", Name: c.line}})[0]
			path := strings.Join(r.Path, " → ")
			if r.Verdict != c.want {
				t.Fatalf("verdict %s, want %s\nreason: %s\npath: %s", r.Verdict, c.want, r.Reason, path)
			}
			if !strings.Contains(r.Reason, c.reason) {
				t.Errorf("reason %q is missing %q", r.Reason, c.reason)
			}
			if !strings.Contains(path, c.pathHas) {
				t.Errorf("path %q is missing %q", path, c.pathHas)
			}
			// The path keeps its four hops (the editor labels them by position).
			if c.want == Working && len(r.Path) != 4 && len(r.Path) != 2 {
				t.Errorf("path has %d hops: %v", len(r.Path), r.Path)
			}
		})
	}
}

// The reason names the button, so a person can find it.
func TestDeadButtonReasonNamesTheButton(t *testing.T) {
	g := graphOf(t, refundApp("export async function doRefund(orderId: string) {\n  await fetch('/api/refund', { method: 'POST' });\n}\n"+
		"export default function RefundButton() {\n  return <button className=\"btn\">Refund</button>;\n}\n", nil))
	r := Run(g, []features.Feature{{ID: "x", Name: refundLine}})[0]
	for _, want := range []string{"`<button>` \"Refund\"", "has no click handler", "RefundButton.tsx"} {
		if !strings.Contains(r.Reason, want) {
			t.Errorf("reason %q is missing %q", r.Reason, want)
		}
	}
}

// The patterns real apps use to wire a button. Every one of them works, so every
// one must stay CONNECTED: a wrong "dead button" blocks a commit for nothing.
// (T4 of the prevention-gate plan; if a fix for a miss ever turns one of these
// into HALF BUILT, the fix is wrong.)
func TestWorkingButtonsStayConnected(t *testing.T) {
	post := "await fetch('/api/refund', { method: 'POST', body: JSON.stringify({ orderId }) });"
	helper := "export async function doRefund(orderId: string) {\n  " + post + "\n}\n"
	cases := []struct {
		name   string
		button string            // contents of RefundButton.tsx
		more   map[string]string // other files
	}{
		{"onClick names the helper", helper + "export default function RefundButton() {\n  return <button onClick={doRefund}>Refund</button>;\n}\n", nil},
		{"onClick calls it with an argument", helper + "export default function RefundButton({ id }) {\n  return <button onClick={() => doRefund(id)}>Refund</button>;\n}\n", nil},
		{"an async handler that refreshes after", helper + "export default function RefundButton({ id, router }) {\n  return <button onClick={async () => { await doRefund(id); router.refresh(); }}>Refund</button>;\n}\n", nil},
		{"useCallback", "import { useCallback } from 'react';\n" + helper + "export default function RefundButton({ id }) {\n  const onClick = useCallback(() => doRefund(id), [id]);\n  return <button onClick={onClick}>Refund</button>;\n}\n", nil},
		{"react-hook-form", helper + "export default function RefundButton({ handleSubmit }) {\n  const onValid = (v) => doRefund(v.id);\n  return <form onSubmit={handleSubmit(onValid)}><button type=\"submit\">Refund</button></form>;\n}\n", nil},
		{"a custom hook returns the handler", "import { useRefund } from '../hooks/useRefund';\nexport default function RefundButton() {\n  const { refund } = useRefund();\n  return <button onClick={refund}>Refund</button>;\n}\n",
			map[string]string{"src/hooks/useRefund.ts": "export function useRefund() {\n  const refund = async () => {\n    await fetch('/api/refund', { method: 'POST' });\n  };\n  return { refund };\n}\n"}},
		{"react-query mutation", "import { useMutation } from '@tanstack/react-query';\n" + helper + "export default function RefundButton({ id }) {\n  const m = useMutation({ mutationFn: doRefund });\n  return <button onClick={() => m.mutate(id)}>Refund</button>;\n}\n", nil},
		{"a form action function", "async function refundAction() {\n  " + post + "\n}\nexport default function RefundButton() {\n  return <form action={refundAction}><button>Refund</button></form>;\n}\n", nil},
		{"a div that acts as a button", helper + "export default function RefundButton() {\n  return <div role=\"button\" onClick={doRefund}>Refund</div>;\n}\n", nil},
		{"a component library button", "import { Button } from '@/components/ui/button';\n" + helper + "export default function RefundButton({ id }) {\n  return <Button onClick={() => doRefund(id)}>Refund</Button>;\n}\n", nil},
		{"the parent passes the handler down as a prop",
			"export default function RefundButton({ onRefund }) {\n  return <button onClick={onRefund}>Refund</button>;\n}\n",
			map[string]string{"src/app/page.tsx": "import RefundButton from '../components/RefundButton';\nimport { doRefund } from '../lib/api';\nexport default function Home({ id }) { return <RefundButton onRefund={() => doRefund(id)} />; }\n",
				"src/lib/api.ts": helper}},
		{"debounced", "import { useMemo } from 'react';\nimport { debounce } from 'lodash';\n" + helper + "export default function RefundButton({ id }) {\n  const go = useMemo(() => debounce(() => doRefund(id), 300), [id]);\n  return <button onClick={go}>Refund</button>;\n}\n", nil},
		{"handlers kept in an object", helper + "const handlers = { refund: doRefund };\nexport default function RefundButton() {\n  return <button onClick={handlers.refund}>Refund</button>;\n}\n", nil},
		{"a conditional handler", helper + "export default function RefundButton({ isAdmin }) {\n  return <button onClick={isAdmin ? doRefund : undefined}>Refund</button>;\n}\n", nil},
		{"spread props from a helper", helper + "export default function RefundButton({ getButtonProps }) {\n  return <button {...getButtonProps({ onClick: doRefund })}>Refund</button>;\n}\n", nil},
		{"a class component", "import React from 'react';\nexport default class RefundButton extends React.Component {\n  handleRefund = async () => {\n    " + post + "\n  };\n  render() {\n    return <button onClick={this.handleRefund}>Refund</button>;\n  }\n}\n", nil},
		{"a generic arrow helper", "export const doRefund = async <T,>(orderId: T) => {\n  " + post + "\n};\nexport default function RefundButton() {\n  return <button onClick={() => doRefund('1')}>Refund</button>;\n}\n", nil},
		{"JSX text with an apostrophe on the same line", helper + "export default function RefundButton() {\n  return <p>Don't wait <button onClick={doRefund}>Refund</button></p>;\n}\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := graphOf(t, refundApp(c.button, c.more))
			r := Run(g, []features.Feature{{ID: "x", Name: "The Refund button calls `POST /api/refund`"}})[0]
			if r.Verdict != Working {
				t.Fatalf("a working button read %s: %s", r.Verdict, r.Reason)
			}
		})
	}
}

// Two unrelated functions can share a name (handleDelete, handleSubmit). The
// path must show the button that really runs the request, never a stranger's
// button in another file that happens to name a function the same way. (Found on
// the a real Next.js app app: "Post a Job" was shown as triggered by a form on the
// challenge-submit page.)
func TestPathShowsTheButtonOfTheSameScreen(t *testing.T) {
	deleteRoute := map[string]string{"src/app/api/jobs/[id]/route.ts": "export async function DELETE() { return Response.json({ ok: true }); }\n"}
	adminWithButton := "export default function Admin() {\n  const handleDelete = async () => {\n    await fetch('/api/jobs/1', { method: 'DELETE' });\n  };\n  return <button onClick={handleDelete}>Delete job</button>;\n}\n"
	adminNoButton := "export default function Admin() {\n  const handleDelete = async () => {\n    await fetch('/api/jobs/1', { method: 'DELETE' });\n  };\n  return <p>Jobs</p>;\n}\n"
	feed := "export default function Feed() {\n  const handleDelete = () => console.log('x');\n  return <button onClick={handleDelete}>Remove post</button>;\n}\n"
	line := "The Delete job button calls `DELETE /api/jobs/[id]`"
	pathOf := func(t *testing.T, admin string) (Result, string) {
		files := map[string]string{"src/app/admin/page.tsx": admin, "src/app/feed/page.tsx": feed}
		for k, v := range deleteRoute {
			files[k] = v
		}
		r := Run(graphOf(t, files), []features.Feature{{ID: "x", Name: line}})[0]
		return r, strings.Join(r.Path, " → ")
	}

	t.Run("the button in the same file is the one shown", func(t *testing.T) {
		r, path := pathOf(t, adminWithButton)
		if r.Verdict != Working {
			t.Fatalf("verdict %s: %s", r.Verdict, r.Reason)
		}
		if !strings.Contains(path, "onClick on <button> \"Delete job\"") || strings.Contains(path, "feed") {
			t.Errorf("path should show the admin page's button, not the feed's: %s", path)
		}
	})
	t.Run("a same-named handler in an unrelated file is not a reason to show a button", func(t *testing.T) {
		r, path := pathOf(t, adminNoButton)
		if r.Verdict != Working {
			t.Fatalf("verdict %s: %s", r.Verdict, r.Reason)
		}
		if strings.Contains(path, "onClick") || strings.Contains(path, "feed") {
			t.Errorf("no button of this screen runs it, so the path must not name one: %s", path)
		}
	})
}

// A dead button is dead even when something else in the app can send the request.
// (Found 2026-10-22 by running the binary: `<button>Refund</button>` with no handler
// read CONNECTED, and the run VERIFIED BY CODE, because `doRefund` was used by a link
// elsewhere. The plan line talks about *the Refund button*, so that button is the
// thing to prove.) Every "stays CONNECTED" case here is a button that really works.
func TestTheNamedButtonItselfMustBeWired(t *testing.T) {
	elsewhere := map[string]string{"src/components/Elsewhere.tsx": "import { doRefund } from './RefundButton';\nexport default function Elsewhere() { return <a onClick={doRefund}>Undo</a>; }\n"}
	post := "await fetch('/api/refund', { method: 'POST', body: JSON.stringify({ orderId }) });"
	helper := "export async function doRefund(orderId: string) {\n  " + post + "\n}\n"
	cases := []struct {
		name   string
		button string
		more   map[string]string
		want   Verdict
		reason string
	}{
		// --- the button the line names does nothing ---
		{"no handler, the helper is used by a link elsewhere",
			helper + "export default function RefundButton() {\n  return <button>Refund</button>;\n}\n",
			map[string]string{"src/components/Elsewhere.tsx": "import { doRefund } from './RefundButton';\nexport default function Elsewhere() { return <a onClick={doRefund}>Undo</a>; }\n"},
			HalfBuilt, "has no click handler"},
		{"an empty handler",
			helper + "export default function RefundButton() {\n  return <button onClick={() => {}}>Refund</button>;\n}\n",
			map[string]string{"src/lib/admin.ts": "import { doRefund } from '../components/RefundButton';\nexport const tools = { doRefund };\n"},
			HalfBuilt, "does nothing"},
		{"no handler, the request is sent when the page loads",
			"import { useEffect } from 'react';\n" + helper + "export default function RefundButton() {\n  useEffect(() => { doRefund('1'); }, []);\n  return <button>Refund</button>;\n}\n", nil,
			HalfBuilt, "has no click handler"},

		// --- the button works, or malveon cannot tell: never flagged ---
		{"a wrapper around the button handles the click (it bubbles)",
			helper + "export default function RefundButton() {\n  return <div onClick={doRefund}><button>Refund</button></div>;\n}\n", nil, Working, ""},
		{"the form around the button handles the submit",
			helper + "export default function RefundButton() {\n  return <form onSubmit={doRefund}><button>Refund</button></form>;\n}\n", nil, Working, ""},
		{"a different button has no handler, the Refund button works",
			helper + "export default function RefundButton() {\n  return <div><button onClick={doRefund}>Refund</button><button>Cancel</button></div>;\n}\n", nil, Working, ""},
		{"one Refund button works, one does not (two screens' worth of buttons)",
			helper + "export default function RefundButton() {\n  return <div><button>Refund</button><button onClick={doRefund}>Refund</button></div>;\n}\n", nil, Working, ""},
		{"props are spread onto the button (malveon cannot read them)",
			helper + "export default function RefundButton(props) {\n  return <button {...props}>Refund</button>;\n}\n", elsewhere, Working, ""},
		{"the click is wired by a listener, not by React",
			helper + "export default function RefundButton() {\n  document.addEventListener('click', () => doRefund('1'));\n  return <button>Refund</button>;\n}\n", nil, Working, ""},
		{"another element has a handler malveon cannot read (it may run the request)",
			helper + "export default function RefundButton({ pick }) {\n  return <div><a onClick={pick()}>Undo</a><button>Refund</button></div>;\n}\n", elsewhere, Working, ""},
		{"another element in the file is spread (it may hold the handler)",
			helper + "export default function RefundButton(props) {\n  return <div {...props}><button>Refund</button></div>;\n}\n", elsewhere, Working, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := graphOf(t, refundApp(c.button, c.more))
			r := Run(g, []features.Feature{{ID: "x", Name: refundLine}})[0]
			if r.Verdict != c.want {
				t.Fatalf("verdict %s, want %s\nreason: %s", r.Verdict, c.want, r.Reason)
			}
			if !strings.Contains(r.Reason, c.reason) {
				t.Errorf("reason %q is missing %q", r.Reason, c.reason)
			}
		})
	}
}
