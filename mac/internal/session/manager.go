package session

import (
	"encoding/json"
	"os"
	"sync"
	"syscall"
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

// PruneDeadPIDs checks all sessions on startup and marks those with dead tmux pids as StatusDead.
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

// pidAlive reports whether a process with the given pid is running.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
