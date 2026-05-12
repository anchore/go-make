package release

import (
	"fmt"
	"os"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

const (
	changelogFile = "CHANGELOG.md"
	versionFile   = "VERSION"
)

// ChangelogTask creates a task that generates a changelog using chronicle.
// Requires GitHub authentication via `gh auth login`.
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

// GenerateAndShowChangelog generates a changelog using chronicle, writes it to
// CHANGELOG.md, and displays it (using glow if available for better formatting).
// Returns paths to the generated changelog and version files.
func GenerateAndShowChangelog() (changelogFilePath, versionFilePath string) {
	// gh auth status will fail the user is not authenticated
	log.Debug(Run("gh auth status"))

	ghAuthToken := Run("gh auth token")
	log.Debug("Auth token: %.10s...", ghAuthToken)

	Run(
		fmt.Sprintf(`chronicle -n -o version=%s -o md=%s -o md-pretty`, versionFile, changelogFile),
		run.Stdout(os.Stderr),
		run.Env("GITHUB_TOKEN", ghAuthToken),
	)

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

	Run(
		fmt.Sprintf(`chronicle -n --until-tag %s -o md=%s -o md-pretty`, version, changelogFile),
		run.Stdout(os.Stderr),
		run.Env("GITHUB_TOKEN", ghAuthToken),
	)

	return changelogFile
}
