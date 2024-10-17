package release

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	. "github.com/anchore/go-make" //nolint:stylecheck
)

const configName = ".goreleaser.yaml"

func Tasks() Task {
	return Task{
		Tasks: []Task{
			ChangelogTask(),
			SnapshotTask(),
			CIReleaseTask(),
			WorkflowTask(),
		},
	}
}

func CIReleaseTask() Task {
	return Task{
		Name: "ci-release",
		Desc: "build and publish a release with goreleaser",
		Run: func() {
			EnsureFileExists(configName)

			failIfNotInCI()
			ensureHeadHasTag()
			generateAndShowChangelog()

			Run(fmt.Sprintf(`goreleaser release --clean --snapshot --releasenotes %s`, changelogFile))
		},
	}
}

func ensureHeadHasTag() {
	var tagBuf bytes.Buffer
	RunWithOptions("git tag --points-at HEAD", ExecOut(&tagBuf))

	tags := strings.Split(strings.TrimSpace(tagBuf.String()), "\n")

	for _, tag := range tags {
		if strings.HasPrefix(tag, "v") {
			Log("HEAD has a version tag: %s", tag)
			return
		}
	}

	Throw(errors.New("HEAD does not have a tag that starts with 'v'.")) //nolint:stylecheck
}

func failIfNotInCI() {
	if os.Getenv("CI") == "" {
		Throw(errors.New("this task can only be run in CI"))
	}
}
