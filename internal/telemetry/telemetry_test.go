package telemetry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDisabledRespectsFlagAndEnv(t *testing.T) {
	t.Setenv("MALVEON_NO_TELEMETRY", "")
	t.Setenv("DO_NOT_TRACK", "")

	if Disabled(false) {
		t.Error("expected not disabled with no flag and no env vars set")
	}
	if !Disabled(true) {
		t.Error("expected disabled when the flag is set")
	}

	t.Setenv("MALVEON_NO_TELEMETRY", "1")
	if !Disabled(false) {
		t.Error("expected disabled via MALVEON_NO_TELEMETRY")
	}
	t.Setenv("MALVEON_NO_TELEMETRY", "")

	t.Setenv("DO_NOT_TRACK", "1")
	if !Disabled(false) {
		t.Error("expected disabled via DO_NOT_TRACK")
	}
}

// TestDistinctIDIsPersistedNotRegenerated proves the anonymous ID is
// stable across calls (so repeat runs count as one user, not a fresh
// one every time) and lands in the user's home directory, not whatever
// project happens to be checked.
func TestDistinctIDIsPersistedNotRegenerated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir on Windows reads this

	first := distinctID()
	if first == "" {
		t.Fatal("expected a non-empty distinct ID")
	}
	second := distinctID()
	if second != first {
		t.Errorf("expected the same ID on a second call, got %q then %q", first, second)
	}

	if _, err := os.Stat(filepath.Join(home, ".malveon", "telemetry_id")); err != nil {
		t.Errorf("expected the ID persisted under the home directory: %v", err)
	}
}
