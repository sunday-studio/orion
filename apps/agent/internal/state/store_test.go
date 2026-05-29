package state

import (
	"errors"
	"fmt"
	"os"
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

func TestStoreCreatesStateDatabaseWithPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != stateFileMode {
		t.Fatalf("state.db mode = %o, want %o", got, stateFileMode)
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

func TestStoreApplyReplacementTokenPreservesIdentityAndQueues(t *testing.T) {
	store := openTestStore(t)

	if err := store.UpdateRegistration("agent-1", "token-1", "http://core"); err != nil {
		t.Fatalf("UpdateRegistration() error = %v", err)
	}
	reason := "planned work"
	if err := store.SetMaintenanceMode(true, &reason); err != nil {
		t.Fatalf("SetMaintenanceMode() error = %v", err)
	}
	checkedAt := time.Now().UTC()
	if err := store.ReplaceMonitors([]config.InternalStateMonitor{
		{Name: "homepage", ID: "monitor-1", Status: "running", LastChecked: checkedAt},
	}); err != nil {
		t.Fatalf("ReplaceMonitors() error = %v", err)
	}
	if _, err := store.EnqueueReport(ReportSpoolKindSystem, "agent-1", "", "", map[string]string{"timestamp": "2026-05-27T20:00:00Z"}, errors.New("offline")); err != nil {
		t.Fatalf("EnqueueReport() error = %v", err)
	}

	if err := store.ApplyReplacementToken(" token-2 "); err != nil {
		t.Fatalf("ApplyReplacementToken() error = %v", err)
	}

	state, err := store.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !state.IsRegistered() || state.AgentID != "agent-1" || state.Token != "token-2" || state.CoreURL != "http://core" {
		t.Fatalf("registration = %+v, want preserved identity with replacement token", state)
	}
	if !state.MaintenanceMode || state.MaintenanceReason == nil || *state.MaintenanceReason != reason {
		t.Fatalf("maintenance = %+v, want preserved", state)
	}
	monitor, err := store.GetMonitorByName("homepage")
	if err != nil {
		t.Fatalf("GetMonitorByName() error = %v", err)
	}
	if monitor == nil || monitor.ID != "monitor-1" || !monitor.LastChecked.Equal(checkedAt) {
		t.Fatalf("monitor = %+v, want preserved mapping", monitor)
	}
	if count, err := store.CountSpooledReports(); err != nil || count != 1 {
		t.Fatalf("CountSpooledReports() = %d, %v, want 1 nil", count, err)
	}
}

func TestInspectReadOnlyLoadsStateWithoutChangingPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := store.UpdateRegistration("agent-1", "token-1", "http://core"); err != nil {
		t.Fatalf("UpdateRegistration() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	state, err := InspectReadOnly(path)
	if err != nil {
		t.Fatalf("InspectReadOnly() error = %v", err)
	}
	if !state.IsRegistered() || state.AgentID != "agent-1" || state.Token != "token-1" {
		t.Fatalf("state = %+v, want persisted registration", state)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("state.db mode = %o, want unchanged 640", got)
	}
}

func TestInspectReadOnlyDoesNotCreateMissingStateDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-state.db")

	if _, err := InspectReadOnly(path); err == nil {
		t.Fatal("InspectReadOnly() error = nil, want missing database error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Stat() error = %v, want not exist", err)
	}
}

func TestStoreResetRegistrationKeepsMaintenance(t *testing.T) {
	store := openTestStore(t)

	if err := store.UpdateRegistration("agent-1", "token-1", "http://old-core"); err != nil {
		t.Fatalf("UpdateRegistration() error = %v", err)
	}
	reason := "planned work"
	if err := store.SetMaintenanceMode(true, &reason); err != nil {
		t.Fatalf("SetMaintenanceMode() error = %v", err)
	}
	if err := store.ReplaceMonitors([]config.InternalStateMonitor{
		{Name: "homepage", ID: "monitor-1", Status: "running", LastChecked: time.Now().UTC()},
	}); err != nil {
		t.Fatalf("ReplaceMonitors() error = %v", err)
	}
	if _, err := store.EnqueueReport(ReportSpoolKindSystem, "agent-1", "", "", map[string]string{"timestamp": "2026-05-27T20:00:00Z"}, errors.New("offline")); err != nil {
		t.Fatalf("EnqueueReport() error = %v", err)
	}

	if err := store.ResetRegistration(); err != nil {
		t.Fatalf("ResetRegistration() error = %v", err)
	}

	state, err := store.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if state.IsRegistered() || state.AgentID != "" || state.Token != "" || state.CoreURL != "" {
		t.Fatalf("registration = %+v, want cleared", state)
	}
	if !state.MaintenanceMode || state.MaintenanceReason == nil || *state.MaintenanceReason != reason {
		t.Fatalf("maintenance = %+v, want preserved", state)
	}
	if len(state.Monitors) != 0 {
		t.Fatalf("monitors = %d, want reset", len(state.Monitors))
	}
	if count, err := store.CountSpooledReports(); err != nil || count != 0 {
		t.Fatalf("CountSpooledReports() = %d, %v, want 0 nil after registration reset", count, err)
	}
}

func TestStoreApplyReplacementTokenPreservesIdentityMonitorsAndSpool(t *testing.T) {
	store := openTestStore(t)

	if err := store.UpdateRegistration("agent-1", "token-old", "http://core"); err != nil {
		t.Fatalf("UpdateRegistration() error = %v", err)
	}
	reason := "planned work"
	if err := store.SetMaintenanceMode(true, &reason); err != nil {
		t.Fatalf("SetMaintenanceMode() error = %v", err)
	}
	lastChecked := time.Now().UTC()
	if err := store.ReplaceMonitors([]config.InternalStateMonitor{
		{Name: "homepage", ID: "monitor-1", Status: "running", LastChecked: lastChecked},
	}); err != nil {
		t.Fatalf("ReplaceMonitors() error = %v", err)
	}
	if _, err := store.EnqueueReport(ReportSpoolKindSystem, "agent-1", "", "", map[string]string{"timestamp": "2026-05-27T20:00:00Z"}, errors.New("offline")); err != nil {
		t.Fatalf("EnqueueReport() error = %v", err)
	}

	if err := store.ApplyReplacementToken(" token-new\n"); err != nil {
		t.Fatalf("ApplyReplacementToken() error = %v", err)
	}

	state, err := store.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !state.IsRegistered() || state.AgentID != "agent-1" || state.Token != "token-new" || state.CoreURL != "http://core" {
		t.Fatalf("registration = %+v, want preserved identity with new token", state)
	}
	if !state.MaintenanceMode || state.MaintenanceReason == nil || *state.MaintenanceReason != reason {
		t.Fatalf("maintenance = %+v, want preserved", state)
	}
	if len(state.Monitors) != 1 || state.Monitors[0].ID != "monitor-1" || state.Monitors[0].Name != "homepage" {
		t.Fatalf("monitors = %+v, want preserved mapping", state.Monitors)
	}
	if count, err := store.CountSpooledReports(); err != nil || count != 1 {
		t.Fatalf("CountSpooledReports() = %d, %v, want 1 nil", count, err)
	}
}

func TestStorePersistsReportSpoolAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := store.UpdateRegistration("agent-1", "token-1", "http://core"); err != nil {
		t.Fatalf("UpdateRegistration() error = %v", err)
	}

	payload := map[string]interface{}{
		"timestamp":      "2026-05-27T20:00:00Z",
		"uptime_seconds": 42,
	}
	if _, err := store.EnqueueReport(ReportSpoolKindSystem, "agent-1", "", "", payload, errors.New("connection refused")); err != nil {
		t.Fatalf("EnqueueReport() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen state: %v", err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	state, err := reopened.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if state.AgentID != "agent-1" || state.Token != "token-1" {
		t.Fatalf("registration = %+v, want unchanged identity", state)
	}
	items, err := reopened.ListDueReports(time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("ListDueReports() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("due reports = %d, want 1", len(items))
	}
	if items[0].Kind != ReportSpoolKindSystem || items[0].AgentID != "agent-1" || items[0].LastError == "" {
		t.Fatalf("spooled report = %+v, want persisted system report metadata", items[0])
	}
}

func TestStoreReportSpoolFailureBookkeeping(t *testing.T) {
	store := openTestStore(t)

	item, err := store.EnqueueReport(ReportSpoolKindMonitor, "agent-1", "monitor-1", "homepage", map[string]string{"health": "down"}, errors.New("timeout"))
	if err != nil {
		t.Fatalf("EnqueueReport() error = %v", err)
	}
	if err := store.MarkReportFailed(item.ID, fmt.Errorf("still offline")); err != nil {
		t.Fatalf("MarkReportFailed() error = %v", err)
	}

	due, err := store.ListDueReports(time.Now().UTC().Add(reportSpoolBaseBackoff-time.Second), 10)
	if err != nil {
		t.Fatalf("ListDueReports() before backoff error = %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("due reports before backoff = %d, want 0", len(due))
	}

	due, err = store.ListDueReports(time.Now().UTC().Add(reportSpoolBaseBackoff+time.Second), 10)
	if err != nil {
		t.Fatalf("ListDueReports() after backoff error = %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("due reports after backoff = %d, want 1", len(due))
	}
	if due[0].Attempts != 1 || due[0].LastError != "still offline" {
		t.Fatalf("retry metadata = %+v, want attempts and last error", due[0])
	}
	if count, err := store.CountSpooledReports(); err != nil || count != 1 {
		t.Fatalf("CountSpooledReports() = %d, %v, want 1 nil", count, err)
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
