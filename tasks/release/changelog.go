package release

import (
	"fmt"

	gomake "github.com/anchore/go-make"
)

const (
	changelogFile = "CHANGELOG.md"
	versionFile   = "VERSION"
)

func ChangelogTask() gomake.Task {
	return gomake.Task{
		Name: "changelog",
		Desc: "generate a changelog",
		Run:  generateAndShowChangelog,
	}
}

func generateAndShowChangelog() {
	gomake.RunWithOptions(fmt.Sprintf(`chronicle -n --version-file %s`, versionFile), gomake.ExecStd(), gomake.ExecOutToFile(changelogFile))

	if gomake.IsBinnyManagedTool("glow") {
		gomake.Run(fmt.Sprintf(`glow -w 0 %s`, changelogFile))
		return
	}

	fmt.Println(gomake.ReadFile(changelogFile))
}
