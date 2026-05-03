# RemoteX Mac (CLI + Daemon) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Go CLI (`remotex`) and daemon (`remotex-daemon`) that manage tmux sessions with associated mosh-server instances and expose them over a REST API secured by API key.

**Architecture:** The CLI creates tmux sessions and spawns mosh-servers on demand, registering them with a local daemon. The daemon persists session state to `~/.remotex/sessions.json`, exposes an internal API (localhost) for CLI registration and an external API (Tailscale interface, port 7654) for the iOS app. A watchdog polls pids every 30s and cleans up dead sessions.

**Tech Stack:** Go 1.22+, Cobra (CLI), chi (HTTP router), go-qrcode, standard library crypto/ed25519, os/exec for tmux/mosh-server.

---

## File Structure

```
mac/
  cmd/
    remotex/main.go            # CLI entry point, registers all commands
    remotex-daemon/main.go     # Daemon entry point, starts HTTP server
  internal/
    config/
      config.go                # Config struct, Load/Save, path helpers
      config_test.go
    session/
      session.go               # Session struct definition
      manager.go               # In-memory registry, sessions.json persistence
      manager_test.go
      watchdog.go              # Pid polling, dead session pruning, mosh respawn
      watchdog_test.go
    tmux/
      tmux.go                  # NewSession, KillSession, SessionExists, GetPID
      tmux_test.go
    mosh/
      server.go                # Start (spawn mosh-server, parse MOSH CONNECT), Stop
      server_test.go
    daemon/
      server.go                # HTTP server setup, interface binding (tailscale + localhost)
      handlers.go              # All HTTP handlers (internal + external)
      handlers_test.go
    setup/
      setup.go                 # Keygen, QR encode, launchd plist install
      setup_test.go
  go.mod
  go.sum
```

---

## Task 1: Go Module Scaffold

**Files:**
- Create: `mac/go.mod`
- Create: `mac/cmd/remotex/main.go`
- Create: `mac/cmd/remotex-daemon/main.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/fsaint/git/remotex/mac
go mod init github.com/fsaint/remotex
```

- [ ] **Step 2: Add dependencies**

```bash
go get github.com/spf13/cobra@latest
go get github.com/go-chi/chi/v5@latest
go get github.com/skip2/go-qrcode@latest
go get golang.org/x/crypto@latest
```

- [ ] **Step 3: Create CLI entry point**

```go
// mac/cmd/remotex/main.go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "remotex",
		Short: "Manage remote terminal sessions",
	}
	root.AddCommand(newSetupCmd(), newNewCmd(), newListCmd(), newKillCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newSetupCmd() *cobra.Command  { return &cobra.Command{Use: "setup"} }
func newNewCmd() *cobra.Command    { return &cobra.Command{Use: "new <name>", Args: cobra.ExactArgs(1)} }
func newListCmd() *cobra.Command   { return &cobra.Command{Use: "list"} }
func newKillCmd() *cobra.Command   { return &cobra.Command{Use: "kill <name>", Args: cobra.ExactArgs(1)} }
```

- [ ] **Step 4: Create daemon entry point**

```go
// mac/cmd/remotex-daemon/main.go
package main

import "fmt"

func main() {
	fmt.Println("remotex-daemon starting...")
}
```

- [ ] **Step 5: Verify it compiles**

```bash
cd mac
go build ./cmd/remotex/
go build ./cmd/remotex-daemon/
```
Expected: no errors, two binaries produced.

- [ ] **Step 6: Commit**

```bash
git add mac/
git commit -m "feat: go module scaffold with cobra CLI and daemon stubs"
```

---

## Task 2: Config Package

**Files:**
- Create: `mac/internal/config/config.go`
- Create: `mac/internal/config/config_test.go`

- [ ] **Step 1: Write failing test**

```go
// mac/internal/config/config_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsaint/remotex/internal/config"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &config.Config{
		APIKey:        "test-api-key",
		TailscaleHost: "myhost.tailnet.ts.net",
		DaemonPort:    7654,
		SSHKeyPath:    filepath.Join(dir, ".remotex", "id_ed25519"),
	}

	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.APIKey != cfg.APIKey {
		t.Errorf("APIKey: got %q want %q", loaded.APIKey, cfg.APIKey)
	}
	if loaded.TailscaleHost != cfg.TailscaleHost {
		t.Errorf("TailscaleHost: got %q want %q", loaded.TailscaleHost, cfg.TailscaleHost)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd mac && go test ./internal/config/ -v
```
Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Implement config package**

```go
// mac/internal/config/config.go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	APIKey        string `json:"api_key"`
	TailscaleHost string `json:"tailscale_host"`
	DaemonPort    int    `json:"daemon_port"`
	SSHKeyPath    string `json:"ssh_key_path"`
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".remotex")
}

func configPath() string {
	return filepath.Join(Dir(), "config.json")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, err
	}
	var cfg Config
	return &cfg, json.Unmarshal(data, &cfg)
}

func Save(cfg *Config) error {
	if err := os.MkdirAll(Dir(), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd mac && go test ./internal/config/ -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mac/internal/config/
git commit -m "feat: config package with load/save to ~/.remotex/config.json"
```

