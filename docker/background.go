package docker

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/anchore/go-make/file"
	. "github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/shell"
	"github.com/anchore/go-make/stream"
)

type Process struct {
	cmd         *exec.Cmd
	name        string
	containerID string
	started     sync.WaitGroup
	exited      sync.WaitGroup
	cmdOut      stream.TeeWriter
	cmdErr      stream.TeeWriter
}

// Background runs a container in the background, returning a *Process to execute commands and otherwise
// interact with the container
func Background(containerOrDockerfile string, opts ...Option) *Process {
	proc := &Process{
		name: randomString(),
	}
	proc.started.Add(1)
	proc.exited.Add(1)

	cfg := makeConfig(containerOrDockerfile, append(opts, func(cfg *commandConfig) error {
		waiter := sync.OnceFunc(func() {
			// execute postExecs in the command routine, so that we can wait for the container to start
			for _, postExec := range cfg.postExec {
				err := postExec(proc)
				if err != nil {
					proc.Kill()
				}
			}

			// signal we have the last setup completed
			proc.started.Done() // TODO is there a better way to determine the process has actually started?
		})
		cfg.dockerArgs = append(cfg.dockerArgs, run.Stdout(os.Stderr), func(ctx context.Context, cmd *exec.Cmd) error {
			cmd.Args = append(cmd.Args, "--name", proc.name)

			proc.cmd = cmd

			// inject a TeeWriter into stdout and stderr in order to wait for log text
			proc.cmdOut = stream.Tee()
			if cmd.Stdout == nil {
				proc.cmdOut.AddWriter(cmd.Stdout)
			}
			cmd.Stdout = proc.cmdOut

			proc.cmdErr = stream.Tee()
			if cmd.Stderr == nil {
				proc.cmdErr.AddWriter(cmd.Stderr)
			}
			cmd.Stderr = proc.cmdErr

			go waiter()

			return nil
		})
		return nil
	})...)

	go func() {
		err := Catch(func() {
			defer proc.exited.Done()
			runConfig(cfg)
		})
		if err != nil {
			log.Error(err)
			_ = Catch(func() { // may already be done, don't panic
				proc.started.Done()
			})
		}
	}()

	proc.started.Wait()

	for proc.containerID == "" {
		proc.containerID = Return(run.Command("docker", run.Args("ps", "--all", "--quiet", "--filter", "name="+proc.name)))
	}

	return proc
}

func makeConfig(containerOrDockerfile string, opts ...Option) *commandConfig {
	cfg := commandConfig{}
	for _, opt := range opts {
		Throw(opt(&cfg))
	}
	if file.IsRegular(containerOrDockerfile) {
		h := file.Sha256Hash(containerOrDockerfile)
		if cfg.name == "" {
			cfg.name = DefaultContainerPrefix + h
		}
		Build(containerOrDockerfile, cfg.name)
	} else {
		// otherwise we assume it's a container name
		cfg.name = containerOrDockerfile
	}
	return &cfg
}

func (r *Process) Kill() {
	if r == nil {
		log.Info("nil process, not sending signals")
		return
	}
	if r.cmd != nil && r.cmd.Process != nil {
		for _, signal := range []os.Signal{os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL} {
			// for _, signal := range []os.Signal{syscall.SIGINT, syscall.SIGSTOP, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGABRT} {
			if r.cmd != nil && r.cmd.Process != nil {
				if signal == syscall.SIGKILL {
					log.Error(r.cmd.Process.Kill())
				} else {
					log.Error(r.cmd.Process.Signal(signal))
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
		log.Info("sent kill signals to container: %s", r.name)
	}
	r.WaitUntilExit()
}

func (r *Process) WaitUntilExit() {
	r.exited.Wait()
}

// Exec runs a command in the running container
func (r *Process) Exec(containerCommand string, opts ...run.Option) string {
	cmd := shell.Split(containerCommand)
	return Return(run.Command("docker", run.Args("exec", r.containerID, cmd[0]), run.Args(cmd[1:]...), run.Options(opts...)))
}

// WaitLog waits for the provided text to be present in the stdout or stderr of the container, this is guaranteed
// to capture all the text from startup
func WaitLog(text string) Option {
	return func(cfg *commandConfig) error {
		cfg.postExec = append(cfg.postExec, func(proc *Process) error {
			proc.NextLogMatch(regexp.MustCompile(regexp.QuoteMeta(text)))
			return nil
		})
		return nil
	}
}

// WaitFor waits for the given condition to be true by polling
func WaitFor(condition func() bool) {
	for !condition() {
		time.Sleep(100 * time.Millisecond)
	}
}

// WaitLogText waits for the given text to appear in the container's stdout or stderr and returns
func (r *Process) WaitLogText(text string) {
	WaitFor(func() bool {
		return r.NextLogMatch(regexp.MustCompile(regexp.QuoteMeta(text))) != nil
	})
}

// NextLogMatch waits for the given regexp to appear in the container's stdout or stderr and returns the next match,
// organized with the full match at the "" and named subexpression matches. NOTE: this starts reading from the log
// when the function is called, so any text written to the log before the capture begins will not match, which may be
// problematic if an action is able to be scheduled before log capture begins
func (r *Process) NextLogMatch(re *regexp.Regexp, opts ...stream.Option) map[string]string {
	stdout, m1 := stream.NewRegexpScanner(re, opts...)
	r.cmdOut.AddWriter(stdout)
	defer r.cmdOut.RemoveWriter(stdout)

	stderr, m2 := stream.NewRegexpScanner(re, opts...)
	r.cmdErr.AddWriter(stderr)
	defer r.cmdErr.RemoveWriter(stderr)

	log.Info("waiting for: %s", re.String())

	// block until a match is found in either stdout or stderr
	var match map[string]string
	select {
	case match = <-m1:
	case match = <-m2:
	}
	return match
}

func randomString() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // read is always supposed to succeed
	return fmt.Sprintf("%x", b)
}
