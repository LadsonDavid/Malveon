package extractor

import "testing"

func TestPageNavigationRequiresLiteralGETForm(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want bool
	}{
		{`const f=<form action="/members" method="get"/>;`, true},
		{`const f=<form action="/members"/>;`, true},
		{`const f=<form action="/members" method="post"/>;`, false},
		{`const f=<form action="/members" method={verb}/>;`, false},
		{`const f=fetch('/members');`, false},
	} {
		sites := scanJSLike(tc.src)
		if len(sites) != 1 {
			t.Fatalf("%s: want one extracted call, got %+v", tc.src, sites)
		}
		for _, s := range sites {
			if s.pageNavigation != tc.want {
				t.Errorf("%s: navigation=%t want %t", tc.src, s.pageNavigation, tc.want)
			}
		}
	}
}
