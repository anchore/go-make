package release

import (
	"errors"
	"fmt"
	"os"
	"strings"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/binny"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

const configName = ".goreleaser.yaml"

func Tasks() Task {
	return Task{
		Tasks: []Task{
			ChangelogTask(),
			SnapshotTasks(),
			CIReleaseTask(),
			WorkflowTask(),
		},
	}
}

func CIReleaseTask() Task {
	return Task{
		Name:        "ci-release",
		Description: "build and publish a release with goreleaser",
		Run: func() {
			file.Require(configName)

			failIfNotInCI()
			ensureHeadHasTag()
			generateAndShowChangelog()

			Run(fmt.Sprintf(`goreleaser release --clean --snapshot --releasenotes %s`, changelogFile))
		},
		Tasks: []Task{quillInstallTask(), syftInstallTask()},
	}
}

func ensureHeadHasTag() {
	tags := strings.Split(Run("git tag --points-at HEAD"), "\n")

	for _, tag := range tags {
		if strings.HasPrefix(tag, "v") {
			log.Log("HEAD has a version tag: %s", tag)
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
		Name:   "dependencies:quill",
		RunsOn: lang.List("ci-release"),
		Run: func() {
			if binny.IsManagedTool("quill") {
				binny.Install("quill")
			}
		},
	}
}

func syftInstallTask() Task {
	return Task{
		Name:   "dependencies:syft",
		RunsOn: lang.List("ci-release"),
		Run: func() {
			if binny.IsManagedTool("syft") {
				binny.Install("syft")
			}
		},
	}
}