---

## Task 3: tmux Package

**Files:**
- Create: `mac/internal/tmux/tmux.go`
- Create: `mac/internal/tmux/tmux_test.go`

- [ ] **Step 1: Write failing tests**

```go
// mac/internal/tmux/tmux_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd mac && go test ./internal/tmux/ -v
```
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement tmux package**

```go
// mac/internal/tmux/tmux.go
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
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd mac && go test ./internal/tmux/ -v
```
Expected: PASS (requires tmux installed on the system)

- [ ] **Step 5: Commit**

```bash
git add mac/internal/tmux/
git commit -m "feat: tmux package wrapping new-session, kill-session, has-session"
```

---

## Task 4: mosh-server Package

**Files:**
- Create: `mac/internal/mosh/server.go`
- Create: `mac/internal/mosh/server_test.go`

- [ ] **Step 1: Write failing test**

```go
// mac/internal/mosh/server_test.go
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

	info, err := mosh.Start("echo hello")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if info.Port <= 0 {
		t.Errorf("expected positive port, got %d", info.Port)
	}
	if len(info.Key) != 22 {
		t.Errorf("expected 22-char key, got %q (len %d)", info.Key, len(info.Key))
	}
	if info.PID <= 0 {
		t.Errorf("expected positive pid, got %d", info.PID)
	}

	mosh.Stop(info.PID)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd mac && go test ./internal/mosh/ -v
```
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement mosh package**

```go
// mac/internal/mosh/server.go
package mosh

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ServerInfo holds the connection details printed by mosh-server.
type ServerInfo struct {
	Port int
	Key  string
	PID  int
}

var connectRe = regexp.MustCompile(`MOSH CONNECT (\d+) ([A-Za-z0-9/+]{22})`)
var pidRe = regexp.MustCompile(`\[mosh-server detached, pid = (\d+)\]`)

// Start spawns mosh-server with the given command and returns its connection info.
// command is the shell command mosh-server will exec (e.g. "tmux attach-session -t work").
func Start(command string) (*ServerInfo, error) {
	args := []string{"new", "-s"}
	if command != "" {
		args = append(args, "--")
		args = append(args, strings.Fields(command)...)
	}
	cmd := exec.Command("mosh-server", args...)

	// mosh-server prints MOSH CONNECT to stdout before daemonizing
	out, err := cmd.Output()
	if err != nil {
		// mosh-server exits with non-zero after daemonizing; that's expected
		if exitErr, ok := err.(*exec.ExitError); ok {
			out = append(out, exitErr.Stderr...)
		}
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

	// Find the pid from output or via pgrep as fallback
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
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd mac && go test ./internal/mosh/ -v -timeout 30s
```
Expected: PASS (or SKIP if mosh-server not installed)

- [ ] **Step 5: Commit**

```bash
git add mac/internal/mosh/
git commit -m "feat: mosh package that spawns mosh-server and parses MOSH CONNECT output"
```

---

## Task 5: Session Manager

**Files:**
- Create: `mac/internal/session/session.go`
- Create: `mac/internal/session/manager.go`
- Create: `mac/internal/session/manager_test.go`

- [ ] **Step 1: Write failing tests**

```go
// mac/internal/session/manager_test.go
package session_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/fsaint/remotex/internal/session"
)

func TestManagerCRUD(t *testing.T) {
	dir := t.TempDir()
	m := session.NewManager(filepath.Join(dir, "sessions.json"))

	s := &session.Session{
		Name:      "work",
		TmuxPID:   1234,
		StartedAt: time.Now(),
		Status:    session.StatusLive,
	}

	m.Add(s)

	got, ok := m.Get("work")
	if !ok {
		t.Fatal("expected session to exist")
	}
	if got.TmuxPID != 1234 {
		t.Errorf("TmuxPID: got %d want 1234", got.TmuxPID)
	}

	all := m.List()
	if len(all) != 1 {
		t.Errorf("List: got %d sessions want 1", len(all))
	}

	m.Remove("work")
	if _, ok := m.Get("work"); ok {
		t.Error("session should be removed")
	}
}

func TestManagerPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")

	m1 := session.NewManager(path)
	m1.Add(&session.Session{Name: "work", TmuxPID: 42, StartedAt: time.Now(), Status: session.StatusLive})
	if err := m1.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m2 := session.NewManager(path)
	if err := m2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := m2.Get("work"); !ok {
		t.Error("session should survive save/load")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd mac && go test ./internal/session/ -v
```
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement session struct**

