//go:build integration

package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/anchore/go-make/require"
)

const inDockerEnv = "IN_DOCKER"

// runInDockerIfNeeded checks if we're inside Docker.
// If not, it fails the test (integration tests must be run via `make integration`).
// If we ARE inside Docker, it returns true and the test should continue.
func runInDockerIfNeeded(t *testing.T) (inDocker bool) {
	t.Helper()

	if os.Getenv(inDockerEnv) == "true" {
		return true
	}

	t.Fatal("integration tests must be run via `make integration`")
	return false
}

// requireLinux fails if not running on Linux.
func requireLinux(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Fatalf("test must run on Linux, got GOOS=%s", runtime.GOOS)
	}
}

// isolatedGitRepo creates an isolated git repository for testing.
// It sets up environment variables to prevent interference from the user's
// global git configuration (no gpg signing, no hooks, no global config).
//
// A local bare repository is created in the same temp directory to act as a
// fake "origin" remote. This allows testing remote operations (push, fetch,
// ls-remote) without any network access - everything stays on the local
// filesystem. The "origin" is simply a bare git repo at a file:// path,
// which git treats identically to a remote for most operations.
type isolatedGitRepo struct {
	t          *testing.T
	repoPath   string
	originPath string // bare repo acting as "origin" (local filesystem path, not a real remote)
	env        []string
}

// newIsolatedGitRepo creates a new isolated git repository for testing.
// A local bare repository is always created to simulate a remote origin,
// enabling testing of push/fetch operations without network access.
func newIsolatedGitRepo(t *testing.T) *isolatedGitRepo {
	t.Helper()

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	err := os.MkdirAll(repoPath, 0755)
	require.NoError(t, err)

	// create a bare repo in the same temp directory that acts as our "remote".
	// Git operations like push/fetch work identically whether the remote is a
	// URL or a local filesystem path.
	originPath := filepath.Join(tmpDir, "origin.git")

	// environment that isolates git from global/system config
	env := append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)

	return &isolatedGitRepo{
		t:          t,
		repoPath:   repoPath,
		originPath: originPath,
		env:        env,
	}
}

func (r *isolatedGitRepo) setup() {
	r.t.Helper()

	// initialize repo
	r.runGit("init")
	r.runGit("config", "user.name", "Test User")
	r.runGit("config", "user.email", "test@example.com")
	r.runGit("config", "commit.gpgsign", "false")
	r.runGit("config", "tag.gpgsign", "false")

	// create initial commit
	readmePath := filepath.Join(r.repoPath, "README.md")
	err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0644)
	require.NoError(r.t, err)

	r.runGit("add", "README.md")
	r.runGit("commit", "-m", "Initial commit")

	// set up bare origin. This creates a local bare repository that serves as
	// our "remote" for testing push/fetch operations. No network access occurs -
	// git simply reads/writes to a local directory.
	r.runGitInDir(filepath.Dir(r.originPath), "init", "--bare", r.originPath)
	r.runGit("remote", "add", "origin", r.originPath)
	r.runGit("push", "-u", "origin", "HEAD:main")
}

func (r *isolatedGitRepo) runGit(args ...string) string {
	r.t.Helper()
	return r.runGitInDir(r.repoPath, args...)
}

func (r *isolatedGitRepo) runGitInDir(dir string, args ...string) string {
	r.t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = r.env
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v failed: %s", args, output)
	}
	return string(output)
}

func (r *isolatedGitRepo) chdir() func() {
	r.t.Helper()

	originalDir, err := os.Getwd()
	require.NoError(r.t, err)

	err = os.Chdir(r.repoPath)
	require.NoError(r.t, err)

	return func() {
		_ = os.Chdir(originalDir)
	}
}

// TestIntegrationCreateTag tests the full CreateTag flow with a real git repository.
func TestIntegrationCreateTag(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	tests := []struct {
		name       string
		tag        string
		tagMessage string
	}{
		{
			name:       "simple semver tag",
			tag:        "v1.0.0",
			tagMessage: "Release v1.0.0",
		},
		{
			name:       "prerelease tag",
			tag:        "v2.0.0-rc.1",
			tagMessage: "Release candidate 1",
		},
		{
			name:       "tag with underscore",
			tag:        "release_v3.0.0",
			tagMessage: "Release v3.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newIsolatedGitRepo(t)
			repo.setup()
			restore := repo.chdir()
			defer restore()

			// create tag using production code
			sha := CreateTag(CreateTagConfig{
				Tag:          tt.tag,
				TagMessage:   tt.tagMessage,
				GitUserName:  "Release Bot",
				GitUserEmail: "release@example.com",
			})

			// verify SHA is returned
			require.NotEmpty(t, sha)

			// verify tag exists locally
			localTags := repo.runGit("tag", "-l", tt.tag)
			require.Equal(t, tt.tag, strings.TrimSpace(localTags))

			// verify tag message
			tagInfo := repo.runGit("tag", "-n1", tt.tag)
			require.Contains(t, tagInfo, tt.tagMessage)

			// verify tag points to expected commit
			taggedSHA := strings.TrimSpace(repo.runGit("rev-parse", tt.tag+"^{}"))
			require.Equal(t, sha, taggedSHA)
		})
	}
}

