// Package telemetry sends one anonymous event per `malveon check` run, so
// the founder can see real usage counts in a dashboard instead of only
// GitHub's download/traffic numbers (which can't tell whether a
// downloaded binary was ever actually run). Added 2026-09-19, a
// deliberate reversal of this tool's earlier no-telemetry stance — see
// CLAUDE.md section 2's note on this exact decision for the reasoning.
//
// What gets sent, and what doesn't:
//   - OS, architecture, and whether the run's gate was clean or blocked.
//   - Never: file paths, plan contents, code, feature names, commit
//     messages, or anything else read from the project being checked.
//   - The distinct ID is a random value generated once and stored in the
//     user's home directory (not the project being checked, so checking
//     ten different projects on one machine still counts as one user,
//     not ten) — not tied to any name, email, or machine identifier.
//   - IP-based geolocation is explicitly disabled in the event payload.
//
// Opt-out: --no-telemetry, or the MALVEON_NO_TELEMETRY or standard
// DO_NOT_TRACK (see https://consoledonottrack.com) environment
// variables. Checked before anything is generated or sent, so opting out
// also skips creating the local ID file.
//
// Best-effort only: a short client timeout, and any failure (no network,
// endpoint down) is swallowed silently — telemetry must never slow down
// or fail a real check run.
package telemetry

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	apiKey   = "phc_mpt8ojNqpDA5HLKg9aPePcLeULGtTqami8dVbzbfCWeG"
	endpoint = "https://us.i.posthog.com/capture/"
	timeout  = 2 * time.Second
)

// Disabled reports whether the user opted out via flag or environment
// variable — checked by the caller before Send so an opt-out never even
// generates the local anonymous ID.
func Disabled(flagSet bool) bool {
	if flagSet {
		return true
	}
	if os.Getenv("MALVEON_NO_TELEMETRY") != "" {
		return true
	}
	if os.Getenv("DO_NOT_TRACK") != "" {
		return true
	}
	return false
}

// SendCheckRun fires one "check_run" event: the fact that `malveon
// check` ran, on what OS, and whether its gate came back clean or
// blocked. Nothing about the project being checked is included.
func SendCheckRun(gateBlocked bool) {
	gate := "clean"
	if gateBlocked {
		gate = "blocked"
	}
	send("check_run", map[string]any{
		"os":                      runtime.GOOS,
		"arch":                    runtime.GOARCH,
		"gate":                    gate,
		"$geoip_disable":          true,
		"$process_person_profile": false,
	})
}

func send(event string, properties map[string]any) {
	id := distinctID()
	if id == "" {
		return
	}
	body, err := json.Marshal(map[string]any{
		"api_key":     apiKey,
		"event":       event,
		"distinct_id": id,
		"properties":  properties,
	})
	if err != nil {
		return
	}
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// distinctID reads the per-machine anonymous ID from the user's home
// directory, generating and persisting one on first use. Deliberately
// not tied to the project being checked — the same person checking ten
// different repos should count as one user, not ten.
func distinctID() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".malveon", "telemetry_id")
	if b, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(b)); id != "" {
			return id
		}
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	id := hex.EncodeToString(buf)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return id
	}
	_ = os.WriteFile(path, []byte(id), 0o600)
	return id
}
