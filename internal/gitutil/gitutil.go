// Package gitutil shells out to the system git binary rather than
// vendoring a git implementation — every repo this tool runs against
// already has git installed, since that's how the tester has source
// control in the first place. Kept deliberately tiny: just enough for
// "what changed since ref X."
package gitutil

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// CurrentRef returns the current HEAD commit hash for the repo at root.
func CurrentRef(root string) (string, error) {
	out, err := run(root, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("no commit to start a session from (make an initial commit first): %w", err)
	}
	return strings.TrimSpace(out), nil
}

// ChangedFilesSince returns every file path (relative to root) that
// differs between baseRef and the current working tree — both files
// git already tracks that were modified, and new files that were
// created but never committed. A hero-act or not-in-plan check is only
// meaningful against files actually touched this session, not the whole
// repo.
func ChangedFilesSince(root, baseRef string) (map[string]bool, error) {
	changed := map[string]bool{}

	tracked, err := run(root, "diff", "--name-only", baseRef)
	if err != nil {
		return nil, fmt.Errorf("diffing against %s: %w", baseRef, err)
	}
	for _, f := range splitLines(tracked) {
		changed[f] = true
	}

	untracked, err := run(root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("listing untracked files: %w", err)
	}
	for _, f := range splitLines(untracked) {
		changed[f] = true
	}

	return changed, nil
}

// CommitMessagesSince returns every commit's full message (subject +
// body) made since baseRef, oldest first, one string per commit. This is
// what lets the confidence check read what the agent actually claimed
// without anyone having to manually ask it and paste the answer — it's
// only reading what already exists, not asking for anything new.
func CommitMessagesSince(root, baseRef string) ([]string, error) {
	out, err := run(root, "log", "--reverse", "--format=%B%x00", baseRef+"..HEAD")
	if err != nil {
		return nil, fmt.Errorf("reading commit messages since %s: %w", baseRef, err)
	}
	var messages []string
	for _, m := range strings.Split(out, "\x00") {
		m = strings.TrimSpace(m)
		if m != "" {
			messages = append(messages, m)
		}
	}
	return messages, nil
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, filepathToSlash(line))
		}
	}
	return out
}

// git always prints forward slashes; normalize so callers can compare
// against filepath.Rel output on Windows without special-casing it.
func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

func run(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
