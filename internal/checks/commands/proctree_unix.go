//go:build !windows

package commands

import (
	"os/exec"
	"syscall"
)

// setNewProcessGroup puts cmd in its own process group so killProcessTree
// can reach every process it spawns (npm -> node, make -> its recipe
// shell, ...), not just the direct child.
func setNewProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessTree kills the whole process group a timed-out command
// started, not just the one process exec.Cmd knows about directly.
// Without this, a killed `npm test` can leave the real worker process
// (or a watch-mode process that never exits on its own) running in the
// background indefinitely — exactly the hang this hard timeout exists to
// prevent.
func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
