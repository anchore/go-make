package release

import (
	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/internal/ci"
	"github.com/anchore/go-make/run"
)

// TagAndCreateGHRelease creates a git tag and GitHub release for a specific version.
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
func TagAndCreateGHRelease() Task {
	return Task{
		Name:        "ci-release",
		Description: "creates a GitHub release (to be run in CI only)",
		Run: func() {
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
