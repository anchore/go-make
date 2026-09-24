//go:build !windows

package run

import (
	"os/exec"
	"syscall"
)

func osExecOpts(c *exec.Cmd, cfg *runConfig) {
	if cfg.interactive {
		// an interactive command must stay in our process group, which is the terminal's
		// foreground group: a background process group is stopped by the OS when it reads
		// from the terminal or puts it in raw mode. Only the command itself can be
		// signalled as a result.
		c.Cancel = func() error {
			if c.Process == nil {
				return nil
			}
			return c.Process.Signal(syscall.SIGINT)
		}
		return
	}

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
