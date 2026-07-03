//go:build linux || darwin || freebsd || netbsd || openbsd

package rules

import (
	"os/exec"
	"syscall"
)

// setProcGroup puts the child in its own process group so a timeout can
// kill the whole tree (child + any grandchildren it forked) instead of
// just the direct child. Without this, a command like "sh -c 'x & wait'"
// leaves x running as an orphan after the parent is killed.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcGroup sends SIGKILL to the negative pid, i.e. the whole process
// group rooted at cmd's direct child. This is wired up as cmd.Cancel, so
// it runs when the context is done, before Wait's WaitDelay grace period.
func killProcGroup(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
