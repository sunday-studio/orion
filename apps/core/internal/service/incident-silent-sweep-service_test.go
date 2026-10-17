package service

import (
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"testing"
	"time"

	"gorm.io/gorm"
)

func sweepFixture(t *testing.T, mutate func(*db.Agent, *db.Monitor)) (*gorm.DB, *IncidentService, db.Monitor, time.Time) {
	t.Helper()
	database := setupAlertServiceDatabase(t)
	created := time.Now().UTC().Add(-time.Hour)
	agent := db.Agent{ID: "agent-1", MachineId: "machine-1", Name: "host-1", OS: "linux", Arch: "amd64", Token: "t", LastSeen: created}
	monitor := db.Monitor{ID: "monitor-1", AgentID: agent.ID, Name: "homepage", Type: "http", Lifecycle: "active", Health: "up", ComputedHealth: "up", IncidentState: "up", ReportingIntervalSeconds: 60, CreatedAt: created}
	if mutate != nil {
		mutate(&agent, &monitor)
	}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := database.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	reportedAt := created
	if err := database.Create(&db.MonitorReport{ID: "report-1", MonitorID: monitor.ID, Payload: "{}", CollectedAt: reportedAt.Format(time.RFC3339), Health: "up", CreatedAt: reportedAt}).Error; err != nil {
		t.Fatalf("create report: %v", err)
	}
	return database, NewIncidentService(database, utils.NewLogger(), &config.Config{}), monitor, created
}

func activeIncidents(t *testing.T, database *gorm.DB, monitorID string) []db.Incident {
	t.Helper()
	var incidents []db.Incident
	if err := database.Where("monitor_id = ? AND status IN ?", monitorID, activeIncidentStatuses()).Find(&incidents).Error; err != nil {
		t.Fatalf("load incidents: %v", err)
	}
	return incidents
}

func TestSweepSilentMonitorsOpensStaleIncidentWhenAgentGoesQuiet(t *testing.T) {
	database, svc, monitor, reportedAt := sweepFixture(t, nil)
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})

	// Within the stale window (max(5m, 5x interval)): nothing happens.
	if n, err := svc.SweepSilentMonitors(reportedAt.Add(2 * time.Minute)); err != nil || n != 0 {
		t.Fatalf("early sweep = %d, %v, want 0", n, err)
	}
	if got := activeIncidents(t, database, monitor.ID); len(got) != 0 {
		t.Fatalf("incidents before stale window = %d, want 0", len(got))
	}

	n, err := svc.SweepSilentMonitors(reportedAt.Add(10 * time.Minute))
	if err != nil || n != 1 {
		t.Fatalf("stale sweep = %d, %v, want 1", n, err)
	}
	incidents := activeIncidents(t, database, monitor.ID)
	if len(incidents) != 1 || incidents[0].Status != "open" {
		t.Fatalf("incidents = %+v, want one open incident", incidents)
	}
	var deliveries []db.AlertDelivery
	database.Where("incident_id = ?", incidents[0].ID).Find(&deliveries)
	if len(deliveries) != 1 || deliveries[0].EventType != "incident_opened" || deliveries[0].Status != "pending" {
		t.Fatalf("deliveries = %+v, want one pending incident_opened", deliveries)
	}
	var got db.Monitor
	database.First(&got, "id = ?", monitor.ID)
	if got.IncidentState != "stale" || got.ActiveIncidentID != incidents[0].ID {
		t.Fatalf("monitor state = %q active = %q, want stale tracking incident", got.IncidentState, got.ActiveIncidentID)
	}
}

func TestSweepSilentMonitorsDoesNotRepeatForAlreadyStaleMonitor(t *testing.T) {
	database, svc, monitor, reportedAt := sweepFixture(t, nil)
	at := reportedAt.Add(10 * time.Minute)
	for i := range 3 {
		n, err := svc.SweepSilentMonitors(at.Add(time.Duration(i) * 30 * time.Second))
		if err != nil {
			t.Fatalf("sweep %d: %v", i, err)
		}
		if want := map[bool]int{true: 1, false: 0}[i == 0]; n != want {
			t.Fatalf("sweep %d flagged %d, want %d", i, n, want)
		}
	}
	if got := activeIncidents(t, database, monitor.ID); len(got) != 1 {
		t.Fatalf("incidents = %d, want exactly one", len(got))
	}
	var events int64
	database.Model(&db.IncidentEvent{}).Count(&events)
	if events != 1 {
		t.Fatalf("incident events = %d, want only the opening event", events)
	}
}

