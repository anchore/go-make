package script

import (
	"os"
	"os/exec"

	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/shell"
	"github.com/anchore/go-make/template"
)

// Run executes a shell.Split command, automatically downloading binaries based on configurations such as binny,
// and executing with the run.Option(s) provided
func Run(cmd string, args ...run.Option) string {
	// add the ToolDir first in the path for easier script writing
	lookupPath := os.Getenv("PATH")
	defer func() { log.Error(os.Setenv("PATH", lookupPath)) }()
	lang.Throw(os.Setenv("PATH", template.Render(config.ToolDir)+string(os.PathListSeparator)+lookupPath))

	cmdParts := parseCmd(cmd)

	// append command arguments in order, following the executable
	if len(cmdParts) > 1 {
		args = append([]run.Option{run.Args(cmdParts[1:]...)}, args...)
	}

	// first, check if we have the tool in the path already such as gh on a GitHub Actions runner
	var err error
	cmd, err = exec.LookPath(cmdParts[0])
	if cmd == "" || err != nil {
		// find exact command, call binny to make sure it's up-to-date
		cmd = binny.ManagedToolPath(cmdParts[0])
		if cmd == "" {
			cmd = cmdParts[0]
		}
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
