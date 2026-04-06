package git

import (
	"testing"

	"github.com/anchore/go-make/require"
)

func TestParseSSHAgentOutput(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantAuthSock string
		wantPID      int
		wantErr      require.ValidationError
	}{
		// valid cases
		{
			name: "standard ssh-agent output",
			input: `SSH_AUTH_SOCK=/tmp/ssh-XXXXXX/agent.12345; export SSH_AUTH_SOCK;
SSH_AGENT_PID=12345; export SSH_AGENT_PID;
echo Agent pid 12345;`,
			wantAuthSock: "/tmp/ssh-XXXXXX/agent.12345",
			wantPID:      12345,
		},
		{
			name:         "single line format",
			input:        "SSH_AUTH_SOCK=/var/folders/abc/T/ssh-agent.sock; SSH_AGENT_PID=99999;",
			wantAuthSock: "/var/folders/abc/T/ssh-agent.sock",
			wantPID:      99999,
		},
		{
			name:         "with extra whitespace",
			input:        "  SSH_AUTH_SOCK=/tmp/agent.sock  ;  SSH_AGENT_PID=1  ; ",
			wantAuthSock: "/tmp/agent.sock",
			wantPID:      1,
		},
		{
			name:         "socket path with spaces",
			input:        "SSH_AUTH_SOCK=/tmp/path with spaces/agent.sock; SSH_AGENT_PID=42;",
			wantAuthSock: "/tmp/path with spaces/agent.sock",
			wantPID:      42,
		},
		// invalid cases
		{
			name:    "missing SSH_AUTH_SOCK",
			input:   "SSH_AGENT_PID=12345;",
			wantErr: require.Error,
		},
		{
			name:    "missing SSH_AGENT_PID",
			input:   "SSH_AUTH_SOCK=/tmp/agent.sock;",
			wantErr: require.Error,
		},
		{
			name:    "empty output",
			input:   "",
			wantErr: require.Error,
		},
		{
			name:    "invalid PID (not a number)",
			input:   "SSH_AUTH_SOCK=/tmp/agent.sock; SSH_AGENT_PID=abc;",
			wantErr: require.Error,
		},
		{
			name:    "PID is zero",
			input:   "SSH_AUTH_SOCK=/tmp/agent.sock; SSH_AGENT_PID=0;",
			wantErr: require.Error,
		},
		{
			name:    "garbage input",
			input:   "some random garbage that is not ssh-agent output",
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authSock, pid, err := parseSSHAgentOutput(tt.input)
			tt.wantErr.Validate(t, err)

			if err != nil {
				return
			}

			require.Equal(t, tt.wantAuthSock, authSock)
			require.Equal(t, tt.wantPID, pid)
		})
	}
}

func TestSSHAgentInfoStruct(t *testing.T) {
	// test that sshAgentInfo holds expected fields
	info := sshAgentInfo{
		authSock: "/tmp/agent.sock",
		agentPID: 12345,
	}

	require.Equal(t, "/tmp/agent.sock", info.authSock)
	require.Equal(t, 12345, info.agentPID)
}
