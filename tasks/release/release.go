package release

import (
	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/internal/ci"
	"github.com/anchore/go-make/run"
)

// Tasks creates the complete release task group including changelog generation,
// CI releases (publish tag + gh release... NOT goreleaser), and workflow triggers.
func Tasks() Task {
	return Task{
		Tasks: []Task{
			ChangelogTask(),       // `make chanagelog` to generate and show changes since the last release
			WorkflowReleaseTask(), // `make release` to trigger the release.yaml workflow
			TagReleaseTask(),      // `make ci-release` for publishing a tag and creating a GH release
		},
	}
}

// TagReleaseTask creates a git tag and GitHub release for a specific version.
// This task is designed to run in CI with GitHub's release environment, which provides
// deploy key SSH secrets for tag control. This approach is necessary because repository
// rulesets typically prevent direct tag pushes, so we use a deploy key with write access
// to push the tag via SSH.
//
// The workflow is:
//  1. Validate the version format (must be semver like v1.2.3)
//  2. Create the tag locally (no credentials needed)
//  3. Generate the changelog using chronicle with --until-tag
//  4. Push the tag to remote using the deploy key
//  5. Create the GitHub release with the changelog
func TagReleaseTask() Task {
	return Task{
		Name:        "ci-release",
		Description: "creates a GitHub release (to be run in CI only)",
		Run: func() {
			ensureNoGoreleaserConfig()

			tagName, deployKey := ci.ReleaseInputs()

			// this ensures we are in CI and will tag HEAD and push using the deploy key
			ci.PublishTag(tagName, deployKey)

			// generate changelog for the version (needs the tag to exist locally)
			changelogFile := GenerateAndShowFromVersion(tagName)

			// use --verify-tag to abort if the tag doesn't already exist on remote
			Run("gh release create --latest --verify-tag",
				run.Args(tagName, "--notes-file", changelogFile, "--title", tagName),
			)
		},
	}
}

// TagAndCreateGHRelease is a convenience alias for TagAndCreateGHReleaseTask to simplify the Makefile.
//
// Deprecated: please use TagReleaseTask
func TagAndCreateGHRelease() Task {
	return TagReleaseTask()
}

func ensureNoGoreleaserConfig() {
	// this release flow is for library-only projects; the presence of a goreleaser
	// config hints at one or more distributables, which means this is probably a tool
	// and should be using the goreleaser-backed release task instead.
	if file.Exists(".goreleaser.yaml") {
		panic(".goreleaser.yaml found: this release task is for library-only projects; use the goreleaser release task instead")
	}
}
