package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/stream"
)

// Option is used to alter the command used in Exec calls
type Option func(context.Context, *exec.Cmd) error

// Command runs a command, waits until completion, and returns stdout.
// The first argument is the path to the binary and DOES NOT shell-split.
// When not captured, stderr is output to os.Stderr and returned as part of the error text.
func Command(cmd string, opts ...Option) (string, error) {
	// by default, only capture output without duplicating it to logs
	opts = append([]Option{func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Stdout = io.Discard
		cmd.Stderr = os.Stderr
		cmd.Stdin = nil // do not attach stdin by default
		return nil
	}}, opts...)

	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	opts = append(opts, func(_ context.Context, cmd *exec.Cmd) error {
		// if we are not outputting Stdout, capture and return it
		if cmd.Stdout == io.Discard {
			cmd.Stdout = &stdout
		}
		// if the user isn't capturing stderr, we print to stderr by default and don't need to duplicate this in errors
		if cmd.Stderr != os.Stderr {
			cmd.Stderr = stream.Tee(cmd.Stderr, &stderr)
		}
		return nil
	})

	// create the command, this will look it up based on path:
	c := exec.CommandContext(Context(), cmd)

	env := os.Environ()
	var dropped []string
	for i := 0; i < len(env); i++ {
		nameValue := strings.SplitN(env[i], "=", 2)
		if skipEnvVar(nameValue[0]) {
			dropped = append(dropped, nameValue[0])
			continue
		}
		log.Trace(color.Grey("adding environment entry: %v", env[i]))
		c.Env = append(c.Env, env[i])
	}

	for _, e := range dropped {
		log.Trace(color.Grey("dropped environment entry: %v", e))
	}

	cfg := runConfig{}
	ctx := context.WithValue(Context(), runConfig{}, &cfg)

	// finally, apply all the options to modify the command
	for _, opt := range opts {
		err := opt(ctx, c)
		if err != nil {
			return "", err
		}
	}

	args := shortenedArgs(c.Args[1:]) // exec.Command sets the cmd to Args[0]

	logFunc := log.Info
	if cfg.quiet {
		logFunc = log.Debug
	}
	logFunc("$ %v %v", displayPath(cmd), strings.Join(args, " "))

	// print out c.Env -- GOROOT vs GOBIN
	log.Trace("ENV: %v", c.Env)

	// forward process end signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM) // , syscall.SIGQUIT)
	go func() {
		defer signal.Stop(signals)
		for sig := range signals {
			// SIGABRT is sent when the process ends normally, so we don't wait further
			if sig == syscall.SIGABRT {
				break
			}
			if c.Process != nil {
				log.Error(c.Process.Signal(sig))
			}
		}
	}()

	// WaitDelay specifies the time to wait after the process completes before continuing
	c.WaitDelay = 11 * time.Second
	osExecOpts(c)

	// execute
	err := c.Run()

	// this will cause the signal listener to proceed and stop waiting on other signals
	signals <- syscall.SIGABRT

	exitCode := 0
	if c.ProcessState != nil {
		exitCode = c.ProcessState.ExitCode()
	}
	if err != nil {
		fullStdOut := ""
		if stdout.Len() > 0 {
			fullStdOut = "\nSTDOUT:\n" + stdout.String()
		}
		if stderr.Len() > 0 {
			fullStdOut += "\nSTDERR:\n" + stderr.String()
		}
		err = lang.NewStackTraceError(fmt.Errorf("error executing: '%s %s': %w", cmd, printArgs(opts), err)).
			WithExitCode(exitCode).
			WithLog(fullStdOut)
	}
	if err != nil || exitCode > 0 {
		if cfg.noFail {
			log.Debug("error executing: '%v %v' exit code: %v: %v", displayPath(cmd), strings.Join(args, " "), exitCode, err)
			err = nil
		}
	}

	return strings.TrimSpace(stdout.String()), err
}

// Args appends args to the command
func Args(args ...string) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Args = append(cmd.Args, args...)
		return nil
	}
}

func InDir(dir string) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Dir = dir
		return nil
	}
}

