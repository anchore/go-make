package run

import (
	"os"
	"os/exec"
)

func osExecOpts(c *exec.Cmd) {
	// when the context is cancelled, send an interrupt to the child process
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		return c.Process.Signal(os.Interrupt)
	}
}
