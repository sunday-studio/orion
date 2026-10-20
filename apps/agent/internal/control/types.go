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
	Version           string         `json:"version"`
	PID               int            `json:"pid"`
	StartedAt         time.Time      `json:"started_at"`
	UptimeSeconds     int64          `json:"uptime_seconds"`
	Registered        bool           `json:"registered"`
	AgentID           string         `json:"agent_id,omitempty"`
	CoreURL           string         `json:"core_url,omitempty"`
	LastSync          time.Time      `json:"last_sync"`
	MaintenanceMode   bool           `json:"maintenance_mode"`
	MaintenanceReason string         `json:"maintenance_reason,omitempty"`
	MaintenanceUntil  *time.Time     `json:"maintenance_until,omitempty"`
	Monitors          int            `json:"monitors"`
	QueuePending      int            `json:"queue_pending"`
	Name              string         `json:"name,omitempty"`
	Mode              string         `json:"mode,omitempty"`
	LogSources        int            `json:"log_sources"`
	LastReportAt      time.Time      `json:"last_report_at"`
	MonitorStates     []MonitorState `json:"monitor_states,omitempty"`
	TopApps           []AppUsage     `json:"top_apps,omitempty"`
	// Suggestions are scan results nobody has accepted, dismissed or ignored;
	// NewItems are the ones that appeared since the last review.
	Suggestions int `json:"suggestions"`
	// MissingLogKeys counts configured log sources whose ingest key cannot be read.
	MissingLogKeys int `json:"missing_log_keys"`
	// Host is the latest live host reading; ReportMillis the send time of the
	// last reports (oldest first) and ReportIntervalSeconds how often they go out.
	Host                  *HostStat       `json:"host,omitempty"`
	ReportMillis          []int           `json:"report_millis,omitempty"`
	ReportIntervalSeconds int             `json:"report_interval_seconds"`
	Scanning              bool            `json:"scanning"`
	ThingsFound           int             `json:"things_found"`
	NewItems              []DiscoveryItem `json:"new_items,omitempty"`
	ConfigPath            string          `json:"config_path,omitempty"`
}

// MonitorState is the latest result of one monitor.
type MonitorState struct {
	Name   string    `json:"name"`
	Kind   string    `json:"kind"`
	Target string    `json:"target,omitempty"` // what it watches: "nginx", "port 6379", "disk /var"
	Health string    `json:"health"`           // up | down | slow | unknown
	At     time.Time `json:"at"`               // when it was last checked
	Since  time.Time `json:"since"`            // when it entered this health
	Detail string    `json:"detail,omitempty"`
	Value  string    `json:"value,omitempty"` // a short reading such as "91%"
}

// AppUsage is one of the busiest apps from the last passive sample.
type AppUsage struct {
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
}

// KindState is how scanning one kind went.
type KindState struct {
	Kind    string `json:"kind"`
	State   string `json:"state"` // scanned | off | blocked | absent
	Message string `json:"message,omitempty"`
	Fix     string `json:"fix,omitempty"`
	Why     string `json:"why,omitempty"`
}

// DiscoveryItem is one suggestion as shown to a person.
type DiscoveryItem struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	Detail      string    `json:"detail,omitempty"`
	Confidence  string    `json:"confidence"`
	Status      string    `json:"status"` // suggested | monitored
	ManagedIn   string    `json:"managed_in,omitempty"`
	New         bool      `json:"new,omitempty"`
	Dismissed   bool      `json:"dismissed,omitempty"`
	FirstSeen   time.Time `json:"first_seen"`
	DismissedAt time.Time `json:"dismissed_at"`
}

// Discovery is the daemon's latest scan and the choices made about it.
type Discovery struct {
	Enabled   bool            `json:"enabled"`
	Scanning  bool            `json:"scanning"`
	ScannedAt time.Time       `json:"scanned_at"`
	Kinds     []KindState     `json:"kinds"`
	Items     []DiscoveryItem `json:"items"`
	Ignore    []string        `json:"ignore,omitempty"`
}

// DiscoveryAction changes what is suggested: dismiss, restore or review.
type DiscoveryAction struct {
	Action string `json:"action"`
	ID     string `json:"id,omitempty"`
}

// HostStat is a live reading of this host.
type HostStat struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskPath      string  `json:"disk_path"`
	DiskPercent   float64 `json:"disk_percent"`
	Load1         float64 `json:"load_1"`
	NetRxBps      float64 `json:"net_rx_bps"`
	NetTxBps      float64 `json:"net_tx_bps"`
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
	// SpoolRecent returns the newest n shipped-log entries as NDJSON.
	SpoolRecent(n int) ([]byte, error)
	// Discovery returns the latest scan; Rescan starts one and waits for it.
	Discovery() (Discovery, error)
	Rescan() (Discovery, error)
	DiscoveryAction(a DiscoveryAction) error
}

// TokenCheck is the result of a daemon-side token probe.
type TokenCheck struct {
	HTTPStatus int `json:"http_status"`
}
