//go:build windows

package commands

import (
	"os/exec"
	"strconv"
)

// setNewProcessGroup is a no-op on Windows — killProcessTree below uses
// taskkill's /T (tree) flag to reach child processes instead of a Unix-
// style process group, which Windows doesn't have the same way.
func setNewProcessGroup(cmd *exec.Cmd) {}

// killProcessTree kills a timed-out command and everything it spawned
// (npm -> node, etc.) via taskkill /T, since os.Process.Kill alone only
// reaches the direct child and would leave a real worker process running
// in the background past the configured timeout.
func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}
