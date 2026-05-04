package session

import "time"

const (
	StatusLive = "live"
	StatusDead = "dead"
)

// Session represents one tmux session and its optional mosh-server.
type Session struct {
	Name      string    `json:"name"`
	TmuxPID   int       `json:"tmux_pid"`
	MoshPID   int       `json:"mosh_pid,omitempty"`
	MoshPort  int       `json:"mosh_port,omitempty"`
	MoshKey   string    `json:"mosh_key,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"`
}
