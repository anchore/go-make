package release

import (
	"fmt"

	gomake "github.com/anchore/go-make"
)

func ChangelogTask() gomake.Task {
	return gomake.Task{
		Name: "changelog",
		Desc: "generate a changelog",
		Run:  generateAndShow,
	}
}

func generateAndShow() {
	gomake.RunWithOptions(`chronicle -n --version-file VERSION`, gomake.ExecStd(), gomake.ExecOutToFile("CHANGELOG.md"))

	if gomake.IsBinnyManagedTool("glow") {
		gomake.Run(`glow -w 0 CHANGELOG.md`)
		return
	}

	fmt.Println(gomake.ReadFile("CHANGELOG.md"))
}