// Options allows multiple options to be passed as one Option
func Options(options ...Option) Option {
	return func(ctx context.Context, cmd *exec.Cmd) error {
		for _, opt := range options {
			err := opt(ctx, cmd)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

// Write outputs stdout to a file
func Write(path string) Option {
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	defer lang.Close(fh, path)
	lang.Throw(err)
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Stdout = stream.Tee(cmd.Stdout, fh)
		return nil
	}
}

// Quiet logs at Debug level instead of Info level
func Quiet() Option {
	return func(ctx context.Context, cmd *exec.Cmd) error {
		if !config.Debug {
			if cmd.Stderr == os.Stderr {
				cmd.Stderr = io.Discard
			}
			cfg, _ := ctx.Value(runConfig{}).(*runConfig)
			if cfg != nil {
				cfg.quiet = true
			}
		}
		return nil
	}
}

// NoFail logs at Debug level instead of panicking
func NoFail() Option {
	return func(ctx context.Context, cmd *exec.Cmd) error {
		cfg, _ := ctx.Value(runConfig{}).(*runConfig)
		if cfg != nil {
			cfg.noFail = true
		}
		return nil
	}
}

// Stdout executes with stdout output mapped to the current process' stdout and optionally stderr
func Stdout(w io.Writer) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Stdout = w
		return nil
	}
}

// Stderr executes with stdout output mapped to the current process' stdout and optionally stderr
func Stderr(w io.Writer) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Stderr = w
		return nil
	}
}

// Stdin executes with stdout output mapped to the current process' stdout and optionally stderr
func Stdin(in io.Reader) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Stdin = in
		return nil
	}
}

// Env adds an environment variable to the command
func Env(key, val string) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
		return nil
	}
}

// LDFlags adds an `-ldflags` argument, appending to other existing LDFlags
func LDFlags(flags ...string) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		for i, arg := range cmd.Args {
			// append to existing ldflags arg
			if arg == "-ldflags" {
				if i+1 >= len(cmd.Args) {
					cmd.Args = append(cmd.Args, "")
				} else {
					cmd.Args[i+1] += " "
				}
				cmd.Args[i+1] += strings.Join(flags, " ")
				return nil
			}
		}
		cmd.Args = append(cmd.Args, "-ldflags", strings.Join(flags, " "))
		return nil
	}
}

func shortenedArgs(args []string) []string {
	const maxLen = 16
	var out []string
	for _, arg := range args {
		if len(out) > maxLen {
			arg = arg[:maxLen]
		}
		out = append(out, arg)
	}
	return out
}

func skipEnvVar(s string) bool {
	// it causes problems to keep go environment variables in embedded go executions,
	// removing them all seems to fix this up; a user needing a specific GO installation
	// can specify environment variables to run commands using the Env
	if strings.HasPrefix(s, "GO") || strings.HasPrefix(s, "CGO_") {
		return true
	}
	return false
}

func displayPath(cmd string) string {
	if config.Debug {
		return auxParent(cmd)
	}

	wd, err := os.Getwd()
	if err != nil {
		return auxParent(cmd)
	}

	absWd, err := filepath.Abs(wd)
	if err != nil {
		return auxParent(cmd)
	}

	if strings.HasPrefix(cmd, absWd) {
		relPath, err := filepath.Rel(absWd, cmd)
		if err != nil {
			return auxParent(cmd)
		}

		return auxParent(relPath)
	}

	// this is probably an absolute path to a system binary, just show the base command
	return filepath.Base(cmd)
}

func auxParent(path string) string {
	dir, file := filepath.Split(path)
	return color.Grey(dir) + file
}

func printArgs(args []Option) string {
	c := exec.Cmd{}
	for _, arg := range args {
		_ = arg(context.TODO(), &c)
	}
	for i, arg := range c.Args {
		if strings.Contains(arg, " ") {
			if strings.Contains(arg, `'`) {
				c.Args[i] = `"` + arg + `"`
			} else {
				c.Args[i] = "'" + arg + "'"
			}
		}
	}
	return strings.Join(c.Args, " ")
}

type runConfig struct {
	quiet  bool
	noFail bool
}
