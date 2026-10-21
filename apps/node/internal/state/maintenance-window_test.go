package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMaintenanceWindowExpiresOnReadAndClearsWhenDisabled(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	reason := "kernel upgrade"

	until := time.Now().Add(time.Hour)
	if err := store.SetMaintenanceWindow(true, &reason, &until); err != nil {
		t.Fatal(err)
	}
	got, _ := store.Get()
	if !got.MaintenanceMode || got.MaintenanceUntil == nil || got.MaintenanceReason == nil || *got.MaintenanceReason != reason {
		t.Fatalf("state = %+v, want active maintenance with end time and reason", got)
	}

	past := time.Now().Add(-time.Second)
	if err := store.SetMaintenanceWindow(true, &reason, &past); err != nil {
		t.Fatal(err)
	}
	got, _ = store.Get()
	if got.MaintenanceMode {
		t.Fatal("an ended maintenance window still reads as active; the node would stay silent after Core resumed alerting")
	}

	if err := store.SetMaintenanceWindow(true, &reason, &until); err != nil {
		t.Fatal(err)
	}
	if err := store.SetMaintenanceWindow(false, nil, &until); err != nil {
		t.Fatal(err)
	}
	got, _ = store.Get()
	if got.MaintenanceMode || got.MaintenanceUntil != nil {
		t.Fatalf("state = %+v, want maintenance and end time cleared", got)
	}

	// Older callers without an end time keep their behaviour.
	if err := store.SetMaintenanceMode(true, nil); err != nil {
		t.Fatal(err)
	}
	got, _ = store.Get()
	if !got.MaintenanceMode || got.MaintenanceUntil != nil {
		t.Fatalf("state = %+v, want open-ended maintenance from SetMaintenanceMode", got)
	}
}
