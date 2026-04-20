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
	"time"

	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/stream"
)

// Option is a functional option that modifies command execution behavior.
// Options are applied in order before the command runs, allowing customization
// of arguments, environment, I/O streams, and error handling.
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
	for i := range env {
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

	// WaitDelay specifies the time to wait after context cancellation (and the Cancel func
	// being called) before force-killing the process.
	c.WaitDelay = 11 * time.Second
	osExecOpts(c)

	// execute
	err := c.Run()

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

// InDir executes the command in the specified directory. This is equivalent to
// running "cd dir && cmd" but without spawning a shell. The directory change
// only affects this command execution.
func InDir(dir string) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Dir = dir
		return nil
	}
}

// Options combines multiple Options into a single Option. This is useful for
// creating reusable option sets or conditionally including groups of options.
//
// Example:
//
//	buildOpts := run.Options(run.Env("CGO_ENABLED", "0"), run.Quiet())
//	Run(`go build ./...`, buildOpts)
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

// Write redirects stdout to a file, creating the file if it doesn't exist and truncating
// it if it does. The file is opened before returning the Option, so errors during file
// creation will panic immediately.
func Write(path string) Option {
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	defer lang.Close(fh, path)
	lang.Throw(err)
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Stdout = stream.Tee(cmd.Stdout, fh)
		return nil
	}
}

// Quiet suppresses command output at Info level. The command line is logged at Debug
// level instead, and stderr is discarded unless config.Debug is enabled. Useful for
// commands whose output is only needed programmatically.
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

// NoFail prevents the command from panicking on failure. Instead of panicking,
// the error is logged at Debug level and an empty string is returned. Use this
// when command failure is expected or acceptable.
//
// Example:
//
//	version := Run(`git describe --tags`, run.NoFail())
//	if version == "" {
//	    version = "dev"
//	}
func NoFail() Option {
	return func(ctx context.Context, cmd *exec.Cmd) error {
		cfg, _ := ctx.Value(runConfig{}).(*runConfig)
		if cfg != nil {
			cfg.noFail = true
		}
		return nil
	}
}

// Stdout redirects the command's stdout to the provided writer. By default, stdout
// is captured and returned as the result string. Use this to stream output to a file,
// buffer, or os.Stdout for real-time display.
func Stdout(w io.Writer) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Stdout = w
		return nil
	}
}

// Stderr redirects the command's stderr to the provided writer. By default, stderr
// is sent to os.Stderr. Use io.Discard to suppress error output.
func Stderr(w io.Writer) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Stderr = w
		return nil
	}
}

// Stdin provides input to the command from the given reader. By default, stdin is
// not connected (nil). Use this for commands that read from standard input.
func Stdin(in io.Reader) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Stdin = in
		return nil
	}
}

// Env adds an environment variable to the command's environment. Note that the command
// inherits the current process environment by default (minus GO* and CGO_* variables
// which are filtered to avoid conflicts).
func Env(key, val string) Option {
	return func(_ context.Context, cmd *exec.Cmd) error {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
		return nil
	}
}

// LDFlags adds Go linker flags via the -ldflags argument. If -ldflags is already present
// in the command arguments, the new flags are appended to the existing value rather than
// replacing it. This allows multiple LDFlags() calls to accumulate.
//
// Example:
//
//	Run(`go build ./cmd/app`,
//	    run.LDFlags("-s", "-w"),
//	    run.LDFlags("-X main.version=1.0.0"),
//	)
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
		if len(out) > maxLen && len(arg) > maxLen {
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
