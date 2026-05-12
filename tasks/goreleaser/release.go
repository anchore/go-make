package goreleaser

import (
	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/internal/ci"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/tasks/release"
)

const configName = ".goreleaser.yaml"

// Tasks creates the complete release task group including snapshot builds,
// CI releases, and workflow triggers.
func Tasks() Task {
	return Task{
		Tasks: []Task{
			SnapshotTasks(),
			ReleaseTask(),
			release.WorkflowReleaseTask(),
			release.ChangelogTask(),
		},
	}
}

// ReleaseTask creates a task for running goreleaser in CI environments.
// Requires CI=true, a version tag on HEAD, and optional quill/syft tools
// for signing and SBOM generation.
func ReleaseTask() Task {
	return Task{
		Name:         "ci-release",
		Description:  "build and publish a release with goreleaser",
		Dependencies: Deps("release:dependencies"),
		Run: func() {
			file.Require(configName)

			tagName, deployKey := ci.ReleaseInputs()

			ci.PublishTag(tagName, deployKey)
			changelogFile, _ := release.GenerateAndShowChangelog()

			Run(`goreleaser release --clean --release-notes`, run.Args(changelogFile))
		},
		Tasks: []Task{quillInstallTask(), syftInstallTask(), {
			Name:         "release:dependencies",
			Description:  "ensure all release dependencies are installed",
			Dependencies: Deps("dependencies:quill", "dependencies:syft"),
		}},
	}
}

func quillInstallTask() Task {
	return Task{
		Name: "dependencies:quill",
		Run: func() {
			if binny.IsManagedTool("quill") {
				binny.Install("quill")
			}
		},
	}
}

func syftInstallTask() Task {
	return Task{
		Name: "dependencies:syft",
		Run: func() {
			if binny.IsManagedTool("syft") {
				binny.Install("syft")
			}
		},
	}
}
