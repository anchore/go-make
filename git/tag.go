package git

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

// GitHub's official SSH host keys for MITM protection.
// These are fetched from the GitHub meta API and should match:
// https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints
//
//go:generate go run ./internal/cmd/gen-github-ssh-keys
//go:embed internal/cmd/gen-github-ssh-keys/known_hosts
var githubSSHKnownHosts string

// CreateTag creates an annotated git tag locally without pushing to remote.
// Temporarily modifies git user.name and user.email for the tag, then restores
// the original configuration. Returns the commit SHA that was tagged.
// Panics if the tag already exists locally or if validation fails.
func CreateTag(cfg CreateTagConfig) string {
	cfg.validate()

	// save original git user config for restoration
	origUserName := getGitConfig("user.name")
	origUserEmail := getGitConfig("user.email")

	defer func() {
		log.Debug("restoring original git user configuration")
		if origUserName != "" {
			setGitConfig("user.name", origUserName)
		} else {
			unsetGitConfig("user.name")
		}
		if origUserEmail != "" {
			setGitConfig("user.email", origUserEmail)
		} else {
			unsetGitConfig("user.email")
		}
	}()

	// configure git user
	setGitConfig("user.name", cfg.GitUserName)
	setGitConfig("user.email", cfg.GitUserEmail)

	// verify tag doesn't exist locally
	if tagExistsLocally(cfg.Tag) {
		panic(fmt.Errorf("tag %q already exists locally", cfg.Tag))
	}

	// create annotated tag
	lang.Return(run.Command("git", run.Args("tag", "-a", "-m", cfg.TagMessage, cfg.Tag)))

	// get commit SHA
	return strings.TrimSpace(lang.Return(run.Command("git", run.Args("rev-parse", "HEAD"), run.Quiet())))
}

// PushTag pushes an existing local tag to the remote using SSH with a deploy key.
// Sets up a temporary SSH agent with the deploy key, configures GitHub's known_hosts
// for MITM protection, and temporarily changes the remote URL to SSH format.
// All git configuration changes are restored after the push completes.
// Panics if the tag doesn't exist locally or already exists on the remote.
func PushTag(cfg PushTagConfig) {
	cfg.validate()

	// verify tag exists locally
	if !tagExistsLocally(cfg.Tag) {
		panic(fmt.Errorf("tag %q does not exist locally", cfg.Tag))
	}

	// verify tag doesn't already exist on remote
	if tagExistsRemotely(cfg.Tag) {
		panic(fmt.Errorf("tag %q already exists on remote", cfg.Tag))
	}

	// save original git config for restoration
	origSSHCommand := getGitConfig("core.sshCommand")
	origRemoteURL := getGitConfig("remote.origin.url")

	defer func() {
		log.Debug("restoring original git configuration")
		if origSSHCommand != "" {
			setGitConfig("core.sshCommand", origSSHCommand)
		} else {
			unsetGitConfig("core.sshCommand")
		}
		if origRemoteURL != "" {
			setGitConfig("remote.origin.url", origRemoteURL)
		}
	}()

	file.WithTempDir(func(tmpDir string) {
		// write known_hosts file with GitHub's SSH host keys
		knownHostsPath := filepath.Join(tmpDir, "known_hosts")
		file.Write(knownHostsPath, githubSSHKnownHosts)

		// start ssh-agent and load deploy key
		agentInfo, cleanup := setupSSHAgent(cfg.DeployKey)
		defer cleanup()

		// configure git to use SSH with strict host checking and explicit agent auth
		sshCommand := buildSSHCommand(knownHostsPath, agentInfo.authSock)
		setGitConfig("core.sshCommand", sshCommand)

		// set remote URL to SSH format
		sshURL := fmt.Sprintf("git@github.com:%s.git", cfg.Repository)
		setGitConfig("remote.origin.url", sshURL)

		// push tag to remote (with SSH agent environment)
		lang.Return(run.Command("git", run.Args("push", "origin", cfg.Tag), run.Env("SSH_AUTH_SOCK", agentInfo.authSock)))
	})
}

// buildSSHCommand constructs the SSH command string with security options.
// This is extracted for testability.
func buildSSHCommand(knownHostsPath, agentSocket string) string {
	// all paths are quoted to handle spaces and special characters safely
	return fmt.Sprintf(
		"ssh -o StrictHostKeyChecking=yes -o UserKnownHostsFile=%q -o IdentityAgent=%q -o BatchMode=yes",
		knownHostsPath,
		agentSocket,
	)
}

// tagExistsLocally checks if a tag exists in the local repository.
func tagExistsLocally(tag string) bool {
	output, _ := run.Command("git", run.Args("tag", "-l", "--", tag), run.NoFail(), run.Quiet())
	return strings.TrimSpace(output) == tag
}

// tagExistsRemotely checks if a tag exists on the remote.
func tagExistsRemotely(tag string) bool {
	output, _ := run.Command("git", run.Args("ls-remote", "--tags", "origin", "refs/tags/"+tag), run.NoFail(), run.Quiet())
	return strings.TrimSpace(output) != ""
}

// getGitConfig gets a git config value, returning empty string if not set.
func getGitConfig(key string) string {
	output, _ := run.Command("git", run.Args("config", "--get", key), run.NoFail(), run.Quiet())
	return output
}

// setGitConfig sets a git config value.
func setGitConfig(key, value string) {
	lang.Return(run.Command("git", run.Args("config", key, value)))
}

// unsetGitConfig unsets a git config value.
func unsetGitConfig(key string) {
	_, _ = run.Command("git", run.Args("config", "--unset", key), run.NoFail(), run.Quiet())
}
