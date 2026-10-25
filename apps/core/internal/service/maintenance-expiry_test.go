package service

import (
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"testing"
	"time"
)

var nodeActor = NodeTokenActionInput{ActorType: "node", ActorID: "node-a", RequestID: "req-1"}

func TestNodeInMaintenanceHonoursExpiry(t *testing.T) {
	now := time.Now().UTC()
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	for name, tc := range map[string]struct {
		node db.Node
		want bool
	}{
		"off":                {db.Node{}, false},
		"on, future end":     {db.Node{MaintenanceMode: true, MaintenanceUntil: &future}, true},
		"on, ended":          {db.Node{MaintenanceMode: true, MaintenanceUntil: &past}, false},
		"on, legacy no end":  {db.Node{MaintenanceMode: true}, true},
		"off but stale time": {db.Node{MaintenanceUntil: &future}, false},
	} {
		if got := tc.node.InMaintenance(now); got != tc.want {
			t.Errorf("%s: InMaintenance = %t, want %t", name, got, tc.want)
		}
	}
}

func TestSetMaintenanceModeDefaultsToTwentyFourHoursAndIsAudited(t *testing.T) {
	database := testkit.NewDB(t)
	a, _ := testkit.TwoNodes(t, database, time.Now().UTC())
	svc := NewNodeService(database, utils.NewLogger())

	if err := svc.SetMaintenanceMode(a.Node.ID, true, 0, nodeActor); err != nil {
		t.Fatal(err)
	}
	var node db.Node
	database.First(&node, "id = ?", a.Node.ID)
	if !node.MaintenanceMode || node.MaintenanceUntil == nil {
		t.Fatalf("node = %+v, want maintenance with an end time", node)
	}
	if d := time.Until(*node.MaintenanceUntil); d < 23*time.Hour || d > 25*time.Hour {
		t.Fatalf("default maintenance ends in %v, want about 24h", d)
	}

	if err := svc.SetMaintenanceMode(a.Node.ID, true, 31*24*time.Hour, nodeActor); err != ErrInvalidMaintenanceDuration {
		t.Fatalf("31-day maintenance error = %v, want ErrInvalidMaintenanceDuration", err)
	}
	if err := svc.SetMaintenanceMode(a.Node.ID, false, 0, nodeActor); err != nil {
		t.Fatal(err)
	}
	var cleared db.Node
	database.First(&cleared, "id = ?", a.Node.ID)
	if cleared.MaintenanceMode || cleared.MaintenanceUntil != nil {
		t.Fatalf("node = %+v, want maintenance cleared", cleared)
	}

	var actions []string
	database.Model(&db.AuditEvent{}).Where("affected_object_id = ?", a.Node.ID).Order("created_at ASC").Pluck("action", &actions)
	if len(actions) != 2 || actions[0] != NodeMaintenanceAuditEnabled || actions[1] != NodeMaintenanceAuditDisabled {
		t.Fatalf("audit actions = %v, want enabled then disabled", actions)
	}
	if err := svc.SetMaintenanceMode("no-such-node", true, 0, nodeActor); err == nil {
		t.Fatal("maintenance on an unknown node = nil error")
	}
}

// D6: maintenance suppresses alerts; once it expires, alerting resumes.
func TestScenarioD6ExpiredMaintenanceResumesAlerting(t *testing.T) {
	database := testkit.NewDB(t)
	a, _ := testkit.TwoNodes(t, database, time.Now().UTC())
	cfg := &config.Config{}
	nodes := NewNodeService(database, utils.NewLogger())
	incidents := NewIncidentService(database, utils.NewLogger(), cfg)
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})

	if err := nodes.SetMaintenanceMode(a.Node.ID, true, time.Hour, nodeActor); err != nil {
		t.Fatal(err)
	}
	if err := incidents.ReconcileMonitorReport(a.Monitor.ID, "r1", downPayload()); err != nil {
		t.Fatal(err)
	}
	var count int64
	database.Model(&db.Incident{}).Where("monitor_id = ?", a.Monitor.ID).Count(&count)
	if count != 0 {
		t.Fatalf("incidents during maintenance = %d, want 0", count)
	}

	// The window ends. Even before the sweeper runs, the end time is honoured...
	past := time.Now().UTC().Add(-time.Minute)
	database.Model(&db.Node{}).Where("id = ?", a.Node.ID).Update("maintenance_until", &past)
	if err := incidents.ReconcileMonitorReport(a.Monitor.ID, "r2", downPayload()); err != nil {
		t.Fatal(err)
	}
	database.Model(&db.Incident{}).Where("monitor_id = ?", a.Monitor.ID).Count(&count)
	if count != 1 {
		t.Fatalf("incidents after maintenance ended = %d, want 1 (alerting resumed)", count)
	}

	// ...and the sweeper clears the flag and writes the audit trail.
	ended, err := nodes.ExpireMaintenance(time.Now().UTC())
	if err != nil || ended != 1 {
		t.Fatalf("ExpireMaintenance() = %d, %v, want 1", ended, err)
	}
	var cleared db.Node
	database.First(&cleared, "id = ?", a.Node.ID)
	if cleared.MaintenanceMode || cleared.MaintenanceUntil != nil {
		t.Fatalf("node = %+v, want flag cleared", cleared)
	}
	var expired int64
	database.Model(&db.AuditEvent{}).Where("action = ? AND affected_object_id = ? AND actor_type = ?", NodeMaintenanceAuditExpired, a.Node.ID, "system").Count(&expired)
	if expired != 1 {
		t.Fatalf("expiry audit events = %d, want 1", expired)
	}
	if again, _ := nodes.ExpireMaintenance(time.Now().UTC()); again != 0 {
		t.Fatalf("second ExpireMaintenance() = %d, want idempotent 0", again)
	}
}

func TestSilentSweepSkipsNodesInActiveMaintenanceButNotExpired(t *testing.T) {
	database := testkit.NewDB(t)
	reportedAt := time.Now().UTC().Add(-time.Hour)
	a, _ := testkit.TwoNodes(t, database, reportedAt)
	svc := NewIncidentService(database, utils.NewLogger(), &config.Config{})
	until := time.Now().UTC().Add(time.Hour)
	database.Model(&db.Node{}).Where("id = ?", a.Node.ID).Updates(map[string]any{"maintenance_mode": true, "maintenance_until": &until})

	n, err := svc.SweepSilentMonitors(time.Now().UTC())
	if err != nil || n != 1 { // only the other node's monitor
		t.Fatalf("sweep during maintenance flagged %d (err %v), want 1 (the node not in maintenance)", n, err)
	}
	past := time.Now().UTC().Add(-time.Minute)
	database.Model(&db.Node{}).Where("id = ?", a.Node.ID).Update("maintenance_until", &past)
	if n, err = svc.SweepSilentMonitors(time.Now().UTC()); err != nil || n != 1 {
		t.Fatalf("sweep after expiry flagged %d (err %v), want the formerly silenced monitor", n, err)
	}
}
