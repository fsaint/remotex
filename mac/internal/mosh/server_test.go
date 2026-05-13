package mosh_test

import (
	"os/exec"
	"testing"

	"github.com/fsaint/remotex/internal/mosh"
)

func TestStartAndStop(t *testing.T) {
	if _, err := exec.LookPath("mosh-server"); err != nil {
		t.Skip("mosh-server not installed")
	}

	info, err := mosh.Start("", []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if info.Port <= 0 {
		t.Errorf("expected positive port, got %d", info.Port)
	}
	if len(info.Key) < 22 {
		t.Errorf("expected 22-char key, got %q (len %d)", info.Key, len(info.Key))
	}
	if info.PID <= 0 {
		t.Errorf("expected positive pid, got %d", info.PID)
	}

	mosh.Stop(info.PID)
}
