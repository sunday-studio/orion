package service

import (
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"testing"
	"time"

	"gorm.io/gorm"
)

func offlineFixture(t *testing.T) (*gorm.DB, *IncidentService, db.Node, time.Time) {
	t.Helper()
	database := testkit.NewDB(t)
	seen := time.Now().UTC().Add(-time.Hour)
	node := db.Node{ID: "node-bare", MachineId: "machine-bare", Name: "bare-host", OS: "linux", Arch: "amd64", Token: "t", TokenHash: "h", LastSeen: seen, ReportingIntervalSeconds: 60}
	if err := database.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	return database, NewIncidentService(database, utils.NewLogger(), &config.Config{AlertRecoveryNotifications: true}), node, seen
}

func TestSweepSilentNodesAlertsForAHostWithNoMonitorsAndResolvesWhenItReturns(t *testing.T) {
	database, svc, node, seen := offlineFixture(t)
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})

	if n, err := svc.SweepSilentNodes(seen.Add(2 * time.Minute)); err != nil || n != 0 {
		t.Fatalf("inside the stale window flagged %d (err %v), want 0", n, err)
	}
	n, err := svc.SweepSilentNodes(seen.Add(10 * time.Minute))
	if err != nil || n != 1 {
		t.Fatalf("silent node flagged %d (err %v), want 1", n, err)
	}
	monitorID := nodeOfflineMonitorID(node.ID)
	incidents := activeIncidents(t, database, monitorID)
	if len(incidents) != 1 {
		t.Fatalf("active incidents = %d, want 1", len(incidents))
	}
	var deliveries int64
	database.Model(&db.AlertDelivery{}).Where("incident_id = ? AND event_type = ?", incidents[0].ID, "incident_opened").Count(&deliveries)
	if deliveries != 1 {
		t.Fatalf("opened deliveries = %d, want 1", deliveries)
	}
	var monitor db.Monitor
	database.First(&monitor, "id = ?", monitorID)
	if monitor.Name != "Node connectivity: bare-host" || monitor.Type != "node-connectivity" {
		t.Fatalf("synthetic monitor = %+v", monitor)
	}

	// Sweeping again while it stays down neither duplicates the incident nor re-pages.
	if n, _ := svc.SweepSilentNodes(seen.Add(11 * time.Minute)); n != 0 {
		t.Fatalf("second sweep flagged %d, want 0", n)
	}
	// The ordinary monitor sweep does not double-count the synthetic monitor either.
	if _, err := svc.SweepSilentMonitors(seen.Add(12 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := activeIncidents(t, database, monitorID); len(got) != 1 {
		t.Fatalf("incidents after monitor sweep = %d, want still 1", len(got))
	}

	// The host reports again: the incident resolves and the monitor is parked.
	if err := svc.ResolveNodeOffline(node.ID); err != nil {
		t.Fatal(err)
	}
	if got := activeIncidents(t, database, monitorID); len(got) != 0 {
		t.Fatalf("active incidents after recovery = %d, want 0", len(got))
	}
	var recovered db.Monitor
	database.First(&recovered, "id = ?", monitorID)
	if recovered.Lifecycle != "disabled" {
		t.Fatalf("lifecycle = %q, want the monitor parked so it is never swept as silent", recovered.Lifecycle)
	}
	var resolved int64
	database.Model(&db.AlertDelivery{}).Where("event_type = ?", "incident_resolved").Count(&resolved)
	if resolved != 1 {
		t.Fatalf("recovery deliveries = %d, want 1", resolved)
	}
	if _, err := svc.SweepSilentMonitors(seen.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := activeIncidents(t, database, monitorID); len(got) != 0 {
		t.Fatal("the parked connectivity monitor was flagged by the monitor sweep")
	}

	// It can go offline again later: same monitor is re-activated and a new incident opens.
	database.Model(&db.Node{}).Where("id = ?", node.ID).Update("last_seen", seen)
	if n, err := svc.SweepSilentNodes(seen.Add(2 * time.Hour)); err != nil || n != 1 {
		t.Fatalf("second outage flagged %d (err %v), want 1", n, err)
	}
	if got := activeIncidents(t, database, monitorID); len(got) != 1 {
		t.Fatalf("second outage incidents = %d, want 1", len(got))
	}
}

func TestSweepSilentNodesSkipsExpectedQuietNodes(t *testing.T) {
	t.Run("node with its own monitors is left to the monitor sweep", func(t *testing.T) {
		database, svc, node, seen := offlineFixture(t)
		database.Create(&db.Monitor{ID: "m1", NodeID: node.ID, Name: "web", Type: "http", Lifecycle: "active", Health: "up", ReportingIntervalSeconds: 60, CreatedAt: seen})
		if n, _ := svc.SweepSilentNodes(seen.Add(time.Hour)); n != 0 {
			t.Fatalf("flagged %d, want 0: its monitors already cover it", n)
		}
	})
	t.Run("revoked token", func(t *testing.T) {
		database, svc, node, seen := offlineFixture(t)
		revoked := time.Now().UTC()
		database.Model(&db.Node{}).Where("id = ?", node.ID).Update("token_revoked_at", &revoked)
		if n, _ := svc.SweepSilentNodes(seen.Add(time.Hour)); n != 0 {
			t.Fatalf("flagged %d, want 0: a revoked node is expected to be silent", n)
		}
	})
	t.Run("maintenance", func(t *testing.T) {
		database, svc, node, _ := offlineFixture(t)
		until := time.Now().UTC().Add(time.Hour)
		database.Model(&db.Node{}).Where("id = ?", node.ID).Updates(map[string]any{"maintenance_mode": true, "maintenance_until": &until})
		if n, _ := svc.SweepSilentNodes(time.Now().UTC()); n != 0 {
			t.Fatalf("flagged %d, want 0 during maintenance", n)
		}
	})
	t.Run("core owner node", func(t *testing.T) {
		database, svc, _, seen := offlineFixture(t)
		database.Create(&db.Node{ID: "core", MachineId: coreOwnerMachineID, Name: "Orion Core", OS: "core", Arch: "internal", Token: "ct", LastSeen: seen, ReportingIntervalSeconds: 60})
		n, _ := svc.SweepSilentNodes(seen.Add(time.Hour))
		var coreIncidents int64
		database.Model(&db.Incident{}).Where("node_id = ?", "core").Count(&coreIncidents)
		if coreIncidents != 0 {
			t.Fatalf("Core's own owner node was flagged offline (n=%d)", n)
		}
	})
}

func TestResolveNodeOfflineIsANoopForHealthyNodes(t *testing.T) {
	_, svc, node, _ := offlineFixture(t)
	if err := svc.ResolveNodeOffline(node.ID); err != nil {
		t.Fatalf("ResolveNodeOffline() for a node that was never offline = %v, want nil", err)
	}
}
