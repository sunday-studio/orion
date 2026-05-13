package state

import (
	"path/filepath"
	"testing"
	"time"

	"orion/agent/internal/config"
)

func TestStoreCreatesDefaultState(t *testing.T) {
	store := openTestStore(t)

	state, err := store.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if state.IsRegistered() {
		t.Fatalf("IsRegistered() = true, want false")
	}
	if state.MaintenanceMode {
		t.Fatalf("MaintenanceMode = true, want false")
	}
	if len(state.Monitors) != 0 {
		t.Fatalf("monitors = %d, want 0", len(state.Monitors))
	}
}

func TestStorePersistsRegistrationMaintenanceAndMonitors(t *testing.T) {
	store := openTestStore(t)

	if err := store.UpdateRegistration("agent-1", "token-1", "http://core"); err != nil {
		t.Fatalf("UpdateRegistration() error = %v", err)
	}
	reason := "planned work"
	if err := store.SetMaintenanceMode(true, &reason); err != nil {
		t.Fatalf("SetMaintenanceMode() error = %v", err)
	}
	now := time.Now().UTC()
	if err := store.ReplaceMonitors([]config.InternalStateMonitor{
		{Name: "homepage", ID: "monitor-1", Status: "running", LastChecked: now},
	}); err != nil {
		t.Fatalf("ReplaceMonitors() error = %v", err)
	}

	state, err := store.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if !state.IsRegistered() || state.AgentID != "agent-1" || state.Token != "token-1" || state.CoreURL != "http://core" {
		t.Fatalf("state registration = %+v, want persisted registration", state)
	}
	if !state.MaintenanceMode || state.MaintenanceReason == nil || *state.MaintenanceReason != reason {
		t.Fatalf("maintenance = %+v, want enabled with reason", state)
	}
	monitor, err := store.GetMonitorByName("homepage")
	if err != nil {
		t.Fatalf("GetMonitorByName() error = %v", err)
	}
	if monitor == nil || monitor.ID != "monitor-1" {
		t.Fatalf("monitor = %+v, want persisted monitor", monitor)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return store
}
