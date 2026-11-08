package selfupdate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Marker is written just before the new binary is swapped in and removed once the new version has
// stayed up. It lets the next start tell "the update worked" from "the new version cannot start".
type Marker struct {
	RequestID string    `json:"request_id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	At        time.Time `json:"at"`
	// Starts counts how often the new version began running without being confirmed.
	Starts int `json:"starts"`
	// State is "swapped" until the new version is confirmed, then "rolled_back" after a restore.
	State string `json:"state"`
	// Confirmed is set once the new version has stayed up. A confirmed update that still waits to be
	// reported to Core never counts as a failed start.
	Confirmed bool `json:"confirmed,omitempty"`
	// Reason is why the update was rolled back.
	Reason string `json:"reason,omitempty"`
}

const (
	StateSwapped    = "swapped"
	StateRolledBack = "rolled_back"
	markerName      = "update-pending.json"
)

func (e *Env) markerPath() string { return filepath.Join(e.StateDir, markerName) }

// ReadMarker returns the pending marker, or nil when no update is in flight.
func (e *Env) ReadMarker() (*Marker, error) {
	data, err := os.ReadFile(e.markerPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m Marker
	if err := json.Unmarshal(data, &m); err != nil {
		// A marker nobody can read cannot be acted on; do not keep tripping over it.
		_ = os.Remove(e.markerPath())
		return nil, nil
	}
	return &m, nil
}

func (e *Env) writeMarker(m Marker) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := e.markerPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, e.markerPath())
}

// ClearMarker removes the marker once its result has been reported.
func (e *Env) ClearMarker() error {
	if err := os.Remove(e.markerPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
