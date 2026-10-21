package db

import (
	"os"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestActiveMonitorUniquenessMigrationDedupesAndEnforces(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	database.Exec("DROP INDEX idx_monitors_node_name_active")
	base := time.Now().UTC()
	mk := func(id, node, name, lifecycle string, age int) {
		if err := database.Create(&Monitor{ID: id, NodeID: node, Name: name, Type: "http", Lifecycle: lifecycle, Health: "up", CreatedAt: base.Add(time.Duration(age) * time.Minute)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	mk("m-first", "a1", "site", "active", 0)
	mk("m-dup", "a1", "site", "active", 1)
	mk("m-other-node", "a2", "site", "active", 2)
	mk("m-deleted", "a1", "site", "deleted", 3)
	database.Create(&AlertDelivery{ID: "d1", IncidentID: "i", EventType: "e", Channel: "c", Type: "webhook", Status: "sent"})
	database.Exec("PRAGMA foreign_keys=OFF")
	database.Exec("INSERT INTO alert_delivery_attempts (id, alert_delivery_id, attempt_number, status, stage, started_at) VALUES ('orphan','gone',1,'sent','transport',CURRENT_TIMESTAMP)")

	sql, _ := os.ReadFile("migrations/000033_indexes_and_active_monitor_uniqueness.up.sql")
	if err := database.Exec(string(sql)).Error; err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	var first, dup, other Monitor
	database.First(&first, "id = ?", "m-first")
	database.First(&dup, "id = ?", "m-dup")
	database.First(&other, "id = ?", "m-other-node")
	if first.Lifecycle != "active" || dup.Lifecycle != "deleted" || other.Lifecycle != "active" {
		t.Fatalf("lifecycles = %s/%s/%s, want oldest kept, duplicate deleted, other node untouched", first.Lifecycle, dup.Lifecycle, other.Lifecycle)
	}
	var orphans int64
	database.Raw("SELECT COUNT(*) FROM alert_delivery_attempts WHERE id = 'orphan'").Scan(&orphans)
	if orphans != 0 {
		t.Fatal("orphaned delivery attempt survived the cleanup")
	}
	if err := database.Create(&Monitor{ID: "m-new-dup", NodeID: "a1", Name: "site", Type: "http", Lifecycle: "active", Health: "up"}).Error; err == nil {
		t.Fatal("a second active monitor with the same (node, name) was accepted")
	}
	if err := database.Create(&Monitor{ID: "m-new-ok", NodeID: "a1", Name: "site", Type: "http", Lifecycle: "deleted", Health: "up"}).Error; err != nil {
		t.Fatalf("a soft-deleted twin must be allowed: %v", err)
	}
}

func TestInitializeEnforcesDeclaredForeignKeys(t *testing.T) {
	database, err := Initialize(t.TempDir(), "silent")
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	var on int
	database.Raw("PRAGMA foreign_keys").Scan(&on)
	if on != 1 {
		t.Fatalf("PRAGMA foreign_keys = %d, want 1", on)
	}
	database.Create(&AlertDelivery{ID: "d1", IncidentID: "i", EventType: "e", Channel: "c", Type: "webhook", Status: "sent"})
	database.Create(&AlertDeliveryAttempt{ID: "a1", AlertDeliveryID: "d1", AttemptNumber: 1, Status: "sent", Stage: "transport", StartedAt: time.Now().UTC()})
	if err := database.Exec("DELETE FROM alert_deliveries WHERE id = 'd1'").Error; err == nil {
		t.Fatal("deleting a delivery that still has attempts succeeded, want a foreign key violation")
	}
	if err := database.Create(&AlertDeliveryAttempt{ID: "a2", AlertDeliveryID: "missing", AttemptNumber: 1, Status: "sent", Stage: "transport", StartedAt: time.Now().UTC()}).Error; err == nil {
		t.Fatal("inserting an attempt for a missing delivery succeeded, want a foreign key violation")
	}
}
