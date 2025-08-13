package release

import (
	"fmt"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/file"
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
		Run:         generateAndShowChangelog,
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

func generateAndShowChangelog() {
	run.Command("chronicle -n --version-file", run.Args(versionFile), run.Write(changelogFile))

	if binny.IsManagedTool("glow") {
		run.Command(fmt.Sprintf(`glow -w 0 %s`, changelogFile))
		return
	}

	fmt.Println(file.Read(changelogFile))
}
