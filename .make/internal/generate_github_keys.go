package internal

import (
	. "github.com/anchore/go-make"
)

// GenerateGitHubSSHKeysTask returns a task that refreshes the GitHub SSH known_hosts file
// by fetching the latest keys from GitHub's meta API. This runs the go:generate directive
// defined in git/tag.go.
func GenerateGitHubSSHKeysTask() Task {
	return Task{
		Name:        "generate:github-ssh-keys",
		Description: "refresh GitHub SSH known_hosts from the GitHub meta API",
		Run: func() {
			Run("go generate ./git/...")
		},
	}
}
