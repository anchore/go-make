package release

import (
	"fmt"
	"path/filepath"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
)

func SnapshotTasks() Task {
	return Task{
		Name:        "snapshot",
		Description: "build a snapshot release with goreleaser",
		Run: func() {
			file.Require(configName)

			file.WithTempDir(func(tempDir string) {
				dstConfig := filepath.Join(tempDir, configName)

				configContent := file.Read(configName)

				if !file.Contains(configName, "dist:") {
					configContent += "\ndist: snapshot\n"
				}

				file.Write(dstConfig, configContent)

				Run(fmt.Sprintf(`goreleaser release --clean --snapshot --skip=publish --skip=sign --config=%s`, dstConfig))
			})
		},
		Tasks: []Task{
			{
				Name:        "snapshots:clean",
				Description: "clean all snapshots",
				RunsOn:      lang.List("clean"),
				Run: func() {
					file.Delete("snapshot")
				},
			},
		},
	}
}
