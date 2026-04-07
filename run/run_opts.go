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
	// when the context is cancelled, send SIGINT to the entire process group for
	// graceful shutdown instead of the default SIGKILL to just the child process.
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		return syscall.Kill(-c.Process.Pid, syscall.SIGINT)
	}
}
