package release

import (
	"fmt"
	"path/filepath"

	. "github.com/anchore/go-make" //nolint:stylecheck
)

func SnapshotTask() Task {
	return Task{
		Name: "snapshot",
		Desc: "build a snapshot release with goreleaser",
		Deps: All("dependencies:quill", "dependencies:syft"),
		Run: func() {
			EnsureFileExists(configName)

			WithTempDir(func(tempDir string) {
				dstConfig := filepath.Join(tempDir, configName)

				configContent := ReadFile(configName)

				if !FileContains(configName, "dist:") {
					configContent += "\ndist: snapshot\n"
				}

				WriteFile(dstConfig, configContent)

				Run(fmt.Sprintf(`goreleaser release --clean --snapshot --skip=publish --skip=sign --config=%s`, dstConfig))
			})
		},
		Tasks: []Task{
			{
				Name:   "snapshot:clean",
				Desc:   "clean all snapshots",
				Labels: All("clean"),
				Run: func() {
					Rmdir("snapshot")
				},
			},
		},
	}
}
