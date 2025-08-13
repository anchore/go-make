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
	"github.com/anchore/go-make/script"
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
	// gh auth status should fail the user is not authenticated
	log.Debug(script.Run("gh auth status"))
	ghAuthToken := script.Run("gh auth token")
	log.Debug("Auth token: %.10s...", ghAuthToken)

	script.Run("chronicle -n --version-file", run.Args(versionFile), run.Write(changelogFile), run.Env("GITHUB_TOKEN", ghAuthToken))

	changelog := file.Read(changelogFile)
	if binny.IsManagedTool("glow") {
		changelog = script.Run(fmt.Sprintf(`glow -w 0 %s`, changelogFile))
	}
	lang.Return(os.Stderr.WriteString(changelog))

	return changelogFile, versionFile
}
