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
			ReleaseTask(),                 // `make ci:release` (alias `ci-release`) to build and publish a release with goreleaser
			release.WorkflowReleaseTask(), // `make release` to trigger the release.yaml workflow
			release.ChangelogTask(),       // `make changelog` to generate and show changes since the last release
		},
	}
}

// ReleaseTask creates a task for running goreleaser in CI environments.
// Requires CI=true, a version tag on HEAD, and optional quill/syft tools
// for signing and SBOM generation. It has no Description so it stays hidden
// from `help`.
func ReleaseTask() Task {
	return Task{
		Name:         "ci:release",
		Aliases:      []string{"ci-release"},
		Dependencies: Deps("release:dependencies"),
		Run: func() {
			file.Require(configName)

			tagName := ci.ReleaseTagInput()

			ci.PublishTag(tagName)
			// fetch the pre-built changelog from the OCI cache (falls back to generating it
			// with chronicle if the cache is unavailable)
			changelogFile, _ := release.GetChangelog(tagName)

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
