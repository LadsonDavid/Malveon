package wiring

import "testing"

// 5PM Club: "Set up folder structure: `/app`, `/components`, `/lib`" names directories, not
// server addresses; reading them as URLs made a done line NOT BUILT.
func TestFolderNamesInAPlanLineAreNotUrls(t *testing.T) {
	for _, line := range []string{
		"Set up folder structure: `/app`, `/components`, `/lib`, `/hooks`",
		"Create the directory layout: /src, /public",
	} {
		for _, a := range anchorsIn(line) {
			if a.kind == pathAnchor {
				t.Errorf("%q: %q must not be a URL anchor", line, a.text)
			}
		}
	}
	// a real address on a line that happens to say "structure" is still one when it has a second segment
	found := false
	for _, a := range anchorsIn("Document the API structure: `GET /api/orders`") {
		if a.kind == pathAnchor && a.text == "/api/orders" {
			found = true
		}
	}
	if !found {
		t.Error("a two-segment address is a URL even on a line that says structure")
	}
	// and a one-segment address on an ordinary line is still a URL
	found = false
	for _, a := range anchorsIn("Public pages: `/events`") {
		if a.kind == pathAnchor && a.text == "/events" {
			found = true
		}
	}
	if !found {
		t.Error("/events on an ordinary line must stay a URL")
	}
}