// TestCreateTagRestoresGitConfig verifies that CreateTag properly restores
// git user configuration after creating a tag.
func TestIntegrationCreateTagRestoresGitConfig(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	repo := newIsolatedGitRepo(t)
	repo.setup()
	restore := repo.chdir()
	defer restore()

	// set initial git config
	originalName := "Original User"
	originalEmail := "original@example.com"
	repo.runGit("config", "user.name", originalName)
	repo.runGit("config", "user.email", originalEmail)

	// create tag (this changes git config temporarily)
	_ = CreateTag(CreateTagConfig{
		Tag:          "v1.0.0",
		TagMessage:   "test release",
		GitUserName:  "Tag User",
		GitUserEmail: "tag@example.com",
	})

	// verify original config is restored
	gotName := strings.TrimSpace(repo.runGit("config", "user.name"))
	gotEmail := strings.TrimSpace(repo.runGit("config", "user.email"))

	require.Equal(t, originalName, gotName)
	require.Equal(t, originalEmail, gotEmail)
}

// TestCreateTagWithOriginAndPush tests tag creation and push to a local bare "origin".
// This exercises the full workflow without needing SSH.
func TestIntegrationCreateTagWithOriginAndPush(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	repo := newIsolatedGitRepo(t)
	repo.setup()
	restore := repo.chdir()
	defer restore()

	tag := "v1.0.0"

	// create tag
	sha := CreateTag(CreateTagConfig{
		Tag:          tag,
		TagMessage:   "Release v1.0.0",
		GitUserName:  "Release Bot",
		GitUserEmail: "release@example.com",
	})

	require.NotEmpty(t, sha)

	// push tag to origin (using local filesystem path, no SSH needed)
	repo.runGit("push", "origin", tag)

	// verify tag exists on "remote" by querying the bare repo
	cmd := exec.Command("git", "tag", "-l", tag)
	cmd.Dir = repo.originPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to list tags on origin: %s", output)
	}
	require.Equal(t, tag, strings.TrimSpace(string(output)))
}

// TestTagExistsLocally verifies the tagExistsLocally helper function.
func TestIntegrationTagExistsLocally(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	repo := newIsolatedGitRepo(t)
	repo.setup()
	restore := repo.chdir()
	defer restore()

	// tag shouldn't exist yet
	require.False(t, tagExistsLocally("v1.0.0"))

	// create a tag
	repo.runGit("tag", "v1.0.0")

	// now it should exist
	require.True(t, tagExistsLocally("v1.0.0"))
}

// TestTagExistsRemotely verifies the tagExistsRemotely helper function.
func TestIntegrationTagExistsRemotely(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	repo := newIsolatedGitRepo(t)
	repo.setup()
	restore := repo.chdir()
	defer restore()

	// tag shouldn't exist on remote yet
	require.False(t, tagExistsRemotely("v1.0.0"))

	// create and push a tag
	repo.runGit("tag", "v1.0.0")
	repo.runGit("push", "origin", "v1.0.0")

	// now it should exist on remote
	require.True(t, tagExistsRemotely("v1.0.0"))
}

// TestCreateTagRejectsExistingTag verifies CreateTag panics if tag already exists.
func TestIntegrationCreateTagRejectsExistingTag(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	repo := newIsolatedGitRepo(t)
	repo.setup()
	restore := repo.chdir()
	defer restore()

	// create a tag first
	repo.runGit("tag", "-a", "-m", "existing tag", "v1.0.0")

	// attempting to create the same tag should panic
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected CreateTag to panic for existing tag")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("expected error, got %T: %v", r, r)
		}
		require.Contains(t, err.Error(), "already exists locally")
	}()

	CreateTag(CreateTagConfig{
		Tag:          "v1.0.0",
		TagMessage:   "duplicate tag",
		GitUserName:  "Test User",
		GitUserEmail: "test@example.com",
	})
}

