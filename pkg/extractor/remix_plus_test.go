package extractor

import (
	"reflect"
	"testing"
)

// documenso: app/routes/api+/preferred-team.tsx is /api/preferred-team (remix-flat-routes hybrid
// folders); without it, the page's POST to that address looked like a call to nowhere.
func TestFlatRoutesPlusFolders(t *testing.T) {
	cases := []struct {
		rest []string
		want []string
	}{
		{[]string{"api+", "preferred-team.tsx"}, []string{"/api/preferred-team"}},
		{[]string{"_authenticated+", "admin+", "_layout.tsx"}, []string{"/admin"}},
		{[]string{"_authenticated+", "admin+", "users.$id.tsx"}, []string{"/admin/users/:id"}},
	}
	for _, c := range cases {
		if got := flatRoutePaths(c.rest); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%v: got %v, want %v", c.rest, got, c.want)
		}
	}
}
