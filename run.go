package gomake

import (
	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/shell"
	"github.com/anchore/go-make/template"
)

// Run executes a shell command with automatic template rendering and binny tool management.
//
// The command string is first parsed using shell.Split to handle quoted arguments properly,
// then all parts are rendered through the template engine to expand variables like {{RootDir}}.
// If the command references a binny-managed tool (configured in .binny.yaml), that tool
// will be automatically installed if not already present.
//
// By default, Run panics on command failure. Use run.NoFail() to return an empty string
// instead of panicking. Returns stdout as a trimmed string.
//
// Example:
//
//	Run(`go build -o {{ToolDir}}/myapp ./cmd/myapp`)
//	Run(`golangci-lint run`, run.Quiet())
//	version := Run(`git describe --tags`, run.NoFail())
func Run(cmd string, args ...run.Option) string {
	cmdParts := parseCmd(cmd)

	// append command arguments in order, following the executable
	if len(cmdParts) > 1 {
		args = append([]run.Option{run.Args(cmdParts[1:]...)}, args...)
	}

	// find absolute path to command, call binny to make sure it's up-to-date
	cmd = binny.ManagedToolPath(cmdParts[0])
	if cmd == "" {
		cmd = cmdParts[0]
	}
	return lang.Return(run.Command(cmd, args...))
}

func parseCmd(cmd ...string) []string {
	cmd = append(shell.Split(cmd[0]), cmd[1:]...)
	for i := range cmd {
		cmd[i] = template.Render(cmd[i])
	}
	return cmd
}