// =============================================================================
// SSH Agent Tests
// =============================================================================

// generateTestDeployKey generates an ED25519 SSH key pair for testing.
// Returns the private key in PEM format.
func generateTestDeployKey(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test_key")

	cmd := exec.Command("ssh-keygen",
		"-t", "ed25519",
		"-f", keyPath,
		"-N", "", // no passphrase
		"-C", "integration-test-key",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to generate test key: %s", output)
	}

	keyData, err := os.ReadFile(keyPath)
	require.NoError(t, err)

	return string(keyData)
}

// TestSSHAgentSetup tests that setupSSHAgent correctly starts an agent and loads a key.
func TestIntegrationSSHAgentSetup(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	testKey := generateTestDeployKey(t)

	// exercise the production setupSSHAgent function
	agentInfo, cleanup := setupSSHAgent(testKey)
	defer cleanup()

	// verify agent socket path is set
	if agentInfo.authSock == "" {
		t.Fatal("expected authSock to be set")
	}

	// verify agent PID is set
	if agentInfo.agentPID <= 0 {
		t.Fatal("expected agentPID to be positive")
	}

	// verify agent socket file exists
	if _, err := os.Stat(agentInfo.authSock); os.IsNotExist(err) {
		t.Fatalf("agent socket does not exist: %s", agentInfo.authSock)
	}

	// verify key is loaded by running ssh-add -l
	cmd := exec.Command("ssh-add", "-l")
	cmd.Env = append(os.Environ(), fmt.Sprintf("SSH_AUTH_SOCK=%s", agentInfo.authSock))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ssh-add -l failed: %s", output)
	}

	// output should contain the key fingerprint
	if !strings.Contains(string(output), "ED25519") && !strings.Contains(string(output), "256") {
		t.Fatalf("expected key to be loaded, got: %s", output)
	}

	// verify the agent process is running
	if err := syscall.Kill(agentInfo.agentPID, 0); err != nil {
		t.Fatalf("agent process not running: %v", err)
	}
}

// TestSSHAgentCleanup tests that the cleanup function properly kills the agent.
func TestIntegrationSSHAgentCleanup(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	testKey := generateTestDeployKey(t)

	agentInfo, cleanup := setupSSHAgent(testKey)
	pid := agentInfo.agentPID

	// verify agent is running before cleanup (should be 'S' state, not 'Z' zombie)
	if !isProcessRunning(t, pid) {
		t.Fatal("agent should be running before cleanup")
	}

	// run cleanup
	cleanup()

	// poll for process termination with timeout
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if !isProcessRunning(t, pid) {
			return // success - process terminated
		}
		time.Sleep(10 * time.Millisecond)
	}

	// verify process is dead (either gone or zombie)
	// zombie is acceptable - it means the process terminated, just hasn't been reaped
	if isProcessRunning(t, pid) {
		t.Fatal("agent process should be terminated after cleanup")
	}
}

// isProcessRunning checks if a process is actually running (not just existing as a zombie).
// Returns true only if the process exists AND is not a zombie.
func isProcessRunning(t *testing.T, pid int) bool {
	t.Helper()

	// first check if process exists at all
	if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
		return false // process doesn't exist
	}

	// check if it's a zombie by reading /proc/<pid>/stat
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return false // can't read stat, assume dead
	}

	// stat format: pid (comm) state ...
	// find the state which is after the closing paren of comm
	statStr := string(data)
	closeIdx := strings.LastIndex(statStr, ")")
	if closeIdx == -1 || closeIdx+2 >= len(statStr) {
		return false
	}
	state := statStr[closeIdx+2] // skip ") "

	// Z = zombie, X = dead
	return state != 'Z' && state != 'X'
}

// TestSSHAgentCleanupIdempotent tests that cleanup can be called multiple times safely.
func TestIntegrationSSHAgentCleanupIdempotent(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	testKey := generateTestDeployKey(t)

	_, cleanup := setupSSHAgent(testKey)

	// call cleanup multiple times - should not panic
	cleanup()
	cleanup()
	cleanup()
}