func TestSweepSilentMonitorsResolvesWhenReportsResume(t *testing.T) {
	database, svc, monitor, reportedAt := sweepFixture(t, nil)
	if _, err := svc.SweepSilentMonitors(reportedAt.Add(10 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := activeIncidents(t, database, monitor.ID); len(got) != 1 {
		t.Fatalf("incidents = %d, want 1", len(got))
	}

	payload := MonitorReportPayload{Health: "up"}
	if err := svc.ReconcileMonitorReport(monitor.ID, "report-2", payload); err != nil {
		t.Fatalf("ReconcileMonitorReport() error = %v", err)
	}
	if got := activeIncidents(t, database, monitor.ID); len(got) != 0 {
		t.Fatalf("active incidents after recovery = %d, want 0", len(got))
	}
	var resolved db.Incident
	database.Where("monitor_id = ?", monitor.ID).First(&resolved)
	if resolved.Status != "resolved" {
		t.Fatalf("incident status = %q, want resolved", resolved.Status)
	}
}

func TestSweepSilentMonitorsSkipsMaintenanceCoreManagedAndNeverReported(t *testing.T) {
	t.Run("maintenance mode", func(t *testing.T) {
		database, svc, monitor, reportedAt := sweepFixture(t, func(a *db.Agent, _ *db.Monitor) { a.MaintenanceMode = true })
		if n, _ := svc.SweepSilentMonitors(reportedAt.Add(time.Hour)); n != 0 || len(activeIncidents(t, database, monitor.ID)) != 0 {
			t.Fatalf("maintenance agent flagged %d, want none", n)
		}
	})
	t.Run("core managed monitor", func(t *testing.T) {
		database, svc, monitor, reportedAt := sweepFixture(t, nil)
		database.Create(&db.CoreMonitorConfig{MonitorID: monitor.ID, Kind: "http", ConfigJSON: "{}", SecretRefJSON: "{}", IntervalSeconds: 60})
		if n, _ := svc.SweepSilentMonitors(reportedAt.Add(time.Hour)); n != 0 {
			t.Fatalf("core-managed monitor flagged %d, want 0 (Worker owns it)", n)
		}
	})
	t.Run("disabled monitor", func(t *testing.T) {
		database, svc, monitor, reportedAt := sweepFixture(t, func(_ *db.Agent, m *db.Monitor) { m.Lifecycle = "disabled" })
		if n, _ := svc.SweepSilentMonitors(reportedAt.Add(time.Hour)); n != 0 || len(activeIncidents(t, database, monitor.ID)) != 0 {
			t.Fatalf("disabled monitor flagged %d, want none", n)
		}
	})
}

func TestSweepSilentMonitorsFlagsMonitorThatNeverReported(t *testing.T) {
	database, svc, monitor, created := sweepFixture(t, nil)
	database.Where("monitor_id = ?", monitor.ID).Delete(&db.MonitorReport{})
	if n, err := svc.SweepSilentMonitors(created.Add(2 * time.Minute)); err != nil || n != 0 {
		t.Fatalf("grace period sweep = %d, %v, want 0", n, err)
	}
	if n, err := svc.SweepSilentMonitors(created.Add(10 * time.Minute)); err != nil || n != 1 {
		t.Fatalf("never-reported sweep = %d, %v, want 1", n, err)
	}
}

func TestSweepSilentMonitorsSkipsRevokedAgents(t *testing.T) {
	database, svc, monitor, reportedAt := sweepFixture(t, nil)
	revoked := time.Now().UTC()
	database.Model(&db.Agent{}).Where("id = ?", monitor.AgentID).Update("token_revoked_at", &revoked)
	if n, _ := svc.SweepSilentMonitors(reportedAt.Add(time.Hour)); n != 0 || len(activeIncidents(t, database, monitor.ID)) != 0 {
		t.Fatalf("revoking an agent paged for its monitors (flagged %d)", n)
	}
}
