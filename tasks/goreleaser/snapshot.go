package goreleaser

import (
	"fmt"
	"path/filepath"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
)

func runSnapshot(singleTarget bool) {
	file.Require(configName)

	file.WithTempDir(func(tempDir string) {
		dstConfig := filepath.Join(tempDir, configName)

		configContent := file.Read(configName)

		if !file.Contains(configName, "dist:") {
			configContent += "\ndist: snapshot\n"
		}

		file.Write(dstConfig, configContent)

		Run(snapshotArgs(dstConfig, singleTarget))
	})
}

// snapshotArgs builds the goreleaser command for a snapshot build. single-target
// builds use `goreleaser build` (current OS/arch only) because --single-target is
// a build-only flag — `goreleaser release` rejects it with "unknown flag". Full
// snapshots use `goreleaser release` to exercise the whole release pipeline
// (archives, packages, etc.) while skipping publish and sign.
func snapshotArgs(config string, singleTarget bool) string {
	if singleTarget {
		return fmt.Sprintf("goreleaser build --clean --snapshot --single-target --config=%s", config)
	}
	return fmt.Sprintf("goreleaser release --clean --snapshot --skip=publish --skip=sign --config=%s", config)
}

// SnapshotTasks creates tasks for building snapshot (non-release) builds with goreleaser.
// Snapshot builds skip publishing and signing, useful for testing the release process locally.
//
// Created tasks:
//   - snapshot: builds for all configured targets
//   - snapshot:single-target: builds for current OS/arch only (faster)
//   - snapshots:clean: removes the snapshot output directory
func SnapshotTasks() Task {
	return Task{
		Name:         "snapshot",
		Description:  "build a snapshot release with goreleaser",
		Dependencies: Deps("release:dependencies"),
		Run:          func() { runSnapshot(false) },
		Tasks: []Task{
			{
				Name:         "snapshot:single-target",
				Description:  "build a snapshot release with goreleaser for a single target",
				Dependencies: Deps("release:dependencies"),
				Run:          func() { runSnapshot(true) },
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