// TestParseSSHAgentOutput tests the parseSSHAgentOutput function with various inputs.
func TestIntegrationParseSSHAgentOutput(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	tests := []struct {
		name        string
		input       string
		wantSock    string
		wantPID     int
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid output",
			input:    "SSH_AUTH_SOCK=/tmp/ssh-XXX/agent.12345; export SSH_AUTH_SOCK;\nSSH_AGENT_PID=12345; export SSH_AGENT_PID;",
			wantSock: "/tmp/ssh-XXX/agent.12345",
			wantPID:  12345,
		},
		{
			name:     "valid output with different path",
			input:    "SSH_AUTH_SOCK=/var/folders/abc/xyz/T/ssh-abc123/agent.99999; export SSH_AUTH_SOCK;\nSSH_AGENT_PID=99999; export SSH_AGENT_PID;",
			wantSock: "/var/folders/abc/xyz/T/ssh-abc123/agent.99999",
			wantPID:  99999,
		},
		{
			name:        "missing SSH_AUTH_SOCK",
			input:       "SSH_AGENT_PID=12345; export SSH_AGENT_PID;",
			wantErr:     true,
			errContains: "SSH_AUTH_SOCK not found",
		},
		{
			name:        "missing SSH_AGENT_PID",
			input:       "SSH_AUTH_SOCK=/tmp/ssh-XXX/agent.12345; export SSH_AUTH_SOCK;",
			wantErr:     true,
			errContains: "SSH_AGENT_PID not found",
		},
		{
			name:        "invalid PID",
			input:       "SSH_AUTH_SOCK=/tmp/ssh-XXX/agent.12345; export SSH_AUTH_SOCK;\nSSH_AGENT_PID=notanumber; export SSH_AGENT_PID;",
			wantErr:     true,
			errContains: "failed to parse SSH_AGENT_PID",
		},
		{
			name:        "empty output",
			input:       "",
			wantErr:     true,
			errContains: "SSH_AUTH_SOCK not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sock, pid, err := parseSSHAgentOutput(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got: %v", tt.errContains, err)
				}
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantSock, sock)
			require.Equal(t, tt.wantPID, pid)
		})
	}
}

// =============================================================================
// PushTag Tests
// =============================================================================

// TestPushTagRejectsNonExistentTag verifies PushTag panics if the tag doesn't exist locally.
func TestIntegrationPushTagRejectsNonExistentTag(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	repo := newIsolatedGitRepo(t)
	repo.setup()
	restore := repo.chdir()
	defer restore()

	testKey := generateTestDeployKey(t)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected PushTag to panic for non-existent tag")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("expected error, got %T: %v", r, r)
		}
		require.Contains(t, err.Error(), "does not exist locally")
	}()

	PushTag(PushTagConfig{
		Tag:        "v1.0.0", // tag doesn't exist
		Repository: "owner/repo",
		DeployKey:  testKey,
	})
}

// TestPushTagRejectsExistingRemoteTag verifies PushTag panics if the tag already exists on remote.
func TestIntegrationPushTagRejectsExistingRemoteTag(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	repo := newIsolatedGitRepo(t)
	repo.setup()
	restore := repo.chdir()
	defer restore()

	testKey := generateTestDeployKey(t)

	// point the push at the local bare origin. Without this, PushTag switches the
	// remote to git@github.com:owner/repo.git and the existence check runs against
	// real GitHub (unreachable here) instead of the tag we just pushed locally.
	redirectRemoteToLocal(t, repo.originPath)

	// create and push tag to origin first
	repo.runGit("tag", "v1.0.0")
	repo.runGit("push", "origin", "v1.0.0")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected PushTag to panic for tag that exists on remote")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("expected error, got %T: %v", r, r)
		}
		require.Contains(t, err.Error(), "already exists on remote")
	}()

	PushTag(PushTagConfig{
		Tag:        "v1.0.0", // tag already pushed
		Repository: "owner/repo",
		DeployKey:  testKey,
	})
}

// redirectRemoteToLocal points the production remote-URL builders at a local
// bare repo for the duration of the test, so PushTag's ls-remote and push run
// against the local filesystem instead of github.com. Both transports collapse
// to the same filesystem path (git ignores the SSH/HTTPS auth setup when the
// remote is a local path), which lets the token and deploy-key paths be
// exercised end-to-end offline.
func redirectRemoteToLocal(t *testing.T, originPath string) {
	t.Helper()
	require.SetAndRestore(t, &sshRemoteURL, func(string) string { return originPath })
	require.SetAndRestore(t, &httpsRemoteURL, func(string) string { return originPath })
}

