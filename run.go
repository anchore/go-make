package gomake

import (
	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/shell"
	"github.com/anchore/go-make/template"
)

// Run executes a shell.Split command, automatically downloading binaries based on configurations such as binny,
// and executing with the run.Option(s) provided
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
