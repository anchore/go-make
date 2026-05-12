package ci

import (
	"os"

	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

func PublishTag(tagName, deployKey string) {
	EnsureInCI()

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

	// push the tag to remote (needs deploy key)
	git.PushTag(git.PushTagConfig{
		Tag:        tagName,
		DeployKey:  deployKey,
		Repository: repository,
	})

	log.Info("pushed tag %s to remote", tagName)
}
