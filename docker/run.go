package docker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	. "github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

var DefaultContainerPrefix = "localhost/go-make-auto-build:"
var DefaultContainerDir = "/.data"

type commandConfig struct {
	name        string
	dockerArgs  []run.Option
	commandArgs []run.Option
	postExec    []func(*Process) error
}

func Run(containerOrDockerfile string, opts ...Option) string {
	return runConfig(makeConfig(containerOrDockerfile, opts...))
}

func runConfig(cfg *commandConfig) string {
	var cmd *exec.Cmd
	defer func() {
		if cmd != nil && cmd.Process != nil {
			log.Error(cmd.Process.Kill())
			log.Error(cmd.Process.Signal(syscall.SIGTERM))
		}
	}()
	// , "--env", "TERM=xterm-256color"
	return Return(run.Command("docker", run.Stderr(os.Stderr), func(_ context.Context, c *exec.Cmd) error {
		cmd = c
		// cmd.SysProcAttr = &syscall.SysProcAttr{
		//	Setpgid: true,
		//	//Setsid:  true,
		//	//Pdeathsig: syscall.SIGKILL,
		//}
		return nil
		// }, run.Args("run", "--rm", "--init", "--interactive"), run.Options(cfg.dockerArgs...), run.Args(cfg.name), run.Options(cfg.commandArgs...)))
	}, run.Args("run", "--rm", "--interactive"), run.Options(cfg.dockerArgs...), run.Args(cfg.name), run.Options(cfg.commandArgs...)))
}

func Build(dockerfile, tag string) {
	f := Return(filepath.Abs(dockerfile))
	Return(run.Command("docker", run.Args("build", "--tag", tag, "--file", filepath.Base(dockerfile), "."), run.Stdout(os.Stderr), run.Stderr(os.Stderr), run.InDir(filepath.Dir(f))))
}
