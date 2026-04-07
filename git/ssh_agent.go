//go:build !linux

package git

// sshAgentInfo holds the ssh-agent connection information
type sshAgentInfo struct {
	authSock string
}

// setupSSHAgent is only supported on Linux.
func setupSSHAgent(_ string) (sshAgentInfo, func()) {
	panic("ssh-agent is only supported on Linux")
}
