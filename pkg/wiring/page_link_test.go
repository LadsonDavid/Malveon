package wiring_test

import (
	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
	"github.com/LadsonDavid/Malveon/pkg/wiring"
	"testing"
)

func TestPageLinkClaimUsesActualNavigationNotURLMention(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		linked     bool
	}{
		{"Next Link", `return <Link href="/dashboard/faq">FAQ</Link>;`, true},
		{"anchor", `return <a href="/dashboard/faq?from=members#help">FAQ</a>;`, true},
		{"wrong target", `return <Link href="/dashboard/other">Other</Link>;`, false},
		{"string mention", `const url="/dashboard/faq";return <main>Members</main>;`, false},
		{"dynamic target", `return <Link href={target}>FAQ</Link>;`, false},
		{"comment", `/* <Link href="/dashboard/faq">FAQ</Link> */ return <main>Members</main>;`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := incidentProject(t, map[string]string{
				"app/(dashboard)/dashboard/members/page.tsx": `export default function Members(){` + tc.body + `}`,
				"app/(dashboard)/dashboard/faq/page.tsx":     `export default function FAQ(){return <main>FAQ</main>}`,
				"app/(dashboard)/dashboard/other/page.tsx":   `export default function Other(){return <main>Other</main>}`,
			})
			g, err := extractor.Extract(root)
			if err != nil {
				t.Fatal(err)
			}
			rs := wiring.Run(g, []features.Feature{{Name: "`app/(dashboard)/dashboard/members/page.tsx` links to `/dashboard/faq`"}})
			if len(rs) != 1 {
				t.Fatalf("%+v", rs)
			}
			if tc.linked {
				if rs[0].Verdict == wiring.HalfBuilt || rs[0].Verdict == wiring.Broken {
					t.Fatalf("real page link rejected: %+v", rs[0])
				}
			} else if rs[0].Verdict != wiring.HalfBuilt {
				t.Fatalf("missing link must remain disconnected: %+v", rs[0])
			}
			if len(g.RoutesServing("/dashboard/faq")) != 0 {
				t.Fatal("invented API route for page link")
			}
		})
	}
}

func TestPageLinkDoesNotProveRenderingOrAnotherComponentLink(t *testing.T) {
	root := incidentProject(t, map[string]string{
		"app/members/page.tsx": "export function Members(){\n return <main>Members</main>;\n}\nexport function Other(){\n return <Link href=\"/faq\">FAQ</Link>;\n}\n",
		"app/faq/page.tsx":     `export default function FAQ(){return <main>FAQ</main>}`,
	})
	g, err := extractor.Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, claim := range []string{"`Members` links to `/faq`", "`app/members/page.tsx` renders `/faq`"} {
		rs := wiring.Run(g, []features.Feature{{Name: claim}})
		if len(rs) != 1 || rs[0].Verdict != wiring.HalfBuilt {
			t.Fatalf("unsupported connection accepted for %s: %+v", claim, rs)
		}
	}
}
