package ci

import (
	"os"

	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

// PublishTag creates a release tag locally and pushes it to the GitHub remote.
// The credentials used for the push are fetched from the environment via
// ReleasePushCredentials, which prefers TAG_TOKEN (HTTPS) and falls back to
// DEPLOY_KEY (SSH).
func PublishTag(tagName string) {
	EnsureInCI()

	deployKey, tagToken := ReleasePushCredentials()

	repository := os.Getenv("GITHUB_REPOSITORY")
	if repository == "" {
		panic("GITHUB_REPOSITORY environment variable must be set (e.g 'org/reponame')")
	}

	tagMessage := lang.Default(os.Getenv("TAG_MESSAGE"), "Release "+tagName)
	gitUserName := lang.Default(os.Getenv("GIT_USER_NAME"), "github-actions[bot]")
	gitUserEmail := lang.Default(os.Getenv("GIT_USER_EMAIL"), "github-actions[bot]@users.noreply.github.com")

	// create the tag locally first (no credentials needed)
	sha := git.CreateTag(git.CreateTagConfig{
		Tag:          tagName,
		TagMessage:   tagMessage,
		GitUserName:  gitUserName,
		GitUserEmail: gitUserEmail,
	})

	log.Info("created local tag %s at %s", tagName, sha)

	// push the tag to remote — PushTag dispatches to SSH or HTTPS based on which
	// credential is non-empty (ReleasePushCredentials guarantees exactly one is)
	git.PushTag(git.PushTagConfig{
		Tag:        tagName,
		DeployKey:  deployKey,
		TagToken:   tagToken,
		Repository: repository,
	})

	log.Info("pushed tag %s to remote", tagName)
}
