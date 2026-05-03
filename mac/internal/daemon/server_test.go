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