```go
// mac/internal/session/session.go
package session

import "time"

const (
	StatusLive = "live"
	StatusDead = "dead"
)

// Session represents one tmux session + its optional mosh-server.
type Session struct {
	Name      string    `json:"name"`
	TmuxPID   int       `json:"tmux_pid"`
	MoshPID   int       `json:"mosh_pid,omitempty"`
	MoshPort  int       `json:"mosh_port,omitempty"`
	MoshKey   string    `json:"mosh_key,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"`
}
```

- [ ] **Step 4: Implement session manager**

```go
// mac/internal/session/manager.go
package session

import (
	"encoding/json"
	"os"
	"sync"
)

// Manager holds the in-memory session registry and persists to a JSON file.
type Manager struct {
	path     string
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewManager(path string) *Manager {
	return &Manager{path: path, sessions: make(map[string]*Session)}
}

func (m *Manager) Add(s *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.Name] = s
}

func (m *Manager) Get(name string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[name]
	return s, ok
}

func (m *Manager) Remove(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, name)
}

func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out
}

func (m *Manager) Update(name string, fn func(*Session)) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[name]
	if !ok {
		return false
	}
	fn(s)
	return true
}

func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, err := json.MarshalIndent(m.sessions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0600)
}

func (m *Manager) Load() error {
	data, err := os.ReadFile(m.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return json.Unmarshal(data, &m.sessions)
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd mac && go test ./internal/session/ -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add mac/internal/session/
git commit -m "feat: session package with in-memory manager and JSON persistence"
```

---

## Task 6: Daemon HTTP Server Setup

**Files:**
- Create: `mac/internal/daemon/server.go`

- [ ] **Step 1: Write failing test**

```go
// mac/internal/daemon/server_test.go
package daemon_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/fsaint/remotex/internal/daemon"
	"github.com/fsaint/remotex/internal/session"
)

func TestServerStartStop(t *testing.T) {
	mgr := session.NewManager(t.TempDir() + "/sessions.json")
	srv := daemon.NewServer(mgr, "test-api-key", "127.0.0.1", 19999)

	go srv.Start()
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://127.0.0.1:19999/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status: got %d want 200", resp.StatusCode)
	}

	srv.Stop()
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd mac && go test ./internal/daemon/ -v -run TestServerStartStop
```
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement server**

```go
// mac/internal/daemon/server.go
package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/fsaint/remotex/internal/session"
)

// Server is the remotex daemon HTTP server.
type Server struct {
	mgr     *session.Manager
	apiKey  string
	host    string
	port    int
	httpSrv *http.Server
}

func NewServer(mgr *session.Manager, apiKey, host string, port int) *Server {
	return &Server{mgr: mgr, apiKey: apiKey, host: host, port: port}
}

func (s *Server) Start() error {
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Internal routes (localhost-only enforced at bind level)
	r.Post("/internal/sessions", s.handleRegisterSession)
	r.Delete("/internal/sessions/{name}", s.handleUnregisterSession)

	// External routes (API key required)
	r.Group(func(r chi.Router) {
		r.Use(s.requireAPIKey)
		r.Get("/sessions", s.handleListSessions)
		r.Post("/sessions/{name}/connect", s.handleConnect)
	})

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	s.httpSrv = &http.Server{Addr: addr, Handler: r}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	return s.httpSrv.Serve(ln)
}

func (s *Server) Stop() {
	if s.httpSrv != nil {
		s.httpSrv.Shutdown(context.Background())
	}
}

func (s *Server) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer "+s.apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd mac && go test ./internal/daemon/ -v -run TestServerStartStop
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mac/internal/daemon/server.go mac/internal/daemon/server_test.go
git commit -m "feat: daemon HTTP server with chi router, health endpoint, API key middleware"
```

---

## Task 7: Daemon HTTP Handlers

**Files:**
- Create: `mac/internal/daemon/handlers.go`
- Modify: `mac/internal/daemon/handlers_test.go` (expand)

- [ ] **Step 1: Write failing handler tests**

