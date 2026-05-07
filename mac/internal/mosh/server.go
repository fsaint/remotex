package mosh

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

// ServerInfo holds the connection details printed by mosh-server.
type ServerInfo struct {
	Port int
	Key  string
	PID  int
}

var connectRe = regexp.MustCompile(`MOSH CONNECT (\d+) ([A-Za-z0-9/+]{22,24})`)
var pidRe = regexp.MustCompile(`\[mosh-server detached, pid = (\d+)\]`)

// Start spawns mosh-server with the given command args and returns its connection info.
// cmdArgs are the argv elements mosh-server will exec (e.g. []string{"tmux", "attach-session", "-t", "work"}).
func Start(cmdArgs []string) (*ServerInfo, error) {
	args := []string{"new", "-s"}
	if len(cmdArgs) > 0 {
		args = append(args, "--")
		args = append(args, cmdArgs...)
	}
	cmd := exec.Command("mosh-server", args...)

	// mosh-server prints "MOSH CONNECT <port> <key>" to stdout and
	// "[mosh-server detached, pid = N]" to stderr, then exits 0.
	// Use CombinedOutput so we capture both streams.
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mosh-server: %w; output: %q", err, string(out))
	}

	output := string(out)

	connMatch := connectRe.FindStringSubmatch(output)
	if connMatch == nil {
		return nil, fmt.Errorf("mosh-server did not print MOSH CONNECT; output: %q", output)
	}
	port, _ := strconv.Atoi(connMatch[1])
	key := connMatch[2]

	// Give mosh-server a moment to finish daemonizing
	time.Sleep(200 * time.Millisecond)

	// Find the pid from output
	pid := 0
	if pidMatch := pidRe.FindStringSubmatch(output); pidMatch != nil {
		pid, _ = strconv.Atoi(pidMatch[1])
	}

	return &ServerInfo{Port: port, Key: key, PID: pid}, nil
}

// Stop sends SIGTERM to the mosh-server process.
func Stop(pid int) error {
	if pid <= 0 {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}
