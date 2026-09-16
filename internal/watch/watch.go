// Package watch runs a background file-watcher during an AI agent's
// coding session, taking debounced snapshots of source files as they
// change — independent of git commits, independent of any agent
// self-report. This is what lets hero-act's known-pattern detection
// (see internal/checks/heropatterns) see intermediate states instead of
// only the session's final diff, which is otherwise blind to anything
// introduced and corrected before ever being committed.
//
// Built on fsnotify (github.com/fsnotify/fsnotify) — the standard,
// actively maintained, pure-Go cross-platform watcher (inotify/kqueue/
// ReadDirectoryChangesW), so this stays consistent with the rest of the
// tool: no cgo, one clean binary. See CLAUDE.md for the cadence
// reasoning (continual, not triggered or temporal) and the honest
// limitations (no recursive watching built in — handled manually below;
// no NFS/SMB support; OS watch/fd limits can make startup fail loudly).
package watch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/LadsonDavid/beta-test/internal/skipdirs"
)

const defaultHeartbeatInterval = 5 * time.Second
const defaultDebounce = 2 * time.Second

var watchedExts = map[string]bool{
	".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".py": true, ".go": true,
}

type Generation struct {
	Seq       int       `json:"seq"`
	Timestamp time.Time `json:"timestamp"`
	Files     []string  `json:"files"`
}

type Options struct {
	Root              string
	Debounce          time.Duration // 0 = defaultDebounce
	HeartbeatInterval time.Duration // 0 = defaultHeartbeatInterval
	OnSnapshot        func(Generation) // optional, for tests/observability
}

// Run blocks, watching Root recursively, until ctx is cancelled. It
// writes debounced snapshots to .malveon/history/ and a liveness
// heartbeat to .malveon/watch-status.json throughout, and marks itself
// cleanly stopped on the way out — the honesty record callers rely on
// before trusting anything captured here.
func Run(ctx context.Context, opts Options) error {
	if opts.Debounce == 0 {
		opts.Debounce = defaultDebounce
	}
	if opts.HeartbeatInterval == 0 {
		opts.HeartbeatInterval = defaultHeartbeatInterval
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("starting file watcher: %w", err)
	}
	defer w.Close()

	if err := addRecursive(w, opts.Root); err != nil {
		return fmt.Errorf("watching %s: %w", opts.Root, err)
	}

	now := time.Now()
	status := Status{
		Running: true, StartedAt: now, LastHeartbeat: now,
		HeartbeatInterval: opts.HeartbeatInterval.String(), PID: os.Getpid(),
	}
	if err := writeStatus(opts.Root, status); err != nil {
		return fmt.Errorf("writing watch status: %w", err)
	}
	defer func() {
		status.Running = false
		status.StoppedAt = time.Now()
		_ = writeStatus(opts.Root, status)
	}()

	heartbeat := time.NewTicker(opts.HeartbeatInterval)
	defer heartbeat.Stop()

	var debounceTimer *time.Timer
	pending := map[string]bool{}
	seq := loadNextSeq(opts.Root)

	flush := func() {
		if len(pending) == 0 {
			return
		}
		files := make([]string, 0, len(pending))
		for f := range pending {
			files = append(files, f)
		}
		gen := Generation{Seq: seq, Timestamp: time.Now(), Files: files}
		if err := writeGeneration(opts.Root, gen); err == nil {
			seq++
			if opts.OnSnapshot != nil {
				opts.OnSnapshot(gen)
			}
		}
		pending = map[string]bool{}
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return nil

		case ev, ok := <-w.Events:
			if !ok {
				flush()
				return nil
			}
			if ev.Op == fsnotify.Chmod {
				continue // pure attribute-change noise, not a real edit
			}
			info, statErr := os.Stat(ev.Name)
			if statErr == nil && info.IsDir() {
				if ev.Op&fsnotify.Create != 0 && !skipdirs.Names[filepath.Base(ev.Name)] {
					_ = w.Add(ev.Name) // a new subdirectory appeared mid-session; watch it too
				}
				continue
			}
			if !watchedExts[strings.ToLower(filepath.Ext(ev.Name))] {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			rel, relErr := filepath.Rel(opts.Root, ev.Name)
			if relErr != nil {
				rel = ev.Name
			}
			pending[filepath.ToSlash(rel)] = true

			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.NewTimer(opts.Debounce)

		case <-tickerC(debounceTimer):
			flush()
			debounceTimer = nil

		case _, ok := <-w.Errors:
			if !ok {
				flush()
				return nil
			}
			// A watch error doesn't necessarily mean the whole session is
			// lost — keep going, the heartbeat/staleness check downstream
			// is what ultimately decides whether history is trustworthy.

		case <-heartbeat.C:
			status.LastHeartbeat = time.Now()
			_ = writeStatus(opts.Root, status)
		}
	}
}

// tickerC lets a nil *time.Timer participate in the select above without
// a special case — a nil channel simply never fires.
func tickerC(t *time.Timer) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

func addRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() != filepath.Base(root) && skipdirs.Names[d.Name()] {
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}

func loadNextSeq(root string) int {
	gens, _ := LoadHistory(root)
	if len(gens) == 0 {
		return 0
	}
	return gens[len(gens)-1].Seq + 1
}

func historyIndexPath(root string) string {
	return filepath.Join(root, stateDir, historyDir, indexFile)
}

func writeGeneration(root string, gen Generation) error {
	genDir := filepath.Join(root, stateDir, historyDir, fmt.Sprintf("gen-%d", gen.Seq))
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		return err
	}
	for _, rel := range gen.Files {
		src := filepath.Join(root, filepath.FromSlash(rel))
		dst := filepath.Join(genDir, filepath.FromSlash(rel))
		if err := copyFile(src, dst); err != nil {
			continue // file may have been deleted/moved between event and flush; skip it honestly
		}
	}
	return appendToIndex(root, gen)
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func appendToIndex(root string, gen Generation) error {
	gens, _ := LoadHistory(root)
	gens = append(gens, gen)
	raw, err := json.MarshalIndent(gens, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(historyIndexPath(root)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(historyIndexPath(root), raw, 0o644)
}

// LoadHistory returns every captured generation, oldest first.
func LoadHistory(root string) ([]Generation, error) {
	raw, err := os.ReadFile(historyIndexPath(root))
	if err != nil {
		return nil, nil // no history yet is not an error
	}
	var gens []Generation
	if err := json.Unmarshal(raw, &gens); err != nil {
		return nil, fmt.Errorf("reading watch history: %w", err)
	}
	return gens, nil
}

// SnapshotContent returns the captured content of rel (a root-relative,
// forward-slash path) as of generation gen.Seq.
func SnapshotContent(root string, gen Generation, rel string) (string, bool) {
	path := filepath.Join(root, stateDir, historyDir, fmt.Sprintf("gen-%d", gen.Seq), filepath.FromSlash(rel))
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(raw), true
}