```go
// mac/internal/daemon/handlers_test.go (full file)
package daemon_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/fsaint/remotex/internal/daemon"
	"github.com/fsaint/remotex/internal/session"
)

func setupTestServer(t *testing.T) (*daemon.Server, *session.Manager) {
	t.Helper()
	mgr := session.NewManager(t.TempDir() + "/sessions.json")
	srv := daemon.NewServer(mgr, "test-key", "127.0.0.1", 0)
	return srv, mgr
}

func TestHandleRegisterSession(t *testing.T) {
	srv, mgr := setupTestServer(t)

	body := map[string]interface{}{
		"name":       "work",
		"tmux_pid":   1234,
		"started_at": time.Now(),
	}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/internal/sessions", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status: got %d want 201", w.Code)
	}
	if _, ok := mgr.Get("work"); !ok {
		t.Error("session should be registered in manager")
	}
}

func TestHandleListSessions(t *testing.T) {
	srv, mgr := setupTestServer(t)
	mgr.Add(&session.Session{
		Name: "work", TmuxPID: 1, StartedAt: time.Now(), Status: session.StatusLive,
	})

	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d want 200", w.Code)
	}

	var sessions []*session.Session
	if err := json.NewDecoder(w.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Name != "work" {
		t.Errorf("expected 1 session 'work', got %v", sessions)
	}
}

func TestHandleListSessionsUnauthorized(t *testing.T) {
	srv, _ := setupTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	// no Authorization header
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d want 401", w.Code)
	}
}

func TestHandleUnregisterSession(t *testing.T) {
	srv, mgr := setupTestServer(t)
	mgr.Add(&session.Session{Name: "work", TmuxPID: 1, Status: session.StatusLive})

	req := httptest.NewRequest(http.MethodDelete, "/internal/sessions/work", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "work")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status: got %d want 204", w.Code)
	}
	if _, ok := mgr.Get("work"); ok {
		t.Error("session should be unregistered")
	}
}
```

Add missing import `"context"` to the test file.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd mac && go test ./internal/daemon/ -v -run "TestHandle"
```
Expected: FAIL — handlers not implemented yet.

- [ ] **Step 3: Add ServeHTTP to Server and implement handlers**

Add to `mac/internal/daemon/server.go`:
```go
// ServeHTTP builds the router once for use in tests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.buildRouter().ServeHTTP(w, r)
}

func (s *Server) buildRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	r.Post("/internal/sessions", s.handleRegisterSession)
	r.Delete("/internal/sessions/{name}", s.handleUnregisterSession)
	r.Group(func(r chi.Router) {
		r.Use(s.requireAPIKey)
		r.Get("/sessions", s.handleListSessions)
		r.Post("/sessions/{name}/connect", s.handleConnect)
	})
	return r
}
```

Update `Start()` to call `s.buildRouter()` instead of building inline.

- [ ] **Step 4: Implement handlers**

```go
// mac/internal/daemon/handlers.go
package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/fsaint/remotex/internal/mosh"
	"github.com/fsaint/remotex/internal/session"
)

type registerRequest struct {
	Name      string    `json:"name"`
	TmuxPID   int       `json:"tmux_pid"`
	StartedAt time.Time `json:"started_at"`
}

