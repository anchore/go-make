package release

import (
	"fmt"
	"os"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

const (
	changelogFile = "CHANGELOG.md"
	versionFile   = "VERSION"
)

func ChangelogTask() Task {
	return Task{
		Name:        "changelog",
		Description: "generate a changelog",
		Run:         func() { GenerateAndShowChangelog() },
		Tasks: []Task{
			{
				Name: "clean",
				Run: func() {
					file.Delete(changelogFile)
				},
			},
		},
	}
}

func GenerateAndShowChangelog() (changelogFilePath, versionFilePath string) {
	// gh auth status will fail the user is not authenticated
	log.Debug(Run("gh auth status"))

	ghAuthToken := Run("gh auth token")
	log.Debug("Auth token: %.10s...", ghAuthToken)

	changelog := Run("chronicle -n --version-file", run.Args(versionFile), run.Env("GITHUB_TOKEN", ghAuthToken))

	file.Write(changelogFile, changelog)

	// render the changelog with glow if available
	if binny.IsManagedTool("glow") {
		// without -s dark, it will defailt to no style since it cannot detect a tty with this approach
		changelog = Run(fmt.Sprintf(`glow -s dark -w 0 %s`, changelogFile))
	}
	lang.Return(os.Stderr.WriteString(changelog))

	return changelogFile, versionFile
}

// GenerateAndShowFromVersion generates a changelog for a specific version tag using chronicle's
// --until-tag flag. This is useful when the tag already exists locally and we want to generate
// the changelog up to and including that tag. Returns the changelog file path.
func GenerateAndShowFromVersion(version string) string {
	// gh auth status will fail the user is not authenticated
	log.Debug(Run("gh auth status"))

	ghAuthToken := Run("gh auth token")
	log.Debug("Auth token: %.10s...", ghAuthToken)

	changelog := Run("chronicle --until-tag", run.Args(version), run.Env("GITHUB_TOKEN", ghAuthToken))

	file.Write(changelogFile, changelog)

	// render the changelog with glow if available
	if binny.IsManagedTool("glow") {
		// without -s dark, it will default to no style since it cannot detect a tty with this approach
		changelog = Run(fmt.Sprintf(`glow -s dark -w 0 %s`, changelogFile))
	}
	lang.Return(os.Stderr.WriteString(changelog))

	return changelogFile
}