// validTestToken is a syntactically valid GitHub token (passes validateTagToken)
// used by the token-path integration tests. Its value is never authenticated
// because the push targets a local bare repo.
const validTestToken = "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// TestIntegrationPushTagWithTokenSucceeds verifies the HTTPS/token path pushes
// the tag end-to-end: PushTag dispatches to pushTagWithToken, and the tag lands
// on the (local, stand-in) remote.
func TestIntegrationPushTagWithTokenSucceeds(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	repo := newIsolatedGitRepo(t)
	repo.setup()
	restore := repo.chdir()
	defer restore()

	redirectRemoteToLocal(t, repo.originPath)

	// create the tag locally but do not push it yet; PushTag should push it
	repo.runGit("tag", "v1.0.0")

	PushTag(PushTagConfig{
		Tag:        "v1.0.0",
		Repository: "owner/repo",
		TagToken:   validTestToken,
	})

	// the tag should now exist on the remote
	remoteTags := repo.runGit("ls-remote", "--tags", "origin", "refs/tags/v1.0.0")
	require.Contains(t, remoteTags, "refs/tags/v1.0.0")
}

// TestIntegrationPushTagWithTokenRejectsExistingRemoteTag verifies the token
// path also refuses to clobber a tag that already exists on the remote.
func TestIntegrationPushTagWithTokenRejectsExistingRemoteTag(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	repo := newIsolatedGitRepo(t)
	repo.setup()
	restore := repo.chdir()
	defer restore()

	redirectRemoteToLocal(t, repo.originPath)

	// create and push tag to origin first so it already exists remotely
	repo.runGit("tag", "v1.0.0")
	repo.runGit("push", "origin", "v1.0.0")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected PushTag to panic for tag that exists on remote")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("expected error, got %T: %v", r, r)
		}
		require.Contains(t, err.Error(), "already exists on remote")
	}()

	PushTag(PushTagConfig{
		Tag:        "v1.0.0",
		Repository: "owner/repo",
		TagToken:   validTestToken,
	})
}

// TestIntegrationPushTagWithTokenPreservesPersistedAuthHeaders verifies the
// token path tolerates a persisted http.*.extraheader (what actions/checkout
// writes) and leaves the stored config untouched: it neutralizes the header
// per-command via "-c <key>=" rather than stripping it, so the entry is still
// present, unchanged, after the push completes.
func TestIntegrationPushTagWithTokenPreservesPersistedAuthHeaders(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	repo := newIsolatedGitRepo(t)
	repo.setup()
	restore := repo.chdir()
	defer restore()

	redirectRemoteToLocal(t, repo.originPath)

	// simulate the credential actions/checkout persists into the local config,
	// including a second value to exercise the multi-valued case.
	const extraHeaderKey = "http.https://github.com/.extraheader"
	repo.runGit("config", "--add", extraHeaderKey, "AUTHORIZATION: basic dG9rZW4=")
	repo.runGit("config", "--add", extraHeaderKey, "AUTHORIZATION: bearer second")

	repo.runGit("tag", "v1.0.0")

	PushTag(PushTagConfig{
		Tag:        "v1.0.0",
		Repository: "owner/repo",
		TagToken:   validTestToken,
	})

	// the persisted header must be left exactly as it was: the token path never
	// mutates stored config, so both values remain in their original order.
	got := repo.runGit("config", "--get-all", extraHeaderKey)
	require.Equal(t, "AUTHORIZATION: basic dG9rZW4=\nAUTHORIZATION: bearer second\n", got)
}

// TestIntegrationGitConfigKeysMatching verifies the value-free discovery behind
// pushTagWithToken: gitConfigKeysMatching returns the distinct key NAMES of
// persisted http.*.extraheader entries (what actions/checkout writes) without
// ever reading their values, and collapses a multi-valued key to a single name.
func TestIntegrationGitConfigKeysMatching(t *testing.T) {
	if !runInDockerIfNeeded(t) {
		return
	}
	requireLinux(t)

	repo := newIsolatedGitRepo(t)
	repo.setup()
	restore := repo.chdir()
	defer restore()

	const key = "http.https://github.com/.extraheader"
	// a multi-valued key: gitConfigKeysMatching must still report it just once.
	repo.runGit("config", "--add", key, "AUTHORIZATION: basic dG9rZW4=")
	repo.runGit("config", "--add", key, "AUTHORIZATION: bearer second")

	keys := gitConfigKeysMatching(`^http\..*\.extraheader$`)

	require.Equal(t, []string{key}, keys)

	// the returned key name must not carry the (secret) header value.
	for _, k := range keys {
		require.False(t, strings.Contains(k, "AUTHORIZATION"))
		require.False(t, strings.Contains(k, "dG9rZW4="))
	}

	// discovery must not mutate the stored config.
	got := repo.runGit("config", "--get-all", key)
	require.Equal(t, "AUTHORIZATION: basic dG9rZW4=\nAUTHORIZATION: bearer second\n", got)
}