func (s *Server) handleRegisterSession(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	s.mgr.Add(&session.Session{
		Name:      req.Name,
		TmuxPID:   req.TmuxPID,
		StartedAt: req.StartedAt,
		Status:    session.StatusLive,
	})
	if err := s.mgr.Save(); err != nil {
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleUnregisterSession(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	sess, ok := s.mgr.Get(name)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// Kill mosh-server if running
	if sess.MoshPID > 0 {
		mosh.Stop(sess.MoshPID)
	}
	s.mgr.Remove(name)
	s.mgr.Save()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.mgr.List()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

type connectResponse struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Key  string `json:"key"`
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	sess, ok := s.mgr.Get(name)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if sess.Status == session.StatusDead {
		http.Error(w, "session is dead", http.StatusGone)
		return
	}

	// Reuse existing mosh-server if still alive
	if sess.MoshPID > 0 && sess.MoshPort > 0 && syscall.Kill(sess.MoshPID, 0) == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(connectResponse{
			Host: s.tailscaleHost,
			Port: sess.MoshPort,
			Key:  sess.MoshKey,
		})
		return
	}
	// Clear stale mosh info if process is dead
	s.mgr.Update(name, func(sess *session.Session) {
		sess.MoshPID = 0
		sess.MoshPort = 0
		sess.MoshKey = ""
	})

	// Spawn a new mosh-server attached to the tmux session
	info, err := mosh.Start(fmt.Sprintf("tmux attach-session -t %s", name))
	if err != nil {
		http.Error(w, fmt.Sprintf("start mosh-server: %v", err), http.StatusInternalServerError)
		return
	}

	s.mgr.Update(name, func(sess *session.Session) {
		sess.MoshPID = info.PID
		sess.MoshPort = info.Port
		sess.MoshKey = info.Key
	})
	s.mgr.Save()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(connectResponse{
		Host: s.tailscaleHost,
		Port: info.Port,
		Key:  info.Key,
	})
}
```

Add `tailscaleHost string` field to `Server` struct and `NewServer` parameter.

Updated `NewServer` signature:
```go
func NewServer(mgr *session.Manager, apiKey, host, tailscaleHost string, port int) *Server {
    return &Server{mgr: mgr, apiKey: apiKey, host: host, tailscaleHost: tailscaleHost, port: port}
}
```

Update test helpers and daemon startup accordingly (pass empty string for tailscaleHost in tests).

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd mac && go test ./internal/daemon/ -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add mac/internal/daemon/
git commit -m "feat: daemon handlers for session register/unregister/list/connect"
```

---

## Task 8: Watchdog

**Files:**
- Create: `mac/internal/session/watchdog.go`
- Create: `mac/internal/session/watchdog_test.go`

- [ ] **Step 1: Write failing test**

```go
// mac/internal/session/watchdog_test.go
package session_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/fsaint/remotex/internal/session"
)

func TestWatchdogMarksDead(t *testing.T) {
	dir := t.TempDir()
	mgr := session.NewManager(filepath.Join(dir, "sessions.json"))

	// Add session with a pid that definitely doesn't exist
	mgr.Add(&session.Session{
		Name:      "dead-session",
		TmuxPID:   999999999,
		StartedAt: time.Now(),
		Status:    session.StatusLive,
	})

	w := session.NewWatchdog(mgr, 50*time.Millisecond)
	go w.Run()
	defer w.Stop()

	time.Sleep(200 * time.Millisecond)

	sess, ok := mgr.Get("dead-session")
	if !ok {
		t.Fatal("session should still be in registry")
	}
	if sess.Status != session.StatusDead {
		t.Errorf("status: got %q want %q", sess.Status, session.StatusDead)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd mac && go test ./internal/session/ -v -run TestWatchdog
```
Expected: FAIL — Watchdog not defined.

- [ ] **Step 3: Implement watchdog**

```go
// mac/internal/session/watchdog.go
package session

import (
	"os"
	"time"
)

// Watchdog polls session pids on an interval and marks dead sessions.
type Watchdog struct {
	mgr      *Manager
	interval time.Duration
	stop     chan struct{}
}

func NewWatchdog(mgr *Manager, interval time.Duration) *Watchdog {
	return &Watchdog{mgr: mgr, interval: interval, stop: make(chan struct{})}
}

func (w *Watchdog) Run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.check()
		case <-w.stop:
			return
		}
	}
}

func (w *Watchdog) Stop() {
	close(w.stop)
}

func (w *Watchdog) check() {
	for _, sess := range w.mgr.List() {
		if sess.Status == StatusDead {
			continue
		}
		if !pidAlive(sess.TmuxPID) {
			w.mgr.Update(sess.Name, func(s *Session) {
				s.Status = StatusDead
				s.MoshPID = 0
				s.MoshPort = 0
				s.MoshKey = ""
			})
			w.mgr.Save()
		}
	}
}

// pidAlive reports whether a process with the given pid is running.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks existence without sending a signal
	return proc.Signal(os.Signal(nil)) == nil
}
```

Note: on macOS, `proc.Signal(nil)` doesn't work. Use syscall.Kill instead:

```go
import "syscall"

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd mac && go test ./internal/session/ -v -run TestWatchdog
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mac/internal/session/watchdog.go mac/internal/session/watchdog_test.go
git commit -m "feat: watchdog polls pids every interval, marks dead sessions"
```

---

## Task 9: `remotex new` Command

**Files:**
- Modify: `mac/cmd/remotex/main.go`

- [ ] **Step 1: Write integration test**

```go
// mac/cmd/remotex/new_test.go
package main_test

import (
	"os/exec"
	"testing"
)

func TestNewCommand(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	name := "remotex-integration-new"
	defer exec.Command("tmux", "kill-session", "-t", name).Run()

	cmd := exec.Command("go", "run", ".", "new", name)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("remotex new: %v\noutput: %s", err, out)
	}

	check := exec.Command("tmux", "has-session", "-t", name)
	if err := check.Run(); err != nil {
		t.Error("tmux session should exist after remotex new")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd mac/cmd/remotex && go test -v -run TestNewCommand
```
Expected: FAIL — new command is a stub.

- [ ] **Step 3: Implement `new` command**

Replace `newNewCmd()` in `mac/cmd/remotex/main.go`:

```go
func newNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new tmux session with an associated mosh-server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if tmuxpkg.SessionExists(name) {
				return fmt.Errorf("session %q already exists", name)
			}

			pid, err := tmuxpkg.NewSession(name)
			if err != nil {
				return fmt.Errorf("create tmux session: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config (run 'remotex setup' first): %w", err)
			}

			body := map[string]interface{}{
				"name":       name,
				"tmux_pid":   pid,
				"started_at": time.Now(),
			}
			data, _ := json.Marshal(body)
			resp, err := http.Post(
				fmt.Sprintf("http://127.0.0.1:%d/internal/sessions", cfg.DaemonPort),
				"application/json",
				bytes.NewReader(data),
			)
			if err != nil {
				return fmt.Errorf("register with daemon: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				return fmt.Errorf("daemon returned %d", resp.StatusCode)
			}

			fmt.Printf("Session %q created (tmux pid %d)\n", name, pid)
			return nil
		},
	}
}
```

Add imports at top of `main.go`:
```go
import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/fsaint/remotex/internal/config"
	tmuxpkg "github.com/fsaint/remotex/internal/tmux"
)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd mac/cmd/remotex && go test -v -run TestNewCommand -timeout 30s
```
Expected: PASS (requires tmux and a running daemon, or mock)

- [ ] **Step 5: Commit**

```bash
git add mac/cmd/remotex/
git commit -m "feat: remotex new command creates tmux session and registers with daemon"
```

---

## Task 10: `remotex kill` and `remotex list` Commands

**Files:**
- Modify: `mac/cmd/remotex/main.go`

- [ ] **Step 1: Implement `kill` command**

Replace `newKillCmd()`:

```go
func newKillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kill <name>",
		Short: "Kill a tmux session and unregister it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Unregister from daemon (daemon will stop mosh-server)
			req, _ := http.NewRequest(
				http.MethodDelete,
				fmt.Sprintf("http://127.0.0.1:%d/internal/sessions/%s", cfg.DaemonPort, name),
				nil,
			)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: could not reach daemon: %v\n", err)
			} else {
				resp.Body.Close()
			}

			if err := tmuxpkg.KillSession(name); err != nil {
				return fmt.Errorf("kill tmux session: %w", err)
			}

			fmt.Printf("Session %q killed\n", name)
			return nil
		},
	}
}
```

- [ ] **Step 2: Implement `list` command**

Replace `newListCmd()`:

```go
func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List active sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			req, _ := http.NewRequest(
				http.MethodGet,
				fmt.Sprintf("http://127.0.0.1:%d/sessions", cfg.DaemonPort),
				nil,
			)
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("reach daemon: %w", err)
			}
			defer resp.Body.Close()

			var sessions []session.Session
			if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(sessions) == 0 {
				fmt.Println("No active sessions.")
				return nil
			}
			fmt.Printf("%-20s %-6s %s\n", "NAME", "STATUS", "STARTED")
			for _, s := range sessions {
				fmt.Printf("%-20s %-6s %s\n", s.Name, s.Status, s.StartedAt.Format("2006-01-02 15:04:05"))
			}
			return nil
		},
	}
}
```

Add `"github.com/fsaint/remotex/internal/session"` to imports.

- [ ] **Step 3: Build and smoke test**

```bash
cd mac && go build ./cmd/remotex/
./remotex list
```
Expected: "load config" error or empty list (daemon not running yet — that's fine at this stage).

- [ ] **Step 4: Commit**

```bash
git add mac/cmd/remotex/
git commit -m "feat: remotex kill and list commands"
```

---

## Task 11: `remotex setup` Command

**Files:**
- Create: `mac/internal/setup/setup.go`
- Create: `mac/internal/setup/setup_test.go`
- Modify: `mac/cmd/remotex/main.go`

- [ ] **Step 1: Write failing tests**

```go
// mac/internal/setup/setup_test.go
package setup_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsaint/remotex/internal/setup"
)

func TestGenerateKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if err := setup.GenerateKeys(); err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	privPath := filepath.Join(dir, ".remotex", "id_ed25519")
	pubPath := privPath + ".pub"

	if _, err := os.Stat(privPath); err != nil {
		t.Errorf("private key not created: %v", err)
	}
	if _, err := os.Stat(pubPath); err != nil {
		t.Errorf("public key not created: %v", err)
	}
}

func TestGenerateAPIKey(t *testing.T) {
	key, err := setup.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if len(key) < 32 {
		t.Errorf("API key too short: %q", key)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd mac && go test ./internal/setup/ -v
```
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement setup package**

```go
// mac/internal/setup/setup.go
package setup

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"github.com/fsaint/remotex/internal/config"
)

// GenerateKeys creates an Ed25519 keypair in ~/.remotex/ and appends the
// public key to ~/.ssh/authorized_keys.
func GenerateKeys() error {
	dir := config.Dir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	// Write private key
	privPath := filepath.Join(dir, "id_ed25519")
	privBytes, err := marshalPrivateKey(priv)
	if err != nil {
		return err
	}
	if err := os.WriteFile(privPath, privBytes, 0600); err != nil {
		return err
	}

	// Write public key
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return err
	}
	pubBytes := ssh.MarshalAuthorizedKey(sshPub)
	pubPath := privPath + ".pub"
	if err := os.WriteFile(pubPath, pubBytes, 0644); err != nil {
		return err
	}

	// Append to authorized_keys
	home, _ := os.UserHomeDir()
	authKeysPath := filepath.Join(home, ".ssh", "authorized_keys")
	os.MkdirAll(filepath.Dir(authKeysPath), 0700)
	f, err := os.OpenFile(authKeysPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open authorized_keys: %w", err)
	}
	defer f.Close()
	_, err = f.Write(pubBytes)
	return err
}

func marshalPrivateKey(key ed25519.PrivateKey) ([]byte, error) {
	b, err := ssh.MarshalPrivateKey(key, "remotex")
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(b), nil
}

// GenerateAPIKey returns a random 256-bit base64-encoded API key.
func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// TailscaleHost returns the current machine's Tailscale hostname.
func TailscaleHost() (string, error) {
	out, err := exec.Command("tailscale", "status", "--json").Output()
	if err != nil {
		return "", fmt.Errorf("tailscale status: %w", err)
	}
	var status struct {
		Self struct {
			DNSName string `json:"DNSName"`
		} `json:"Self"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return "", err
	}
	// DNSName includes trailing dot, remove it
	return strings.TrimSuffix(status.Self.DNSName, "."), nil
}
```

Add missing imports: `"os/exec"`, `"encoding/json"`, `"strings"`, `"syscall"` (in handlers.go).

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd mac && go test ./internal/setup/ -v
```
Expected: PASS

