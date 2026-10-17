package db

import (
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUniqueActiveIncidentMigrationDedupesExistingRows(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	// Recreate the pre-migration world: no unique index, duplicate active incidents.
	if err := database.Exec("DROP INDEX idx_incidents_one_active_per_monitor").Error; err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	for i, id := range []string{"old", "mid", "new"} {
		inc := Incident{ID: id, Status: "open", Severity: "high", Title: "t", AgentID: "a", MonitorID: "m1", OpenedAt: base.Add(time.Duration(i) * time.Minute), LastEventAt: base, NotificationStatus: "pending"}
		if err := database.Create(&inc).Error; err != nil {
			t.Fatal(err)
		}
	}
	other := Incident{ID: "other", Status: "open", Severity: "high", Title: "t", AgentID: "a", MonitorID: "m2", OpenedAt: base, LastEventAt: base, NotificationStatus: "pending"}
	database.Create(&other)

	sql, err := os.ReadFile("migrations/000028_unique_active_incident.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Exec(string(sql)).Error; err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	var active []Incident
	database.Where("monitor_id = ? AND status = ?", "m1", "open").Find(&active)
	if len(active) != 1 || active[0].ID != "new" {
		t.Fatalf("active incidents for m1 = %+v, want only the newest", active)
	}
	var dup Incident
	database.First(&dup, "id = ?", "old")
	if dup.Status != "resolved" || dup.ResolutionKind != "deduplicated" {
		t.Fatalf("old duplicate = status %q kind %q, want resolved/deduplicated", dup.Status, dup.ResolutionKind)
	}
	var untouched Incident
	database.First(&untouched, "id = ?", "other")
	if untouched.Status != "open" {
		t.Fatalf("other monitor's incident status = %q, want open", untouched.Status)
	}
}

func TestMaintenanceMigrationGivesLegacyMaintenanceAnEnd(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	database.Create(&Agent{ID: "legacy", MachineId: "m1", Name: "legacy", OS: "linux", Arch: "amd64", Token: "t1", MaintenanceMode: true})
	database.Create(&Agent{ID: "idle", MachineId: "m2", Name: "idle", OS: "linux", Arch: "amd64", Token: "t2"})
	sql, _ := os.ReadFile("migrations/000034_agent_maintenance_until.up.sql")
	// Re-run only the backfill UPDATE (the ALTER already ran with Migrate).
	update := string(sql)[strings.Index(string(sql), "UPDATE agents"):]
	if err := database.Exec(update).Error; err != nil {
		t.Fatal(err)
	}
	var legacy, idle Agent
	database.First(&legacy, "id = ?", "legacy")
	database.First(&idle, "id = ?", "idle")
	if legacy.MaintenanceUntil == nil || time.Until(*legacy.MaintenanceUntil) < 23*time.Hour {
		t.Fatalf("legacy maintenance end = %v, want about 24h from now", legacy.MaintenanceUntil)
	}
	if idle.MaintenanceUntil != nil {
		t.Fatal("agents not in maintenance must be untouched")
	}
}
