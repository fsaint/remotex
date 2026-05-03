package session_test

import (
	"os"
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

func TestWatchdogKeepsLiveSessions(t *testing.T) {
	dir := t.TempDir()
	mgr := session.NewManager(filepath.Join(dir, "sessions.json"))

	// Use current process pid (guaranteed to exist)
	mgr.Add(&session.Session{
		Name:      "live-session",
		TmuxPID:   os.Getpid(),
		StartedAt: time.Now(),
		Status:    session.StatusLive,
	})

	w := session.NewWatchdog(mgr, 50*time.Millisecond)
	go w.Run()
	defer w.Stop()

	time.Sleep(200 * time.Millisecond)

	sess, ok := mgr.Get("live-session")
	if !ok {
		t.Fatal("session should still be in registry")
	}
	if sess.Status != session.StatusLive {
		t.Errorf("live session should stay live, got %q", sess.Status)
	}
}
