package run

import (
	"os"
	"os/exec"
)

func osExecOpts(c *exec.Cmd) {
	// on Windows, os.Process.Signal(os.Interrupt) is not supported for child processes.
	// Instead, kill the process directly when the context is cancelled. This is less
	// graceful than the Unix approach but is the only reliable option on Windows.
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		return c.Process.Signal(os.Kill)
	}
}
