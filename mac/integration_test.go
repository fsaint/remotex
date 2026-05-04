//go:build integration

package main_test

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestFullSessionLifecycle starts the daemon, creates a session via CLI,
// verifies it via REST, and kills it.
// Run with: go test -tags integration -v -timeout 60s .
func TestFullSessionLifecycle(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	// Set up a temp config dir
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	remotexBin := "./bin/remotex"
	daemonBin := "./bin/remotex-daemon"

	for _, bin := range []string{remotexBin, daemonBin} {
		if _, err := os.Stat(bin); err != nil {
			t.Skipf("binary not found: %s (run: go build -o %s ./cmd/...)", bin, bin)
		}
	}

	// Write a minimal config for the test
	configDir := filepath.Join(dir, ".remotex")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := map[string]interface{}{
		"api_key":        "integration-test-key",
		"tailscale_host": "localhost",
		"daemon_port":    19876,
		"ssh_key_path":   filepath.Join(configDir, "id_ed25519"),
	}
	cfgData, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), cfgData, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Start daemon bound to loopback so the test doesn't need Tailscale.
	daemon := exec.Command(daemonBin)
	daemon.Env = append(os.Environ(), "HOME="+dir, "REMOTEX_BIND_ADDR=127.0.0.1")
	if err := daemon.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer func() {
		daemon.Process.Kill()
		daemon.Wait()
	}()

	// Wait for daemon to be ready
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:19876/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify daemon is up
	resp, err := http.Get("http://127.0.0.1:19876/health")
	if err != nil {
		t.Fatalf("daemon not reachable: %v", err)
	}
	resp.Body.Close()

	// Create session via CLI
	sessionName := "integration-test-session"
	defer exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	newCmd := exec.Command(remotexBin, "new", sessionName)
	newCmd.Env = append(os.Environ(), "HOME="+dir)
	out, err := newCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("remotex new: %v\n%s", err, out)
	}
	t.Logf("remotex new output: %s", out)

	// List sessions via REST API
	req, _ := http.NewRequest("GET", "http://127.0.0.1:19876/sessions", nil)
	req.Header.Set("Authorization", "Bearer integration-test-key")
	listResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /sessions: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != 200 {
		t.Fatalf("GET /sessions status: %d", listResp.StatusCode)
	}

	var sessions []map[string]interface{}
	if err := json.NewDecoder(listResp.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}

	found := false
	for _, s := range sessions {
		if s["name"] == sessionName {
			found = true
			t.Logf("found session: name=%s status=%s", s["name"], s["status"])
			break
		}
	}
	if !found {
		t.Errorf("session %q not found in API response; got: %v", sessionName, sessions)
	}

	// Verify tmux session exists
	if err := exec.Command("tmux", "has-session", "-t", sessionName).Run(); err != nil {
		t.Error("tmux session should exist")
	}

	// Kill session via CLI
	killCmd := exec.Command(remotexBin, "kill", sessionName)
	killCmd.Env = append(os.Environ(), "HOME="+dir)
	out, err = killCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("remotex kill: %v\n%s", err, out)
	}

	// Verify tmux session is gone
	if exec.Command("tmux", "has-session", "-t", sessionName).Run() == nil {
		t.Error("tmux session should be gone after kill")
	}

	t.Log("Integration test passed: create -> list -> kill lifecycle verified")
}
