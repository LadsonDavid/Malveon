package wiring

import (
	"github.com/LadsonDavid/Malveon/pkg/features"
	"testing"
)

func TestUnchangedPromptFileIsNotEvidenceOfMissingBehavior(t *testing.T) {
	g := changedGraph(t)
	r := RunWithChanges(g, []features.Feature{{Name: "`" + menuFile + "` displays the text post", InSession: true}}, map[string]bool{})[0]
	if r.Verdict != Skipped || !r.Unchecked {
		t.Fatalf("unchanged source cannot prove missing behavior: %+v", r)
	}
	for _, claim := range []string{"`missing.tsx` displays a label", "`NewMissingFunction` is implemented"} {
		r := RunWithChanges(g, []features.Feature{{Name: claim, InSession: true}}, map[string]bool{})[0]
		if r.Verdict != NotBuilt {
			t.Fatalf("missing source anchor must remain NOT BUILT: %s: %+v", claim, r)
		}
	}
}
