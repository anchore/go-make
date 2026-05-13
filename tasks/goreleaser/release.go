package goreleaser

import (
	"fmt"

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
			SnapshotTasks(),               // `make snapshot` to build a local snapshot to ./snapshot
			ReleaseTask(),                 // `make ci-release` for building and publishing a release with goreleaser
			release.WorkflowReleaseTask(), // `make release` to trigger the release.yaml workflow
			release.ChangelogTask(),       // `make changelog` to generate and show changes since the last release
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
		Tasks: releaseDependencyTasks("quill", "syft", "cosign"),
	}
}

func releaseDependencyTasks(names ...string) []Task {
	tasks := make([]Task, len(names))
	taskNames := make([]string, len(names))
	for i, name := range names {
		taskNames[i] = fmt.Sprintf("dependencies:%s", name)
		tasks[i] = Task{
			Name: taskNames[i],
			Run: func() {
				if binny.IsManagedTool(name) {
					binny.Install(name)
				}
			},
		}
	}

	tasks = append(tasks, Task{
		Name:         "release:dependencies",
		Description:  "ensure all release dependencies are installed",
		Dependencies: Deps(taskNames...),
	})

	return tasks
}
