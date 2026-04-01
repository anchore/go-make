package goreleaser

import (
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
)

func runSnapshot(extraArgs ...string) {
	file.Require(configName)

	file.WithTempDir(func(tempDir string) {
		dstConfig := filepath.Join(tempDir, configName)

		configContent := file.Read(configName)

		if !file.Contains(configName, "dist:") {
			configContent += "\ndist: snapshot\n"
		}

		file.Write(dstConfig, configContent)

		args := fmt.Sprintf(`goreleaser release --clean --snapshot --skip=publish --skip=sign --config=%s`, dstConfig)
		if len(extraArgs) > 0 {
			args += " " + strings.Join(extraArgs, " ")
		}
		Run(args)
	})
}

func SnapshotTasks() Task {
	return Task{
		Name:         "snapshot",
		Description:  "build a snapshot release with goreleaser",
		Dependencies: Deps("release:dependencies"),
		Run:          func() { runSnapshot() },
		Tasks: []Task{
			{
				Name:         "snapshot:single-target",
				Description:  "build a snapshot release with goreleaser for a single target",
				Dependencies: Deps("release:dependencies"),
				Run:          func() { runSnapshot("--single-target") },
			},
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
