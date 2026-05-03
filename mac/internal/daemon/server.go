package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/fsaint/remotex/internal/session"
	"github.com/go-chi/chi/v5"
)

// Server is the remotex daemon HTTP server.
type Server struct {
	mgr           *session.Manager
	apiKey        string
	host          string
	tailscaleHost string
	port          int
	httpSrv       *http.Server
}

func NewServer(mgr *session.Manager, apiKey, host, tailscaleHost string, port int) *Server {
	return &Server{
		mgr:           mgr,
		apiKey:        apiKey,
		host:          host,
		tailscaleHost: tailscaleHost,
		port:          port,
	}
}

func (s *Server) buildRouter() http.Handler {
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Internal routes (CLI → daemon, localhost only)
	r.Post("/internal/sessions", s.handleRegisterSession)
	r.Delete("/internal/sessions/{name}", s.handleUnregisterSession)

	// External routes (iOS app → daemon, API key required)
	r.Group(func(r chi.Router) {
		r.Use(s.requireAPIKey)
		r.Get("/sessions", s.handleListSessions)
		r.Post("/sessions/{name}/connect", s.handleConnect)
	})

	return r
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	s.httpSrv = &http.Server{Addr: addr, Handler: s.buildRouter()}

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

// ServeHTTP implements http.Handler for use in tests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.buildRouter().ServeHTTP(w, r)
}

func (s *Server) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+s.apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

