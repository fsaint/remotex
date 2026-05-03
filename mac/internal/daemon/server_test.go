package daemon_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/fsaint/remotex/internal/daemon"
	"github.com/fsaint/remotex/internal/session"
)

func newTestServer(t *testing.T) (*daemon.Server, *session.Manager) {
	t.Helper()
	mgr := session.NewManager(t.TempDir() + "/sessions.json")
	srv := daemon.NewServer(mgr, "test-key", "127.0.0.1", "mac.ts.net", 0)
	return srv, mgr
}

func TestServerStartStop(t *testing.T) {
	mgr := session.NewManager(t.TempDir() + "/sessions.json")
	srv := daemon.NewServer(mgr, "test-api-key", "127.0.0.1", "mac.tailnet.ts.net", 19999)

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

func TestHandleRegisterSession(t *testing.T) {
	srv, mgr := newTestServer(t)

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

func TestHandleRegisterSessionBadRequest(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/internal/sessions", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d want 400", w.Code)
	}
}

func TestHandleListSessions(t *testing.T) {
	srv, mgr := newTestServer(t)
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
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d want 401", w.Code)
	}
}

func TestHandleUnregisterSession(t *testing.T) {
	srv, mgr := newTestServer(t)
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

func TestHandleUnregisterNotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/internal/sessions/nonexistent", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d want 404", w.Code)
	}
}
