package wiring_test

import (
	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
	"github.com/LadsonDavid/Malveon/pkg/wiring"
	"testing"
)

func TestLiteralGETFormReachesDynamicPageWithoutInventingAPIHandler(t *testing.T) {
	root := incidentProject(t, map[string]string{
		"app/(dashboard)/members/[id]/page.tsx": `export default function Member(){return <main>Member</main>}`,
		"app/search/page.tsx":                   `export default function Search(){return <form action="/members/42" method="get"><button>Find</button></form>}`,
	})
	g, err := extractor.Extract(root)
	if err != nil {
		t.Fatal(err)
	}
	rs := wiring.Run(g, []features.Feature{{Name: "`app/search/page.tsx` exists"}})
	if len(rs) != 1 || rs[0].Verdict == wiring.Broken {
		t.Fatalf("%+v", rs)
	}
	// Page navigation must not fabricate a RouteHandler in the API inventory.
	if len(g.RoutesServing("/members/42")) != 0 {
		t.Fatal("invented API handler")
	}
}
