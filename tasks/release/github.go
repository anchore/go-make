package release

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
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

			var b = []byte{0}
		readLoop:
			for {
				fmt.Printf("Do you want to create a release for version '%s'? [y/n] ", version)
				lang.Return(os.Stdin.Read(b))
				fmt.Println()

				switch b[0] {
				case 'y', 'Y':
					break readLoop
				default:
					return // do not run release
				}
			}

			Run("gh release create --latest --fail-on-no-commits",
				run.Args(version, "--notes-file", changelogFile, "--title", version),
			)
		},
	}
}
