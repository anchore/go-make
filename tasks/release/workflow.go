package release

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/gomod"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/script"
)

const (
	releaseWorkflowName = "release.yaml"
	workflowsPath       = ".github/workflows"
)

func WorkflowTask() Task {
	return Task{
		Name:        "release",
		Description: "trigger a release github actions workflow",
		Run: func() {
			file.Require(filepath.Join(workflowsPath, releaseWorkflowName))

			Run("gh auth status", run.Stdout(os.Stderr))

			m := gomod.Read()
			if m != nil {
				if strings.HasPrefix(m.Module.Mod.Path, "github.com/") {
					ghRepo := strings.TrimPrefix(m.Module.Mod.Path, "github.com/")
					ghRepo = regexp.MustCompile(`([^/]+/[^/]+)/.*`).ReplaceAllString(ghRepo, "$1")
					if strings.Count(ghRepo, "/") == 1 {
						Run("gh repo set default", run.Args(ghRepo))
					}
				}
			}

			// get the GitHub token
			githubToken := os.Getenv("GITHUB_TOKEN")
			if githubToken == "" {
				githubToken = Run("gh auth token")
				lang.Throw(os.Setenv("GITHUB_TOKEN", githubToken))
			}

			// ensure we have up-to-date git tags
			Run("git fetch --tags")

			GenerateAndShowChangelog()

			// read next version from VERSION file
			version := file.Read(versionFile)
			nextVersion := strings.TrimSpace(version)

			if nextVersion == "" || nextVersion == "(Unreleased)" {
				log.Log("Could not determine the next version to release. Exiting...")
				os.Exit(1)
			}

			// confirm if we should release
			script.Confirm("Do you want to trigger a release for version '%s'?", nextVersion)

			// trigger release
			log.Log("Kicking off release for %s", nextVersion)
			Run(fmt.Sprintf("gh workflow run %s -f version=%s", releaseWorkflowName, nextVersion))

			log.Log("Waiting for release to start...")
			time.Sleep(10 * time.Second)

			url := Run(fmt.Sprintf("gh run list --workflow=%s --limit=1 --json url --jq '.[].url'", releaseWorkflowName))
			log.Log(url)
		},
	}
}
