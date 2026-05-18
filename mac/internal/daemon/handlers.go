package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/fsaint/remotex/internal/mosh"
	"github.com/fsaint/remotex/internal/session"
)

var validSessionName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type registerRequest struct {
	Name      string    `json:"name"`
	TmuxPID   int       `json:"tmux_pid"`
	StartedAt time.Time `json:"started_at"`
}

// sessionListItem is the API DTO for GET /sessions — omits the live mosh_key.
type sessionListItem struct {
	Name      string    `json:"name"`
	TmuxPID   int       `json:"tmux_pid"`
	MoshPort  int       `json:"mosh_port,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"`
}

func (s *Server) handleRegisterSession(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" || !validSessionName.MatchString(req.Name) {
		http.Error(w, "name must match ^[A-Za-z0-9._-]{1,64}$", http.StatusBadRequest)
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
	s.mgr.Save() // best-effort; in-memory state is already correct
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.mgr.List()
	items := make([]sessionListItem, len(sessions))
	for i, sess := range sessions {
		items[i] = sessionListItem{
			Name:      sess.Name,
			TmuxPID:   sess.TmuxPID,
			MoshPort:  sess.MoshPort,
			StartedAt: sess.StartedAt,
			Status:    sess.Status,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
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
	if sess.Status == session.StatusDead {
		http.Error(w, "session is dead", http.StatusGone)
		return
	}

	// Kill any existing mosh-server so we always get a fresh UDP binding.
	// tmux provides session persistence; a fresh mosh-server avoids stale state.
	if sess.MoshPID > 0 {
		mosh.Stop(sess.MoshPID)
	}
	s.mgr.Update(name, func(sess *session.Session) {
		sess.MoshPID = 0
		sess.MoshPort = 0
		sess.MoshKey = ""
	})

	// Spawn a new mosh-server attached to the tmux session, bound to the Tailscale interface.
	info, err := mosh.Start(s.host, []string{"tmux", "attach-session", "-t", name})
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
		Host: s.host,
		Port: info.Port,
		Key:  info.Key,
	})
}
