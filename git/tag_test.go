package git

import (
	"testing"

	"github.com/anchore/go-make/require"
)

func TestBuildSSHCommand(t *testing.T) {
	tests := []struct {
		name           string
		knownHostsPath string
		agentSocket    string
		wantContains   []string
	}{
		{
			name:           "simple paths",
			knownHostsPath: "/tmp/known_hosts",
			agentSocket:    "/tmp/ssh-agent.sock",
			wantContains: []string{
				"ssh",
				"-o StrictHostKeyChecking=yes",
				"-o UserKnownHostsFile=",
				"/tmp/known_hosts",
				"-o IdentityAgent=",
				"/tmp/ssh-agent.sock",
				"-o BatchMode=yes",
			},
		},
		{
			name:           "paths with spaces",
			knownHostsPath: "/tmp/path with spaces/known_hosts",
			agentSocket:    "/tmp/socket path/agent.sock",
			wantContains: []string{
				"ssh",
				"path with spaces",
				"socket path",
			},
		},
		{
			name:           "paths with special characters",
			knownHostsPath: `/tmp/test"path/known_hosts`,
			agentSocket:    "/tmp/test'path/agent.sock",
			wantContains: []string{
				"ssh",
				"-o StrictHostKeyChecking=yes",
				"-o BatchMode=yes",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSSHCommand(tt.knownHostsPath, tt.agentSocket)

			for _, want := range tt.wantContains {
				require.Contains(t, result, want)
			}
		})
	}
}

func TestBuildSSHCommandSecurity(t *testing.T) {
	// verify specific security properties of the SSH command
	t.Run("uses strict host key checking", func(t *testing.T) {
		cmd := buildSSHCommand("/tmp/hosts", "/tmp/sock")
		require.Contains(t, cmd, "StrictHostKeyChecking=yes")
	})

	t.Run("uses explicit identity agent", func(t *testing.T) {
		cmd := buildSSHCommand("/tmp/hosts", "/custom/agent.sock")
		require.Contains(t, cmd, "IdentityAgent=")
		require.Contains(t, cmd, "/custom/agent.sock")
	})

	t.Run("uses batch mode to prevent prompts", func(t *testing.T) {
		cmd := buildSSHCommand("/tmp/hosts", "/tmp/sock")
		require.Contains(t, cmd, "BatchMode=yes")
	})

	t.Run("uses custom known_hosts file", func(t *testing.T) {
		cmd := buildSSHCommand("/custom/known_hosts", "/tmp/sock")
		require.Contains(t, cmd, "UserKnownHostsFile=")
		require.Contains(t, cmd, "/custom/known_hosts")
	})

	t.Run("paths are quoted", func(t *testing.T) {
		cmd := buildSSHCommand("/path/with space/hosts", "/sock/with space")
		// the %q format should quote the paths
		require.Contains(t, cmd, `"`)
	})
}
