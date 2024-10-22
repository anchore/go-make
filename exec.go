package make

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
)

// Run a command, logging with current stdout / stderr
func Run(cmd ...string) {
	cmd = parseCmd(cmd...)

	NoErr(Exec(cmd[0], ExecArgs(cmd[1:]...), ExecStd()))
}

func RunWithOptions(cmd string, opts ...ExecOpt) {
	cmds := parseCmd(cmd)

	opts = append(opts, ExecArgs(cmds[1:]...))

	if len(opts) == 0 {
		opts = append(opts, ExecStd())
	}

	NoErr(Exec(cmds[0], opts...))
}

// RunE runs a command, returning stdout, stderr, err
func RunE(cmd ...string) (string, string, error) {
	cmd = parseCmd(cmd...)
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	err := Exec(cmd[0], ExecArgs(cmd[1:]...), func(cmd *exec.Cmd) {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	})
	return stdout.String(), stderr.String(), err
}

func parseCmd(cmd ...string) []string {
	cmd = append(ShellSplit(cmd[0]), cmd[1:]...)
	for i := range cmd {
		cmd[i] = Tpl(cmd[i])
	}
	return cmd
}

// Exec executes the given command, returning stdout and any error information
func Exec(cmd string, opts ...ExecOpt) error {
	// add the ToolDir first in the path for easier script writing
	lookupPath := os.Getenv("PATH")
	defer func() { LogErr(os.Setenv("PATH", lookupPath)) }()
	NoErr(os.Setenv("PATH", Tpl(ToolDir)+string(os.PathListSeparator)+lookupPath))

	// find exact command, call binny to make sure it's up-to-date
	cmd = binnyManagedToolPath(cmd)

	// create the command, this will look it up based on path:
	c := exec.CommandContext(ctx, cmd)
	c.Env = os.Environ()
	for k, v := range Globals {
		val := ""
		switch v := v.(type) {
		case func() string:
			val = v()
		case string:
			val = Tpl(v)
		default:
			continue
		}
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, val))
	}

	// finally, apply all the options to modify the command
	for _, opt := range opts {
		opt(c)
	}

	args := c.Args[1:] // exec.Command sets the cmd to Args[0]
	Log("%v %v", displayPath(cmd), strings.Join(args, " "))

	// print out c.Env -- GOROOT  vs GOBIN
	Debug("ENV: %v", c.Env)

	// execute
	err := c.Start()
	if err == nil {
		err = c.Wait()
	}
	if err != nil || (c.ProcessState != nil && c.ProcessState.ExitCode() > 0) {
		return &StackTraceError{
			Err:      fmt.Errorf("error executing: %s %s: %w", cmd, printArgs(args), err),
			ExitCode: c.ProcessState.ExitCode(),
		}
	}
	return nil
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

// ExecArgs appends args to the command
func ExecArgs(args ...string) ExecOpt {
	return func(cmd *exec.Cmd) {
		cmd.Args = append(cmd.Args, args...)
	}
}

// ExecStd executes with output mapped to the current process' stdout, stderr, stdin
func ExecStd() ExecOpt {
	return func(cmd *exec.Cmd) {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
	}
}

// ExecOut sends stdout to the writer, and optionally stderr
func ExecOut(stdout io.Writer, stderr ...io.Writer) ExecOpt {
	err := io.Writer(os.Stderr)
	if len(stderr) > 1 {
		err = stderr[1]
	}
	return func(cmd *exec.Cmd) {
		cmd.Stdout = stdout
		cmd.Stderr = err
		cmd.Stdin = os.Stdin
	}
}

func ExecOutToFile(path string) ExecOpt {
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	NoErr(err)
	return func(cmd *exec.Cmd) {
		cmd.Stdout = fh
	}
}

func ExecErrToFile(path string) ExecOpt {
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	NoErr(err)
	return func(cmd *exec.Cmd) {
		cmd.Stderr = fh
	}
}

// ExecEnv adds an environment variable to the command
func ExecEnv(key, val string) ExecOpt {
	return func(cmd *exec.Cmd) {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, Tpl(val)))
	}
}

// ExecOpts combines all opts into a single one
func ExecOpts(opts ...ExecOpt) ExecOpt {
	return func(cmd *exec.Cmd) {
		for _, opt := range opts {
			opt(cmd)
		}
	}
}

// ExecOpt is used to alter the command used in Exec calls
type ExecOpt func(*exec.Cmd)

var ctx, cancel = context.WithCancel(context.Background())

// Cancel invokes the cancel call on all active
func Cancel() {
	cancel()
}

func printArgs(args []string) string {
	for i, arg := range args {
		if strings.Contains(arg, " ") {
			if strings.Contains(arg, `'`) {
				args[i] = `"` + arg + `"`
			} else {
				args[i] = "'" + arg + "'"
			}
		}
	}
	return strings.Join(args, " ")
}
