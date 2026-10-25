package service

import (
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"testing"
	"time"
)

func TestRegisterMonitorPrefersLiveRowOverLowerIDSoftDeletedTwin(t *testing.T) {
	database := testkit.NewDB(t)
	a, _ := testkit.TwoNodes(t, database, time.Now().UTC())
	now := time.Now().UTC()
	// "a-deleted" sorts before "monitor-a" by id: the old lookup picked it and tried to revive it.
	database.Create(&db.Monitor{ID: "a-deleted", NodeID: a.Node.ID, Name: a.Monitor.Name, Type: "http", Lifecycle: "deleted", Health: "unknown", CreatedAt: now.Add(-time.Hour)})

	svc := NewMonitorService(database, utils.NewLogger())
	resp, err := svc.RegisterMonitor(&RegisterMonitorRequest{NodeID: a.Node.ID, Name: a.Monitor.Name, Type: "http"})
	if err != nil {
		t.Fatalf("RegisterMonitor() error = %v, want the live monitor returned", err)
	}
	if resp.MonitorID != a.Monitor.ID {
		t.Fatalf("monitor id = %s, want the live monitor %s", resp.MonitorID, a.Monitor.ID)
	}
	var deleted db.Monitor
	database.First(&deleted, "id = ?", "a-deleted")
	if deleted.Lifecycle != "deleted" {
		t.Fatal("the soft-deleted twin was revived")
	}
}

func TestRegisterMonitorRevivesNewestDeletedWhenNoLiveRowExists(t *testing.T) {
	database := testkit.NewDB(t)
	a, _ := testkit.TwoNodes(t, database, time.Now().UTC())
	now := time.Now().UTC()
	database.Model(&db.Monitor{}).Where("id = ?", a.Monitor.ID).Update("lifecycle", "deleted")
	database.Create(&db.Monitor{ID: "older-deleted", NodeID: a.Node.ID, Name: a.Monitor.Name, Type: "http", Lifecycle: "deleted", Health: "unknown", CreatedAt: now.Add(-48 * time.Hour)})

	resp, err := NewMonitorService(database, utils.NewLogger()).RegisterMonitor(&RegisterMonitorRequest{NodeID: a.Node.ID, Name: a.Monitor.Name, Type: "http"})
	if err != nil || resp.MonitorID != a.Monitor.ID {
		t.Fatalf("RegisterMonitor() = %+v, %v, want the newest deleted monitor %s revived", resp, err, a.Monitor.ID)
	}
	var active int64
	database.Model(&db.Monitor{}).Where("node_id = ? AND name = ? AND lifecycle <> ?", a.Node.ID, a.Monitor.Name, "deleted").Count(&active)
	if active != 1 {
		t.Fatalf("live monitors with this name = %d, want exactly 1", active)
	}
}
