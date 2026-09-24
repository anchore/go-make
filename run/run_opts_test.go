//go:build !windows

package run

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/anchore/go-make/require"
)

func Test_Interactive(t *testing.T) {
	cfg := runConfig{}
	cmd := exec.Command("echo")

	require.NoError(t, Interactive()(context.WithValue(context.Background(), runConfig{}, &cfg), cmd))
	require.True(t, cfg.interactive)
	require.Equal(t, os.Stdin, cmd.Stdin)

	// an interactive command must not get its own process group: a background process group
	// is stopped by the OS as soon as the command puts the terminal in raw mode.
	osExecOpts(cmd, &cfg)
	require.True(t, cmd.SysProcAttr == nil)

	other := exec.Command("echo")
	osExecOpts(other, &runConfig{})
	require.True(t, other.SysProcAttr != nil && other.SysProcAttr.Setpgid)
}
