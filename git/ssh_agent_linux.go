package git

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

// sshAgentInfo holds the ssh-agent connection information
type sshAgentInfo struct {
	authSock string
	agentPID int
}

// setupSSHAgent starts an ssh-agent and loads the deploy key into it.
// Returns the agent info and a cleanup function that kills the agent.
// The cleanup function is safe to call multiple times and handles already-exited agents.
func setupSSHAgent(deployKey string) (sshAgentInfo, func()) {
	var agentPID int

	// ensure cleanup happens even if we panic during setup
	cleanup := func() {
		killSSHAgent(agentPID)
	}

	// start ssh-agent and capture output
	var stdout bytes.Buffer
	output, err := run.Command("ssh-agent", run.Args("-s"), run.Stdout(&stdout))
	if err != nil {
		panic(fmt.Errorf("failed to start ssh-agent: %w", err))
	}

	// use stdout buffer if available, otherwise use return value
	agentOutput := stdout.String()
	if agentOutput == "" {
		agentOutput = output
	}

	authSock, pid, err := parseSSHAgentOutput(agentOutput)
	if err != nil {
		panic(fmt.Errorf("failed to parse ssh-agent output: %w", err))
	}
	agentPID = pid // set for cleanup

	log.Debug("started ssh-agent with PID %d, socket %s", agentPID, authSock)

	// add the deploy key to the agent using stdin
	_, err = run.Command("ssh-add",
		run.Args("-"),
		run.Stdin(strings.NewReader(deployKey)),
		run.Env("SSH_AUTH_SOCK", authSock),
	)
	if err != nil {
		cleanup() // kill agent before panicking
		panic(fmt.Errorf("failed to add deploy key to ssh-agent: %w", err))
	}

	log.Debug("added deploy key to ssh-agent")

	return sshAgentInfo{
		authSock: authSock,
		agentPID: agentPID,
	}, cleanup
}

// parseSSHAgentOutput parses the ssh-agent output to extract SSH_AUTH_SOCK and SSH_AGENT_PID.
// Returns an error if required values are missing or invalid.
func parseSSHAgentOutput(output string) (authSock string, agentPID int, err error) {
	// output format: SSH_AUTH_SOCK=/tmp/ssh-XXX/agent.PID; export SSH_AUTH_SOCK;
	//                SSH_AGENT_PID=PID; export SSH_AGENT_PID;
	for line := range strings.SplitSeq(output, ";") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "SSH_AUTH_SOCK="); ok {
			authSock = after
		} else if after, ok := strings.CutPrefix(line, "SSH_AGENT_PID="); ok {
			pidStr := after
			var parseErr error
			agentPID, parseErr = strconv.Atoi(pidStr)
			if parseErr != nil {
				return "", 0, fmt.Errorf("failed to parse SSH_AGENT_PID %q: %w", pidStr, parseErr)
			}
		}
	}

	if authSock == "" {
		return "", 0, fmt.Errorf("SSH_AUTH_SOCK not found in ssh-agent output")
	}
	if agentPID == 0 {
		return "", 0, fmt.Errorf("SSH_AGENT_PID not found in ssh-agent output")
	}

	return authSock, agentPID, nil
}

// killSSHAgent attempts to kill the ssh-agent process. It logs but does not fail on errors
// since the agent may have already exited. Uses SIGTERM first, then SIGKILL as fallback.
func killSSHAgent(pid int) {
	if pid <= 0 {
		return
	}
	log.Debug("killing ssh-agent (PID %d)", pid)
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		// ESRCH means process doesn't exist, which is fine (already exited)
		if err != syscall.ESRCH {
			log.Debug("failed to kill ssh-agent: %v", err)
		}
		return
	}

	// wait briefly for graceful termination, then SIGKILL if still running
	for range 10 {
		time.Sleep(10 * time.Millisecond)
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return // process exited
		}
	}

	// process still running, force kill
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		log.Debug("failed to SIGKILL ssh-agent: %v", err)
	}
}
