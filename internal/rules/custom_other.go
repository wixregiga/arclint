//go:build !(linux || darwin || freebsd || netbsd || openbsd)

package rules

import "os/exec"

// setProcGroup is a no-op on platforms without POSIX process groups
// (e.g. Windows). The project targets WSL/Linux; this keeps the build
// cross-platform without pretending to solve group-kill there.
func setProcGroup(cmd *exec.Cmd) {}

// killProcGroup falls back to killing just the direct child.
func killProcGroup(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
