package wiring

import (
	"strings"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/features"
)

// A component often doesn't make its request itself: its parent hands it the
// function (<RefundButton onRefund={doRefund} />). A line that names both the
// component's file and the URL used to read HALF BUILT ("nothing connects it
// to route /api/refund"), because the component's own file never reaches the
// call: the parent does. Found 2026-10-19 while testing the dead-button check.
//
// The connection is real when the parent renders the component, gives it a
// handler, and itself reaches the code that makes the request. Anything less
// stays HALF BUILT: a parent that gives no handler, a plain value, a handler
// to a different component, or code that has nothing to do with the request.

const refundFileLine = "`src/components/RefundButton.tsx` calls `POST /api/refund`"

const handedButton = "export default function RefundButton({ onRefund }) {\n  return <button onClick={onRefund}>Refund</button>;\n}\n"

const refundHelper = "export async function doRefund(orderId: string) {\n  await fetch('/api/refund', { method: 'POST', body: JSON.stringify({ orderId }) });\n}\n"

func handedApp(page string, more map[string]string) map[string]string {
	files := map[string]string{
		"src/app/page.tsx":                page,
		"src/components/RefundButton.tsx": handedButton,
		"src/lib/api.ts":                  refundHelper,
		"src/app/api/refund/route.ts":     refundRoute,
	}
	for k, v := range more {
		files[k] = v
	}
	return files
}

func TestParentHandsTheComponentTheRequest(t *testing.T) {
	const head = "import RefundButton from '../components/RefundButton';\nimport { doRefund } from '../lib/api';\n"
	cases := []struct {
		name   string
		page   string
		more   map[string]string
		line   string
		want   Verdict
		reason string
	}{
		// --- the parent assembles it: CONNECTED ---
		{"the parent hands down the helper itself", head + "export default function Home() { return <main><RefundButton onRefund={doRefund} /></main>; }\n", nil,
			refundFileLine, Working, "its parent"},
		{"the original report: the line also says button", head + "export default function Home() { return <main><RefundButton onRefund={doRefund} /></main>; }\n", nil,
			refundLine, Working, ""},
		{"the parent hands down a wrapper around it", head + "export default function Home() {\n  const handleRefund = () => doRefund('1');\n  return <main><RefundButton onRefund={handleRefund} /></main>;\n}\n", nil,
			refundFileLine, Working, "its parent"},
		{"the parent hands down a function written in place", head + "export default function Home() { return <main><RefundButton onRefund={() => doRefund('1')} /></main>; }\n", nil,
			refundFileLine, Working, "its parent"},
		{"the line names the component, not its file", head + "export default function Home() { return <main><RefundButton onRefund={doRefund} /></main>; }\n", nil,
			"`RefundButton` calls `POST /api/refund`", Working, "its parent"},

		// --- close, but nothing connects them: still HALF BUILT ---
		{"the parent gives a handler but has nothing to do with the request",
			"import RefundButton from '../components/RefundButton';\nexport default function Home() { return <main><RefundButton onRefund={() => {}} /></main>; }\n", nil,
			refundFileLine, HalfBuilt, "nothing connects it"},
		{"the parent gives the component no handler", head + "export default function Home() { return <main><RefundButton /></main>; }\n", nil,
			refundFileLine, HalfBuilt, "nothing connects it"},
		{"the parent gives a plain value, not a handler", head + "export default function Home() { return <main><RefundButton label=\"Refund\" /></main>; }\n", nil,
			refundFileLine, HalfBuilt, "nothing connects it"},
		{"the parent explicitly attaches nothing", head + "export default function Home() { return <main><RefundButton onRefund={undefined} /></main>; }\n", nil,
			refundFileLine, HalfBuilt, "nothing connects it"},
		{"the handler goes to another component in the same file as the one the line names",
			head + "import { Sibling } from '../components/RefundButton';\nexport default function Home() { return <main><Sibling onRefund={doRefund} /><RefundButton /></main>; }\n",
			map[string]string{"src/components/RefundButton.tsx": handedButton + "export function Sibling({ onRefund }) {\n  return <button onClick={onRefund}>x</button>;\n}\n"},
			"`RefundButton` calls `POST /api/refund`", HalfBuilt, "nothing connects it"},
		{"the parent makes the request itself and hands it down",
			"import RefundButton from '../components/RefundButton';\nexport default function Home() {\n  const doRefund = async () => { await fetch('/api/refund', { method: 'POST' }); };\n  return <main><RefundButton onRefund={doRefund} /></main>;\n}\n",
			map[string]string{"src/lib/api.ts": "export const unrelated = 1;\n"},
			refundFileLine, Working, "its parent"},
		{"another component with the same name as the file, from another folder",
			"import RefundButton from '../ui/RefundButton';\nimport { doRefund } from '../lib/api';\nexport default function Home() { return <main><RefundButton onRefund={doRefund} /></main>; }\n",
			map[string]string{
				"src/ui/RefundButton.tsx":         handedButton,
				"src/components/RefundButton.tsx": "export default function Pay() {\n  return <button>Pay</button>;\n}\n",
			},
			refundFileLine, HalfBuilt, "nothing connects it"},
		// A known limit, pinned so nobody mistakes it for a pass: the parent imports
		// the component under another name, and malveon doesn't record local names.
		{"the parent imports the component under another name (a known limit)",
			"import Refund from '../components/RefundButton';\nimport { doRefund } from '../lib/api';\nexport default function Home() { return <main><Refund onRefund={doRefund} /></main>; }\n", nil,
			refundFileLine, HalfBuilt, "nothing connects it"},
		{"the handler goes to a different component",
			"import RefundButton from '../components/RefundButton';\nimport Other from '../components/Other';\nimport { doRefund } from '../lib/api';\n" +
				"export default function Home() { return <main><Other onRefund={doRefund} /><RefundButton /></main>; }\n",
			map[string]string{"src/components/Other.tsx": "export default function Other({ onRefund }) {\n  return <button onClick={onRefund}>x</button>;\n}\n"},
			refundFileLine, HalfBuilt, "nothing connects it"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := graphOf(t, handedApp(c.page, c.more))
			r := Run(g, []features.Feature{{ID: "x", Name: c.line}})[0]
			if r.Verdict != c.want {
				t.Fatalf("verdict %s, want %s\nreason: %s", r.Verdict, c.want, r.Reason)
			}
			if !strings.Contains(r.Reason, c.reason) {
				t.Errorf("reason %q is missing %q", r.Reason, c.reason)
			}
		})
	}
}
