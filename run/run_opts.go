//go:build !windows

package run

import (
	"os/exec"
	"syscall"
)

func osExecOpts(c *exec.Cmd) {
	// set pgid so any kill operations apply to spawned children
	c.SysProcAttr = &syscall.SysProcAttr{
		Pgid:    0,
		Setpgid: true,
	}
}
