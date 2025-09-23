package goreleaser

import (
	"errors"
	"os"
	"strings"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/tasks/release"
)

const configName = ".goreleaser.yaml"

func Tasks() Task {
	return Task{
		Tasks: []Task{
			SnapshotTasks(),
			CIReleaseTask(),
			release.WorkflowReleaseTask(),
		},
	}
}

func CIReleaseTask() Task {
	return Task{
		Name:         "ci-release",
		Description:  "build and publish a release with goreleaser",
		Dependencies: Deps("release:dependencies"),
		Run: func() {
			file.Require(configName)

			failIfNotInCI()
			ensureHeadHasTag()
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

func ensureHeadHasTag() {
	tags := strings.Split(Run("git tag --points-at HEAD"), "\n")

	for _, tag := range tags {
		if strings.HasPrefix(tag, "v") {
			log.Info("HEAD has a version tag: %s", tag)
			return
		}
	}

	panic(errors.New("HEAD does not have a tag that starts with 'v'"))
}

func failIfNotInCI() {
	if os.Getenv("CI") == "" {
		panic(errors.New("this task can only be run in CI"))
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
