package gotest

import (
	"os"
	"strings"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

// Downstream runs the provided set of commands on a downstream repository, with the current repository
// integrated via go work. The name is used for logging, task naming, and environment lookup
// in the case of pull requests needing to run on alternate repository forks or branches
func Downstream(name, repo, branch string, commands ...[]string) Task {
	repo = config.Env(strings.ToUpper(name)+"_REPO", repo)
	branch = config.Env(strings.ToUpper(name)+"_BRANCH", branch)
	// TODO detect running in pull requests, search comments for last: /repo <repo> <branch>
	return Task{
		Name:        name,
		Description: "test downstream: " + repo,
		RunsOn:      Deps("test"),
		Run: func() {
			repoRoot := git.Root()
			git.InClone(repo, branch, func() {
				Log("running %s with repo: %s branch: %s in dir: %v", name, repo, branch, file.Cwd())

				file.LogWorkdir()

				Run(`go work init`)
				Run(`go work use .`)
				Run(`go work use`, run.Args(repoRoot))

				log.Error(lang.Catch(func() {
					for _, command := range commands {
						Run(command[0], run.Args(command[1:]...), run.Stdout(os.Stderr))
					}
				}))

				log.Info("done")
			})
		},
	}
}
