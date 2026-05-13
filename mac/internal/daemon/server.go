package daemon

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/fsaint/remotex/internal/session"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server is the remotex daemon HTTP server.
type Server struct {
	mgr           *session.Manager
	apiKey        string
	host          string
	tailscaleHost string
	port          int
	httpSrv       *http.Server
	localSrv      *http.Server
	connectMu     sync.Mutex
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

// buildExternalRouter serves iOS-facing routes (API key required).
func (s *Server) buildExternalRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Group(func(r chi.Router) {
		r.Use(s.requireAPIKey)
		r.Get("/sessions", s.handleListSessions)
		r.Post("/sessions/{name}/connect", s.handleConnect)
	})
	return r
}

// buildInternalRouter serves CLI-facing routes (localhost only, no auth).
func (s *Server) buildInternalRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/internal/sessions", s.handleRegisterSession)
	r.Delete("/internal/sessions/{name}", s.handleUnregisterSession)
	return r
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	s.httpSrv = &http.Server{Addr: addr, Handler: s.buildExternalRouter()}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	// localhost listener for CLI internal routes only.
	localAddr := fmt.Sprintf("127.0.0.1:%d", s.port)
	if localAddr != addr {
		localLn, err := net.Listen("tcp", localAddr)
		if err != nil {
			log.Printf("warn: local listener: %v", err)
		} else {
			s.localSrv = &http.Server{Addr: localAddr, Handler: s.buildInternalRouter()}
			go s.localSrv.Serve(localLn)
		}
	}

	return s.httpSrv.Serve(ln)
}

func (s *Server) Stop() {
	if s.httpSrv != nil {
		s.httpSrv.Shutdown(context.Background())
	}
	if s.localSrv != nil {
		s.localSrv.Shutdown(context.Background())
	}
}

// ServeHTTP implements http.Handler for use in tests (external routes).
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.buildExternalRouter().ServeHTTP(w, r)
}

// ServeInternalHTTP dispatches through the internal (localhost-only) router, for use in tests.
func (s *Server) ServeInternalHTTP(w http.ResponseWriter, r *http.Request) {
	s.buildInternalRouter().ServeHTTP(w, r)
}

func (s *Server) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		want := "Bearer " + s.apiKey
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

