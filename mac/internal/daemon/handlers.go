package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"syscall"
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
		http.Error(w, "failed to persist session", http.StatusInternalServerError)
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
	if sess.MoshPID > 0 {
		mosh.Stop(sess.MoshPID)
	}
	s.mgr.Remove(name)
	if err := s.mgr.Save(); err != nil {
		http.Error(w, "failed to persist session", http.StatusInternalServerError)
		return
	}
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

	s.connectMu.Lock()
	defer s.connectMu.Unlock()

	// Re-fetch after acquiring lock (state may have changed)
	sess, ok = s.mgr.Get(name)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
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
	if err := s.mgr.Save(); err != nil {
		http.Error(w, "failed to save session state", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(connectResponse{
		Host: s.tailscaleHost,
		Port: info.Port,
		Key:  info.Key,
	})
}
