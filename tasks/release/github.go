package release

import (
	"os"
	"regexp"
	"strings"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/run"
)

// GhCIReleaseTask creates a GitHub release for a specific version.
// The tag must already exist; the release will abort if it doesn't.
func GhCIReleaseTask() Task {
	return Task{
		Name:        "ci-release",
		Description: "creates a GitHub release (to be run in CI only)",
		Run: func() {
			if os.Getenv("CI") != "true" {
				panic("this task must be run in CI (CI=true)")
			}

			// get all up-to-date tags from the server
			Run("git fetch --tags --prune --prune-tags")

			changelogFile, versionFile := GenerateAndShowChangelog()

			version := strings.TrimSpace(file.Read(versionFile))
			if !regexp.MustCompile(`v\d+\.\d+\.\d+`).MatchString(version) {
				panic("version file does not appear to be a valid semver")
			}

			// use --verify-tag to abort if the tag doesn't already exist
			Run("gh release create --latest --verify-tag",
				run.Args(version, "--notes-file", changelogFile, "--title", version),
			)
		},
	}
}
