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

func TestManagerUpdate(t *testing.T) {
	dir := t.TempDir()
	m := session.NewManager(filepath.Join(dir, "sessions.json"))
	m.Add(&session.Session{Name: "work", TmuxPID: 1, Status: session.StatusLive})

	updated := m.Update("work", func(s *session.Session) {
		s.MoshPID = 9999
		s.MoshPort = 60001
		s.MoshKey = "abc123"
	})
	if !updated {
		t.Error("Update should return true for existing session")
	}

	got, _ := m.Get("work")
	if got.MoshPID != 9999 {
		t.Errorf("MoshPID: got %d want 9999", got.MoshPID)
	}

	notUpdated := m.Update("nonexistent", func(s *session.Session) {})
	if notUpdated {
		t.Error("Update should return false for missing session")
	}
}
