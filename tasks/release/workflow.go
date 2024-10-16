package release

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/anchore/go-make" //nolint:stylecheck
)

const (
	releaseWorkflowName = "release.yaml"
	workflowsPath       = ".github/workflows"
)

func WorkflowTask() Task {
	return Task{
		Name: "release",
		Desc: "trigger a release github actions workflow",
		Run: func() {
			EnsureFileExists(filepath.Join(workflowsPath, releaseWorkflowName))

			Run("gh auth status")

			// TODO: gh repo set-default OWNER/PROJECT

			// get the GitHub token
			githubToken := os.Getenv("GITHUB_TOKEN")
			if githubToken == "" {
				var tokenBuf bytes.Buffer
				RunWithOptions("gh auth token", ExecOut(&tokenBuf))
				githubToken = strings.TrimSpace(tokenBuf.String())
				NoErr(os.Setenv("GITHUB_TOKEN", githubToken))
			}

			// ensure we have up-to-date git tags
			Run("git fetch --tags")

			generateAndShowChangelog()

			// read next version from VERSION file
			version := ReadFile(versionFile)
			nextVersion := strings.TrimSpace(version)

			if nextVersion == "" || nextVersion == "(Unreleased)" {
				Log("Could not determine the next version to release. Exiting...")
				os.Exit(1)
			}

			// confirm if we should release
		loop:
			for {
				Log("Do you want to trigger a release for version '%s'? [y/n]", nextVersion)
				var yn string
				_, err := fmt.Scan(&yn)
				NoErr(err)
				switch strings.ToLower(yn) {
				case "y":
					break loop
				case "n":
					Log("Cancelling release...")
					return
				default:
					Log("Please answer 'y' or 'n'")
				}
			}

			// trigger release
			Log("Kicking off release for %s", nextVersion)
			Run(fmt.Sprintf("gh workflow run %s -f version=%s", releaseWorkflowName, nextVersion))

			Log("Waiting for release to start...")
			time.Sleep(10 * time.Second)

			var urlBuf bytes.Buffer
			RunWithOptions(fmt.Sprintf("gh run list --workflow=%s --limit=1 --json url --jq '.[].url'", releaseWorkflowName), ExecOut(&urlBuf))
			Log(urlBuf.String())
		},
	}
}
