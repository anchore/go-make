package git

import (
	_ "embed"
	"fmt"
	"os"
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

// PushTag pushes an existing local tag to the remote. The credential in cfg
// selects the transport: a DeployKey pushes via SSH, a TagToken pushes via
// HTTPS. Exactly one must be set (enforced by PushTagConfig.validate).
// Panics if the tag doesn't exist locally or already exists on the remote.
func PushTag(cfg PushTagConfig) {
	cfg.validate()

	if !tagExistsLocally(cfg.Tag) {
		panic(fmt.Errorf("tag %q does not exist locally", cfg.Tag))
	}

	if cfg.TagToken != "" {
		pushTagWithToken(cfg)
		return
	}
	pushTagWithDeployKey(cfg)
}

// sshRemoteURL and httpsRemoteURL build the GitHub remote URL for a repository
// in each transport's format. They are package variables (never reassigned by
// production code) so integration tests can redirect a push at a local bare
// repo instead of github.com — the remote-tag-existence check otherwise only
// ever talks to real GitHub and can't be exercised offline.
var (
	sshRemoteURL = func(repository string) string {
		return fmt.Sprintf("git@github.com:%s.git", repository)
	}
	httpsRemoteURL = func(repository string) string {
		return fmt.Sprintf("https://github.com/%s.git", repository)
	}
)

// pushTagWithDeployKey pushes the tag via SSH using a temporary ssh-agent
// loaded with the deploy key. GitHub's known_hosts is pinned to prevent MITM,
// the remote URL is temporarily switched to SSH, and all git config changes
// are restored on return.
func pushTagWithDeployKey(cfg PushTagConfig) {
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
		} else {
			unsetGitConfig("remote.origin.url")
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
		sshURL := sshRemoteURL(cfg.Repository)
		setGitConfig("remote.origin.url", sshURL)

		// every git invocation in this block must carry SSH_AUTH_SOCK so it can
		// talk to the temporary agent that holds the deploy key.
		authEnv := run.Env("SSH_AUTH_SOCK", agentInfo.authSock)

		// check remote tag existence against the NEW (SSH, authenticated) remote
		// rather than whatever URL/credential the runner started with.
		if tagExistsRemotely(cfg.Tag, authEnv) {
			panic(fmt.Errorf("tag %q already exists on remote", cfg.Tag))
		}

		// push tag to remote (with SSH agent environment)
		lang.Return(run.Command("git", run.Args("push", "origin", cfg.Tag), authEnv))
	})
}

// pushTagWithToken pushes the tag via HTTPS using a GitHub token. The token
// is delivered to git via a GIT_ASKPASS helper script and an environment
// variable scoped to the git child process. The token is never written to
// disk, never embedded in the remote URL, and never appears on the command
// line or in git config.
func pushTagWithToken(cfg PushTagConfig) {
	origRemoteURL := getGitConfig("remote.origin.url")

	// CI checkout steps (notably actions/checkout) persist the job's default
	// credentials into the repo's local git config as one or more
	// http.<server>.extraheader entries, e.g.
	//
	//   http.https://github.com/.extraheader = AUTHORIZATION: basic <base64 GITHUB_TOKEN>
	//
	// git attaches that Authorization header to EVERY HTTPS request. Because the
	// request then arrives already authenticated, the server never replies 401,
	// so git never invokes GIT_ASKPASS and the token we supply below is silently
	// ignored — the push goes out as the persisted checkout credential (usually
	// github-actions[bot]) instead of cfg.TagToken.
	persistedAuthHeaders := getGitConfigRegexp(`^http\..*\.extraheader$`)
	for _, h := range persistedAuthHeaders {
		unsetAllGitConfig(h.key)
	}

	defer func() {
		log.Debug("restoring original git configuration")
		if origRemoteURL != "" {
			setGitConfig("remote.origin.url", origRemoteURL)
		} else {
			unsetGitConfig("remote.origin.url")
		}
		// re-add the auth headers we stripped above, preserving order so a
		// multi-valued key is restored exactly as we found it.
		for _, h := range persistedAuthHeaders {
			addGitConfig(h.key, h.value)
		}
	}()

	file.WithTempDir(func(tmpDir string) {
		// write the askpass helper. The script reads the token from an env var
		// rather than embedding it on disk; the temp file holds no secret.
		askpassPath := filepath.Join(tmpDir, "askpass.sh")
		file.Write(askpassPath, askpassScript)
		lang.Throw(os.Chmod(askpassPath, 0o700))

		// use a plain HTTPS URL so the token never appears in git config or ps output
		httpsURL := httpsRemoteURL(cfg.Repository)
		setGitConfig("remote.origin.url", httpsURL)

		// env passed to every git child process in this block:
		//   - GIT_ASKPASS    points at our helper script
		//   - GIT_TERMINAL_PROMPT=0 makes git fail fast (instead of hanging on a
		//     tty prompt) if askpass returns nothing
		//   - ANCHORE_GO_MAKE_TAG_TOKEN is read inside askpass.sh
		//   - LC_ALL=C forces git's "Username for ..." / "Password for ..."
		//     prompts to English so the askpass case-statement matches even on
		//     non-English runner locales
		authEnv := run.Options(
			run.Env("GIT_ASKPASS", askpassPath),
			run.Env("GIT_TERMINAL_PROMPT", "0"),
			run.Env(tagTokenEnvVar, cfg.TagToken),
			run.Env("LC_ALL", "C"),
		)

		// after a successful HTTPS authentication git runs "credential approve",
		// which hands the credential to any configured credential.helper (macOS
		// Keychain, the cache daemon, or a plaintext store file). That would
		// persist cfg.TagToken beyond this process. An empty helper value resets
		// the helper chain for these invocations, so nothing stores the token.
		// This applies to the ls-remote check below as well, which authenticates
		// over HTTPS just like the push.
		noCredentialStore := run.Args("-c", "credential.helper=")

		// check remote tag existence against the NEW (HTTPS, authenticated)
		// remote rather than whatever URL the runner started with.
		if tagExistsRemotely(cfg.Tag, noCredentialStore, authEnv) {
			panic(fmt.Errorf("tag %q already exists on remote", cfg.Tag))
		}

		lang.Return(run.Command("git", noCredentialStore, run.Args("push", "origin", cfg.Tag), authEnv))
	})
}

