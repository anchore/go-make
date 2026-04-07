package release

import (
	"os"
	"regexp"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
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
			if os.Getenv("CI") != "true" {
				panic("this task must be run in CI (CI=true)")
			}

			deployKey := os.Getenv("DEPLOY_KEY")
			if deployKey == "" {
				panic("DEPLOY_KEY environment variable must be set")
			}

			tagName := os.Getenv("RELEASE_VERSION")
			if tagName == "" {
				panic("RELEASE_VERSION environment variable must be set")
			}

			// validate version format early before doing any work
			if !regexp.MustCompile(`^v\d+\.\d+\.\d+`).MatchString(tagName) {
				panic("RELEASE_VERSION does not appear to be a valid semver (e.g. v1.2.3)")
			}

			repository := os.Getenv("GITHUB_REPOSITORY")
			if repository == "" {
				panic("GITHUB_REPOSITORY environment variable must be set (e.g 'org/reponame')")
			}

			tagMessage := lang.Default(os.Getenv("TAG_MESSAGE"), "Release "+tagName)
			gitUserName := lang.Default(os.Getenv("GIT_USER_NAME"), "github-actions[bot]")
			gitUserEmail := lang.Default(os.Getenv("GIT_USER_EMAIL"), "github-actions[bot]@users.noreply.github.com")

			// create the tag locally first (no deploy key needed)
			sha := git.CreateTag(git.CreateTagConfig{
				Tag:          tagName,
				TagMessage:   tagMessage,
				GitUserName:  gitUserName,
				GitUserEmail: gitUserEmail,
			})

			log.Info("created local tag %s at %s", tagName, sha)

			// generate changelog for the version (needs the tag to exist locally)
			changelogFile := GenerateAndShowFromVersion(tagName)

			// push the tag to remote (needs deploy key)
			git.PushTag(git.PushTagConfig{
				Tag:        tagName,
				DeployKey:  deployKey,
				Repository: repository,
			})

			log.Info("pushed tag %s to remote", tagName)

			// use --verify-tag to abort if the tag doesn't already exist on remote
			Run("gh release create --latest --verify-tag",
				run.Args(tagName, "--notes-file", changelogFile, "--title", tagName),
			)
		},
	}
}
