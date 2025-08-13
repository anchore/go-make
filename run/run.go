package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

// Option is used to alter the command used in Exec calls
type Option func(context.Context, *exec.Cmd) error

// Command a command: wait until completion, return stdout and discard Stderr unless an error occurs, in which case the entire
// contents of stdout and stderr are returned as part of the error text. This function DOES NOT shell-split the command text
// provided.
func Command(cmd string, opts ...Option) string {
	// by default, only capture output without duplicating it to logs
	opts = append([]Option{func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Stdout = io.Discard
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return nil
	}}, opts...)

	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	opts = append(opts, func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Stdout = TeeWriter(cmd.Stdout, &stdout)
		// if the user isn't capturing stderr, we print to stderr by default and don't need to duplicate this in errors
		if cmd.Stderr != os.Stderr {
			cmd.Stderr = TeeWriter(cmd.Stderr, &stderr)
		}
		return nil
	})

	exitCode, err := runCommand(cmd, opts...)
	if err != nil {
		outStr := ""
		if stdout.Len() > 0 {
			outStr = "\n\n" + stdout.String()
		} else {
			outStr = "<no output>"
		}
		if stderr.Len() > 0 {
			outStr += "\nSTDERR:\n" + stderr.String()
		}
		panic(
			lang.NewStackTraceError(fmt.Errorf("error executing: '%s %s': %w", cmd, printArgs(opts), err)).
				WithExitCode(exitCode).
				WithLog(outStr))
	}

	return strings.TrimSpace(stdout.String())
}

// Args appends args to the command
func Args(args ...string) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Args = append(cmd.Args, args...)
		return nil
	}
}

// Write outputs stdout to a file
func Write(path string) Option {
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	lang.Throw(err)
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Stdout = TeeWriter(cmd.Stdout, fh)
		return nil
	}
}

// Quiet logs at Debug level instead of Log level
func Quiet() Option {
	return func(ctx context.Context, _ *exec.Cmd) error {
		cfg, _ := ctx.Value(runConfig{}).(*runConfig)
		cfg.quiet = true
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

// Cancel invokes a cancel call on all active commands
func Cancel() {
	config.Cancel()
}

// runCommand executes the given command, returning any error information
func runCommand(cmd string, opts ...Option) (int, error) {
	// create the command, this will look it up based on path:
	c := exec.CommandContext(config.Context, cmd)
	env := os.Environ()
	var dropped []string
	for i := 0; i < len(env); i++ {
		nameValue := strings.Split(env[i], "=")
		if skipEnvVar(nameValue[0]) {
			dropped = append(dropped, nameValue[0])
			continue
		}
		log.Trace(color.Grey("adding environment entry: %v", env[i]))
		c.Env = append(c.Env, env[i])
	}

	for _, e := range dropped {
		log.Debug(color.Grey("dropped environment entry: %v", e))
	}

	cfg := runConfig{}
	ctx := context.WithValue(config.Context, runConfig{}, &cfg)

	// finally, apply all the options to modify the command
	for _, opt := range opts {
		err := opt(ctx, c)
		if err != nil {
			return 0, err
		}
	}

	args := c.Args[1:] // exec.Command sets the cmd to Args[0]

	logFunc := log.Log
	if cfg.quiet {
		logFunc = log.Debug
	}
	logFunc("$ %v %v", displayPath(cmd), strings.Join(args, " "))

	// print out c.Env -- GOROOT vs GOBIN
	log.Debug("ENV: %v", c.Env)

	// execute
	err := c.Start()
	if err == nil {
		err = c.Wait()
	}
	if err != nil || (c.ProcessState != nil && c.ProcessState.ExitCode() > 0) {
		return c.ProcessState.ExitCode(), err
	}
	return 0, nil
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
	wd, err := os.Getwd()
	if err != nil {
		return auxParent(cmd)
	}

	absWd, err := filepath.Abs(wd)
	if err != nil {
		return auxParent(cmd)
	}

	relPath, err := filepath.Rel(absWd, cmd)
	if err != nil {
		return auxParent(cmd)
	}
	return auxParent(relPath)
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
	quiet bool
}
