package watch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	stateDir      = ".malveon"
	statusFile    = "watch-status.json"
	historyDir    = "history"
	indexFile     = "index.json"
	staleAfterMul = 3 // heartbeat considered dead after this many missed intervals
)

// Status is the watcher's own honesty record. Anything reading captured
// history must check this first — a background process that silently
// dies is exactly the kind of unverifiable claim this tool exists to
// refuse to make.
type Status struct {
	Running           bool      `json:"running"`
	StartedAt         time.Time `json:"started_at"`
	LastHeartbeat     time.Time `json:"last_heartbeat"`
	StoppedAt         time.Time `json:"stopped_at,omitempty"`
	HeartbeatInterval string    `json:"heartbeat_interval"` // stored as a duration string, e.g. "5s"
	PID               int       `json:"pid"`
}

func statusPath(root string) string {
	return filepath.Join(root, stateDir, statusFile)
}

func writeStatus(root string, st Status) error {
	if err := os.MkdirAll(filepath.Join(root, stateDir), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statusPath(root), raw, 0o644)
}

// LoadStatus reads the watcher's status file. ok is false if it doesn't
// exist at all — watch was never run.
func LoadStatus(root string) (Status, bool) {
	raw, err := os.ReadFile(statusPath(root))
	if err != nil {
		return Status{}, false
	}
	var st Status
	if json.Unmarshal(raw, &st) != nil {
		return Status{}, false
	}
	return st, true
}

// Usable reports whether the captured history can honestly be trusted:
// either the watcher stopped cleanly, or it's still running with a
// recent heartbeat. Anything else (stale heartbeat = crashed, or no
// status at all) means the history may be incomplete and must not be
// used as if it were the whole session.
func Usable(st Status) (bool, string) {
	interval, err := time.ParseDuration(st.HeartbeatInterval)
	if err != nil {
		interval = defaultHeartbeatInterval
	}
	if !st.Running {
		if st.StoppedAt.IsZero() {
			return false, "watcher status is invalid (never marked running or stopped)"
		}
		return true, ""
	}
	if time.Since(st.LastHeartbeat) > interval*staleAfterMul {
		return false, fmt.Sprintf("watcher heartbeat is stale (last seen %s ago) — it likely crashed, so captured history may be incomplete", time.Since(st.LastHeartbeat).Round(time.Second))
	}
	return true, ""
}
