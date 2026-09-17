// Package session manages the one piece of local state this tool keeps:
// the git ref captured when a task started, so later checks (not-in-plan,
// hero-act) can ask "what changed since then" without the tester ever
// looking up a commit hash by hand.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LadsonDavid/beta-test/internal/gitutil"
)

const stateDir = ".malveon"
const stateFile = "session.json"

type State struct {
	StartRef string `json:"start_ref"`
}

func statePath(root string) string {
	return filepath.Join(root, stateDir, stateFile)
}

// Start captures the current HEAD as the session baseline and writes it
// to .malveon/session.json under root.
func Start(root string) (State, error) {
	ref, err := gitutil.CurrentRef(root)
	if err != nil {
		return State{}, err
	}
	st := State{StartRef: ref}

	if err := os.MkdirAll(filepath.Join(root, stateDir), 0o755); err != nil {
		return State{}, fmt.Errorf("creating %s: %w", stateDir, err)
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return State{}, err
	}
	if err := os.WriteFile(statePath(root), raw, 0o644); err != nil {
		return State{}, fmt.Errorf("writing session state: %w", err)
	}
	return st, nil
}

// Load reads the previously captured session state. The second return
// value is false if no session was ever started — callers must treat
// that as "can't run this check" (NO PROOF), never as an empty diff.
func Load(root string) (State, bool) {
	raw, err := os.ReadFile(statePath(root))
	if err != nil {
		return State{}, false
	}
	var st State
	if json.Unmarshal(raw, &st) != nil || st.StartRef == "" {
		return State{}, false
	}
	return st, true
}
