package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// NewSession creates a new detached tmux session and returns the server pid.
func NewSession(name string) (int, error) {
	if err := exec.Command("tmux", "new-session", "-d", "-s", name).Run(); err != nil {
		return 0, fmt.Errorf("tmux new-session: %w", err)
	}
	return GetPID(name)
}

// KillSession kills an existing tmux session.
func KillSession(name string) error {
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

// SessionExists reports whether a named session is running.
func SessionExists(name string) bool {
	err := exec.Command("tmux", "has-session", "-t", name).Run()
	return err == nil
}

// GetPID returns the pid of the tmux server process for the given session.
func GetPID(name string) (int, error) {
	out, err := exec.Command("tmux", "display-message", "-t", name, "-p", "#{pid}").Output()
	if err != nil {
		return 0, fmt.Errorf("tmux display-message: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse pid %q: %w", string(out), err)
	}
	return pid, nil
}