// tagTokenEnvVar is the name of the env var the askpass helper reads to obtain
// the GitHub token. Using a dedicated, project-prefixed name (not the bare
// caller-facing TAG_TOKEN) keeps the credential passing explicit and decoupled
// from how callers source the token. The name deliberately avoids the bare
// "GO_" prefix so it does not collide with run.skipEnvVar's GO* / CGO_*
// filter on inherited environment.
const tagTokenEnvVar = "ANCHORE_GO_MAKE_TAG_TOKEN" //nolint:gosec // env var name, not a credential

// askpassScript is a POSIX shell script invoked by git when it needs HTTPS
// credentials. Git calls it with a single argument: a prompt string starting
// with "Username" or "Password" (we force LC_ALL=C on the git child so these
// strings are not localized). We respond with "x-access-token" as the
// username and the token from the environment as the password.
//
// SECURITY: the token's safety from shell metacharacter abuse here comes from
// the double-quoted variable expansion ("$ANCHORE_GO_MAKE_TAG_TOKEN"). Inside
// double quotes the shell does NOT re-evaluate the expanded value, so any
// shell-special bytes in the token ($, `, ", \) are treated as literal data.
// validateTagToken's printable-ASCII restriction is defense-in-depth on top
// of that; if the quoting here is ever changed (e.g. to $VAR unquoted, or
// passed through eval), validateTagToken MUST be re-audited.
//
// printf '%s' avoids appending a trailing newline that git would otherwise
// send to the server as part of the credential. The '%s' is a literal in the
// format string, so '%' bytes in the token are not interpreted as format
// specifiers — but again, the real protection is the shell quoting above.
//
//nolint:gosec // shell script template, not a credential
const askpassScript = `#!/bin/sh
case "$1" in
  Username*) printf '%s' 'x-access-token' ;;
  Password*) printf '%s' "$ANCHORE_GO_MAKE_TAG_TOKEN" ;;
esac
`

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

// tagExistsRemotely checks if a tag exists on the remote. Extra options (e.g.
// SSH_AUTH_SOCK or askpass env vars) are forwarded to git so the check is
// performed against the same authenticated remote the caller is about to push
// to, rather than whatever remote URL the runner started with.
func tagExistsRemotely(tag string, extraOpts ...run.Option) bool {
	// apply extraOpts first so any leading git flags they carry (e.g.
	// "-c credential.helper=") land before the ls-remote subcommand, where git
	// requires them. Env-only options are order-independent.
	opts := append([]run.Option{}, extraOpts...)
	opts = append(opts, run.Args("ls-remote", "--tags", "origin", "refs/tags/"+tag), run.NoFail(), run.Quiet())
	output, _ := run.Command("git", opts...)
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

// unsetAllGitConfig removes every value for a git config key. Unlike
// unsetGitConfig (git config --unset), --unset-all also clears multi-valued
// keys and does not error when the key holds more than one value.
func unsetAllGitConfig(key string) {
	_, _ = run.Command("git", run.Args("config", "--unset-all", key), run.NoFail(), run.Quiet())
}

// addGitConfig appends a value to a git config key without replacing existing
// values, so a multi-valued key (e.g. several extraheader entries) can be
// rebuilt one value at a time.
func addGitConfig(key, value string) {
	lang.Return(run.Command("git", run.Args("config", "--add", key, value)))
}

// gitConfigEntry is a single key/value pair as reported by
// `git config --get-regexp`.
type gitConfigEntry struct {
	key   string
	value string
}

// getGitConfigRegexp returns all git config entries whose key matches the given
// regular expression, in the order git reports them. Used to discover persisted
// credentials (e.g. http.<server>.extraheader entries) that must be cleared
// before a token-authenticated push.
func getGitConfigRegexp(pattern string) []gitConfigEntry {
	output, _ := run.Command("git", run.Args("config", "--get-regexp", pattern), run.NoFail(), run.Quiet())

	var entries []gitConfigEntry
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		// git prints each match as "<key> <value>"; an extraheader value contains
		// spaces, so split on the first space only.
		key, value, _ := strings.Cut(line, " ")
		entries = append(entries, gitConfigEntry{key: key, value: value})
	}
	return entries
}
