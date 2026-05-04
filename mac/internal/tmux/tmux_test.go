package tmux_test

import (
	"testing"

	"github.com/fsaint/remotex/internal/tmux"
)

func TestSessionLifecycle(t *testing.T) {
	name := "remotex-test-session"

	// cleanup in case previous test run left it
	_ = tmux.KillSession(name)

	if tmux.SessionExists(name) {
		t.Fatal("session should not exist before creation")
	}

	pid, err := tmux.NewSession(name)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if pid <= 0 {
		t.Errorf("expected positive pid, got %d", pid)
	}
	if !tmux.SessionExists(name) {
		t.Error("session should exist after creation")
	}

	if err := tmux.KillSession(name); err != nil {
		t.Fatalf("KillSession: %v", err)
	}
	if tmux.SessionExists(name) {
		t.Error("session should not exist after kill")
	}
}
