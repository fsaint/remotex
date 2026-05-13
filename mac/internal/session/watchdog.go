package session

import (
	"sync"
	"time"
)

// Watchdog polls session pids on an interval and marks dead sessions.
type Watchdog struct {
	mgr      *Manager
	interval time.Duration
	stop     chan struct{}
	once     sync.Once
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
	w.once.Do(func() { close(w.stop) })
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
