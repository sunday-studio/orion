// Package control implements the local control socket shared by the daemon
// (server) and the CLI (client). It speaks JSON over HTTP on a unix socket so
// unprivileged operators in the orion group can inspect and steer the daemon
// without sudo. The agent token is never exposed through this API.
package control

import (
	"os"
	"runtime"
	"time"
)

// SocketEnv overrides the default socket path for both daemon and CLI.
const SocketEnv = "ORION_AGENT_SOCKET"

// Group is the OS group granted access to the socket.
const Group = "orion"

// DefaultSocketPath returns the platform default socket location.
func DefaultSocketPath() string {
	if v := os.Getenv(SocketEnv); v != "" {
		return v
	}
	switch runtime.GOOS {
	case "linux":
		return "/run/orion/agent.sock"
	case "darwin":
		return "/usr/local/var/run/orion/agent.sock"
	default:
		return "agent.sock"
	}
}

// Status is the daemon's own view of itself.
type Status struct {
	Version           string    `json:"version"`
	PID               int       `json:"pid"`
	StartedAt         time.Time `json:"started_at"`
	UptimeSeconds     int64     `json:"uptime_seconds"`
	Registered        bool      `json:"registered"`
	AgentID           string    `json:"agent_id,omitempty"`
	CoreURL           string    `json:"core_url,omitempty"`
	LastSync          time.Time `json:"last_sync"`
	MaintenanceMode   bool      `json:"maintenance_mode"`
	MaintenanceReason string    `json:"maintenance_reason,omitempty"`
	Monitors          int       `json:"monitors"`
	QueuePending      int       `json:"queue_pending"`
	ConfigPath        string    `json:"config_path,omitempty"`
}

// MaintenanceRequest toggles maintenance mode.
type MaintenanceRequest struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason,omitempty"`
	// DurationSeconds bounds maintenance; 0 uses the agent default.
	DurationSeconds int64 `json:"duration_seconds,omitempty"`
}

// TokenRequest carries a replacement agent token.
type TokenRequest struct {
	Token string `json:"token"`
}

// Error is the JSON body of every non-2xx response.
type Error struct {
	Message string `json:"error"`
}

// Handler is implemented by the daemon to serve control requests.
type Handler interface {
	Status() (Status, error)
	SetMaintenance(req MaintenanceRequest) error
	ApplyToken(token string) error
	// LogPath is the daemon's JSONL log file; empty if file logging is off.
	LogPath() string
	Reload() error
	// TokenStatus asks Core, with the agent token, for a cheap authenticated
	// resource and returns the HTTP status (the token itself never leaves the daemon).
	TokenStatus() (int, error)
}

// TokenCheck is the result of a daemon-side token probe.
type TokenCheck struct {
	HTTPStatus int `json:"http_status"`
}