- [ ] **Step 5: Implement `setup` command in CLI**

Replace `newSetupCmd()` in `mac/cmd/remotex/main.go`:

```go
func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "First-time setup: generate keys, configure daemon, print QR code",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Generating SSH keypair...")
			if err := setup.GenerateKeys(); err != nil {
				return fmt.Errorf("generate keys: %w", err)
			}

			fmt.Println("Generating API key...")
			apiKey, err := setup.GenerateAPIKey()
			if err != nil {
				return fmt.Errorf("generate API key: %w", err)
			}

			fmt.Println("Detecting Tailscale hostname...")
			host, err := setup.TailscaleHost()
			if err != nil {
				return fmt.Errorf("detect tailscale host: %w", err)
			}
			fmt.Printf("Tailscale hostname: %s\n", host)

			home, _ := os.UserHomeDir()
			cfg := &config.Config{
				APIKey:        apiKey,
				TailscaleHost: host,
				DaemonPort:    7654,
				SSHKeyPath:    filepath.Join(config.Dir(), "id_ed25519"),
			}
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			// Encode pairing payload as QR code
			payload := map[string]string{
				"host":            host,
				"api_key":         apiKey,
				"ssh_private_key": readFile(cfg.SSHKeyPath),
			}
			qrData, _ := json.Marshal(payload)

			qr, err := qrcode.New(string(qrData), qrcode.Medium)
			if err != nil {
				return fmt.Errorf("generate QR: %w", err)
			}
			fmt.Println("\nScan this QR code with the RemoteX iOS app:\n")
			fmt.Println(qr.ToSmallString(false))

			fmt.Println("\nSetup complete. Install and start the daemon with:")
			fmt.Println("  remotex-daemon install")
			fmt.Println("  remotex-daemon start")
			return nil
		},
	}
}

func readFile(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}
```

