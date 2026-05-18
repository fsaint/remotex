package mosh

import (
	"context"
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

// connectRe matches "MOSH CONNECT <port> <key>". Key charset covers standard and URL-safe base64.
var connectRe = regexp.MustCompile(`MOSH CONNECT (\d+) ([A-Za-z0-9/+=-]{22,44})`)

// pidRe matches "pid = N" anywhere in the output to tolerate minor version differences.
var pidRe = regexp.MustCompile(`pid\s*=\s*(\d+)`)

// Start spawns mosh-server bound to bindIP with the given command args and returns its connection info.
// bindIP should be the Tailscale interface address so mosh only accepts UDP on that interface.
// cmdArgs are the argv elements mosh-server will exec (e.g. []string{"tmux", "attach-session", "-t", "work"}).
func Start(bindIP string, cmdArgs []string) (*ServerInfo, error) {
	args := []string{"new", "-s"}
	if bindIP != "" {
		args = append(args, "-i", bindIP)
	}
	if len(cmdArgs) > 0 {
		args = append(args, "--")
		args = append(args, cmdArgs...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "mosh-server", args...)

	// mosh-server prints "MOSH CONNECT <port> <key>" to stdout and
	// "[...pid = N]" to stderr, then exits 0.
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Don't include raw output — it may contain the MOSH CONNECT key.
		return nil, fmt.Errorf("mosh-server: %w", err)
	}

	output := string(out)

	connMatch := connectRe.FindStringSubmatch(output)
	if connMatch == nil {
		return nil, fmt.Errorf("mosh-server did not print MOSH CONNECT")
	}
	port, _ := strconv.Atoi(connMatch[1])
	key := connMatch[2]

	// Give mosh-server a moment to finish daemonizing
	time.Sleep(200 * time.Millisecond)

	pidMatch := pidRe.FindStringSubmatch(output)
	if pidMatch == nil {
		return nil, fmt.Errorf("mosh-server started but could not parse PID from output")
	}
	pid, _ := strconv.Atoi(pidMatch[1])

	return &ServerInfo{Port: port, Key: key, PID: pid}, nil
}

// Stop sends SIGTERM to the mosh-server process.
func Stop(pid int) error {
	if pid <= 0 {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}
