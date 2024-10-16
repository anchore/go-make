package gobuild

import (
	"fmt"
	"path/filepath"

	. "github.com/anchore/go-make" //nolint:stylecheck
)

const configName = ".goreleaser.yaml"

func SnapshotTask() Task {
	return Task{
		Name: "snapshot",
		Desc: "build a snapshot release with goreleaser",
		Run: func() {
			EnsureFileExists(configName)

			WithTempDir(func(tempDir string) {
				dstConfig := filepath.Join(tempDir, configName)

				configContent := ReadFile(configName)

				if !FileContains(configName, "dist:") {
					configContent += "\ndist: snapshot\n"
				}

				WriteFile(configContent, dstConfig)

				Run(fmt.Sprintf(`goreleaser release --clean --snapshot --skip=publish --skip=sign --config=%s`, dstConfig))
			})
		},
	}
}
