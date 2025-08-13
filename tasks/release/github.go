package release

import (
	"regexp"
	"strings"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/script"
)

func GhReleaseTask() Task {
	return Task{
		Name:        "release",
		Description: "creates a gh release",
		Run: func() {
			changelogFile, versionFile := GenerateAndShowChangelog()
			version := strings.TrimSpace(file.Read(versionFile))
			log.Log("Creating release for version: %s", version)
			if !regexp.MustCompile(`v\d+\.\d+\.\d+`).MatchString(version) {
				panic("version file does not appear to be a valid semver")
			}

			script.Confirm("Do you want to create a release for version '%s'? ", version)

			Run("gh release create --latest --fail-on-no-commits",
				run.Args(version, "--notes-file", changelogFile, "--title", version),
			)
		},
	}
}
