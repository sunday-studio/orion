package service

import (
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"testing"
	"time"
)

var agentActor = AgentTokenActionInput{ActorType: "agent", ActorID: "agent-a", RequestID: "req-1"}

func TestAgentInMaintenanceHonoursExpiry(t *testing.T) {
	now := time.Now().UTC()
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	for name, tc := range map[string]struct {
		agent db.Agent
		want  bool
	}{
		"off":                {db.Agent{}, false},
		"on, future end":     {db.Agent{MaintenanceMode: true, MaintenanceUntil: &future}, true},
		"on, ended":          {db.Agent{MaintenanceMode: true, MaintenanceUntil: &past}, false},
		"on, legacy no end":  {db.Agent{MaintenanceMode: true}, true},
		"off but stale time": {db.Agent{MaintenanceUntil: &future}, false},
	} {
		if got := tc.agent.InMaintenance(now); got != tc.want {
			t.Errorf("%s: InMaintenance = %t, want %t", name, got, tc.want)
		}
	}
}

func TestSetMaintenanceModeDefaultsToTwentyFourHoursAndIsAudited(t *testing.T) {
	database := testkit.NewDB(t)
	a, _ := testkit.TwoAgents(t, database, time.Now().UTC())
	svc := NewAgentService(database, utils.NewLogger())

	if err := svc.SetMaintenanceMode(a.Agent.ID, true, 0, agentActor); err != nil {
		t.Fatal(err)
	}
	var agent db.Agent
	database.First(&agent, "id = ?", a.Agent.ID)
	if !agent.MaintenanceMode || agent.MaintenanceUntil == nil {
		t.Fatalf("agent = %+v, want maintenance with an end time", agent)
	}
	if d := time.Until(*agent.MaintenanceUntil); d < 23*time.Hour || d > 25*time.Hour {
		t.Fatalf("default maintenance ends in %v, want about 24h", d)
	}

	if err := svc.SetMaintenanceMode(a.Agent.ID, true, 31*24*time.Hour, agentActor); err != ErrInvalidMaintenanceDuration {
		t.Fatalf("31-day maintenance error = %v, want ErrInvalidMaintenanceDuration", err)
	}
	if err := svc.SetMaintenanceMode(a.Agent.ID, false, 0, agentActor); err != nil {
		t.Fatal(err)
	}
	var cleared db.Agent
	database.First(&cleared, "id = ?", a.Agent.ID)
	if cleared.MaintenanceMode || cleared.MaintenanceUntil != nil {
		t.Fatalf("agent = %+v, want maintenance cleared", cleared)
	}

	var actions []string
	database.Model(&db.AuditEvent{}).Where("affected_object_id = ?", a.Agent.ID).Order("created_at ASC").Pluck("action", &actions)
	if len(actions) != 2 || actions[0] != AgentMaintenanceAuditEnabled || actions[1] != AgentMaintenanceAuditDisabled {
		t.Fatalf("audit actions = %v, want enabled then disabled", actions)
	}
	if err := svc.SetMaintenanceMode("no-such-agent", true, 0, agentActor); err == nil {
		t.Fatal("maintenance on an unknown agent = nil error")
	}
}

// D6: maintenance suppresses alerts; once it expires, alerting resumes.
func TestScenarioD6ExpiredMaintenanceResumesAlerting(t *testing.T) {
	database := testkit.NewDB(t)
	a, _ := testkit.TwoAgents(t, database, time.Now().UTC())
	cfg := &config.Config{}
	agents := NewAgentService(database, utils.NewLogger())
	incidents := NewIncidentService(database, utils.NewLogger(), cfg)
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})

	if err := agents.SetMaintenanceMode(a.Agent.ID, true, time.Hour, agentActor); err != nil {
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
	database.Model(&db.Agent{}).Where("id = ?", a.Agent.ID).Update("maintenance_until", &past)
	if err := incidents.ReconcileMonitorReport(a.Monitor.ID, "r2", downPayload()); err != nil {
		t.Fatal(err)
	}
	database.Model(&db.Incident{}).Where("monitor_id = ?", a.Monitor.ID).Count(&count)
	if count != 1 {
		t.Fatalf("incidents after maintenance ended = %d, want 1 (alerting resumed)", count)
	}

	// ...and the sweeper clears the flag and writes the audit trail.
	ended, err := agents.ExpireMaintenance(time.Now().UTC())
	if err != nil || ended != 1 {
		t.Fatalf("ExpireMaintenance() = %d, %v, want 1", ended, err)
	}
	var cleared db.Agent
	database.First(&cleared, "id = ?", a.Agent.ID)
	if cleared.MaintenanceMode || cleared.MaintenanceUntil != nil {
		t.Fatalf("agent = %+v, want flag cleared", cleared)
	}
	var expired int64
	database.Model(&db.AuditEvent{}).Where("action = ? AND affected_object_id = ? AND actor_type = ?", AgentMaintenanceAuditExpired, a.Agent.ID, "system").Count(&expired)
	if expired != 1 {
		t.Fatalf("expiry audit events = %d, want 1", expired)
	}
	if again, _ := agents.ExpireMaintenance(time.Now().UTC()); again != 0 {
		t.Fatalf("second ExpireMaintenance() = %d, want idempotent 0", again)
	}
}

func TestSilentSweepSkipsAgentsInActiveMaintenanceButNotExpired(t *testing.T) {
	database := testkit.NewDB(t)
	reportedAt := time.Now().UTC().Add(-time.Hour)
	a, _ := testkit.TwoAgents(t, database, reportedAt)
	svc := NewIncidentService(database, utils.NewLogger(), &config.Config{})
	until := time.Now().UTC().Add(time.Hour)
	database.Model(&db.Agent{}).Where("id = ?", a.Agent.ID).Updates(map[string]any{"maintenance_mode": true, "maintenance_until": &until})

	n, err := svc.SweepSilentMonitors(time.Now().UTC())
	if err != nil || n != 1 { // only the other agent's monitor
		t.Fatalf("sweep during maintenance flagged %d (err %v), want 1 (the agent not in maintenance)", n, err)
	}
	past := time.Now().UTC().Add(-time.Minute)
	database.Model(&db.Agent{}).Where("id = ?", a.Agent.ID).Update("maintenance_until", &past)
	if n, err = svc.SweepSilentMonitors(time.Now().UTC()); err != nil || n != 1 {
		t.Fatalf("sweep after expiry flagged %d (err %v), want the formerly silenced monitor", n, err)
	}
}