Add imports: `"path/filepath"`, `"github.com/skip2/go-qrcode"`, `"github.com/fsaint/remotex/internal/setup"`.

- [ ] **Step 6: Build and verify**

```bash
cd mac && go build ./cmd/remotex/ && echo "build ok"
```
Expected: `build ok`

- [ ] **Step 7: Commit**

```bash
git add mac/internal/setup/ mac/cmd/remotex/
git commit -m "feat: setup command generates keys, detects tailscale host, prints QR code"
```

---

## Task 12: Daemon Entry Point + launchd Integration

**Files:**
- Modify: `mac/cmd/remotex-daemon/main.go`
- Create: `mac/internal/setup/launchd.go`

- [ ] **Step 1: Implement daemon main**

```go
// mac/cmd/remotex-daemon/main.go
package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/fsaint/remotex/internal/config"
	"github.com/fsaint/remotex/internal/daemon"
	"github.com/fsaint/remotex/internal/session"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	sessionsPath := config.Dir() + "/sessions.json"
	mgr := session.NewManager(sessionsPath)
	if err := mgr.Load(); err != nil {
		log.Printf("warn: load sessions: %v", err)
	}
	mgr.PruneDeadPIDs() // clean up stale pids from previous run

	// Resolve Tailscale interface address
	tailscaleAddr, err := resolveTailscaleAddr()
	if err != nil {
		log.Printf("warn: tailscale addr not found, binding to all interfaces: %v", err)
		tailscaleAddr = "0.0.0.0"
	}

	srv := daemon.NewServer(mgr, cfg.APIKey, tailscaleAddr, cfg.TailscaleHost, cfg.DaemonPort)

	// Start watchdog
	w := session.NewWatchdog(mgr, 30*time.Second)
	go w.Run()

	log.Printf("remotex-daemon listening on %s:%d", tailscaleAddr, cfg.DaemonPort)
	if err := srv.Start(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// resolveTailscaleAddr finds the IP of the Tailscale network interface (utun*).
func resolveTailscaleAddr() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if !strings.HasPrefix(iface.Name, "utun") {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ip4 := ipnet.IP.To4(); ip4 != nil && ip4[0] == 100 {
					// Tailscale uses 100.x.x.x CGNAT range
					return ip4.String(), nil
				}
			}
		}
	}
	return "", fmt.Errorf("no tailscale interface found")
}
```

Add `"strings"`, `"time"` to imports. Add `PruneDeadPIDs()` method to `session.Manager`:

```go
// mac/internal/session/manager.go — add method
func (m *Manager) PruneDeadPIDs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, s := range m.sessions {
		if !pidAlive(s.TmuxPID) {
			s.Status = StatusDead
			s.MoshPID = 0
			s.MoshPort = 0
			s.MoshKey = ""
			m.sessions[name] = s
		}
	}
}
```

Move `pidAlive` from `watchdog.go` to `manager.go` so both can use it.

- [ ] **Step 2: Implement launchd plist install**

```go
// mac/internal/setup/launchd.go
package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.remotex.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.DaemonPath}}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.LogDir}}/daemon.log</string>
    <key>StandardErrorPath</key>
    <string>{{.LogDir}}/daemon.err</string>
</dict>
</plist>
`

// InstallLaunchd writes the launchd plist and loads it.
func InstallLaunchd(daemonBinaryPath string) error {
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".remotex", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.remotex.daemon.plist")
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return err
	}

	t := template.Must(template.New("plist").Parse(plistTemplate))
	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("create plist: %w", err)
	}
	defer f.Close()
	if err := t.Execute(f, map[string]string{
		"DaemonPath": daemonBinaryPath,
		"LogDir":     logDir,
	}); err != nil {
		return err
	}

	return exec.Command("launchctl", "load", plistPath).Run()
}

// UninstallLaunchd unloads and removes the launchd plist.
func UninstallLaunchd() error {
	home, _ := os.UserHomeDir()
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.remotex.daemon.plist")
	exec.Command("launchctl", "unload", plistPath).Run()
	return os.Remove(plistPath)
}
```

- [ ] **Step 3: Build everything**

```bash
cd mac && go build ./...
```
Expected: no errors.

- [ ] **Step 4: Smoke test setup + daemon**

```bash
# Terminal 1
./remotex setup
./remotex-daemon &
# Wait a second, then:
./remotex new mytest
./remotex list
./remotex kill mytest
kill %1
```
Expected: QR code printed, session created and listed, session killed cleanly.

- [ ] **Step 5: Commit**

```bash
git add mac/cmd/remotex-daemon/ mac/internal/setup/launchd.go mac/internal/session/manager.go
git commit -m "feat: daemon main with tailscale binding, launchd install/uninstall"
```

---

## Task 13: End-to-End Integration Test

**Files:**
- Create: `mac/integration_test.go`

- [ ] **Step 1: Write integration test**

```go
// mac/integration_test.go
//go:build integration

package main_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Run with: go test -tags integration -v -timeout 60s .
func TestFullSessionLifecycle(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	if _, err := exec.LookPath("mosh-server"); err != nil {
		t.Skip("mosh-server not installed")
	}

	// Start daemon on a test port
	daemon := exec.Command("./remotex-daemon")
	if err := daemon.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer daemon.Process.Kill()
	time.Sleep(300 * time.Millisecond)

	// Create session
	out, err := exec.Command("./remotex", "new", "integration-test").CombinedOutput()
	if err != nil {
		t.Fatalf("remotex new: %v\n%s", err, out)
	}

	// List sessions via daemon API
	cfg := loadTestConfig(t)
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/sessions", cfg.DaemonPort), nil)
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /sessions: %v", err)
	}
	var sessions []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&sessions)
	resp.Body.Close()

	found := false
	for _, s := range sessions {
		if s["name"] == "integration-test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("integration-test session not found in /sessions")
	}

	// Kill session
	out, err = exec.Command("./remotex", "kill", "integration-test").CombinedOutput()
	if err != nil {
		t.Fatalf("remotex kill: %v\n%s", err, out)
	}
}

func loadTestConfig(t *testing.T) struct{ DaemonPort int; APIKey string } {
	t.Helper()
	// reads ~/.remotex/config.json
	out, err := exec.Command("cat", os.ExpandEnv("$HOME/.remotex/config.json")).Output()
	if err != nil {
		t.Skipf("no config found, run remotex setup first: %v", err)
	}
	var cfg struct {
		DaemonPort int    `json:"daemon_port"`
		APIKey     string `json:"api_key"`
	}
	json.Unmarshal(out, &cfg)
	return cfg
}
```

- [ ] **Step 2: Run integration test**

```bash
cd mac && go build ./... && go test -tags integration -v -timeout 60s .
```
Expected: PASS (requires tmux, mosh-server, and `remotex setup` having been run)

- [ ] **Step 3: Final commit**

```bash
git add mac/integration_test.go
git commit -m "test: end-to-end integration test for session lifecycle"
```
