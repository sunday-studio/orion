package service

import (
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"sync"
	"testing"
	"time"
)

func downPayload() MonitorReportPayload {
	return MonitorReportPayload{Timestamp: time.Now().UTC().Format(time.RFC3339), Health: "down", Metrics: map[string]any{}}
}

// A7: 20 concurrent failing reports for one monitor yield exactly one incident
// and exactly one incident_opened delivery.
func TestScenarioA7ConcurrentFailingReportsOpenOneIncident(t *testing.T) {
	database := testkit.NewDB(t)
	a, _ := testkit.TwoAgents(t, database, time.Now().UTC())
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc := NewIncidentService(database, utils.NewLogger(), &config.Config{})
			if err := svc.ReconcileMonitorReport(a.Monitor.ID, "report-concurrent-"+string(rune('a'+i)), downPayload()); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent reconcile failed: %v", err)
	}

	var incidents int64
	database.Model(&db.Incident{}).Where("monitor_id = ?", a.Monitor.ID).Count(&incidents)
	if incidents != 1 {
		t.Fatalf("incidents = %d, want exactly 1", incidents)
	}
	var opened int64
	database.Model(&db.AlertDelivery{}).Where("event_type = ?", "incident_opened").Count(&opened)
	if opened != 1 {
		t.Fatalf("incident_opened deliveries = %d, want exactly 1", opened)
	}
}

// A6: if queuing the delivery fails, the incident is not left behind without its page.
func TestScenarioA6IncidentAndDeliveryCommitTogether(t *testing.T) {
	database := testkit.NewDB(t)
	a, _ := testkit.TwoAgents(t, database, time.Now().UTC())
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	if err := database.Exec("ALTER TABLE alert_deliveries RENAME TO alert_deliveries_broken").Error; err != nil {
		t.Fatal(err)
	}

	svc := NewIncidentService(database, utils.NewLogger(), &config.Config{})
	if err := svc.ReconcileMonitorReport(a.Monitor.ID, "report-x", downPayload()); err == nil {
		t.Fatal("ReconcileMonitorReport() = nil, want error when the delivery cannot be queued")
	}

	var incidents, events int64
	database.Model(&db.Incident{}).Count(&incidents)
	database.Model(&db.IncidentEvent{}).Count(&events)
	if incidents != 0 || events != 0 {
		t.Fatalf("incidents = %d events = %d after failed queue, want none (all-or-nothing)", incidents, events)
	}
	var m db.Monitor
	database.First(&m, "id = ?", a.Monitor.ID)
	if m.ActiveIncidentID != "" {
		t.Fatalf("monitor active_incident_id = %q after rollback, want empty", m.ActiveIncidentID)
	}
}

func TestUniqueActiveIncidentIndexRejectsSecondActiveIncident(t *testing.T) {
	database := testkit.NewDB(t)
	a, _ := testkit.TwoAgents(t, database, time.Now().UTC())
	now := time.Now().UTC()
	mk := func(id, status string) error {
		return database.Create(&db.Incident{ID: id, Status: status, Severity: "high", Title: "t", AgentID: a.Agent.ID, MonitorID: a.Monitor.ID, OpenedAt: now, LastEventAt: now, NotificationStatus: "pending"}).Error
	}
	if err := mk("i1", "open"); err != nil {
		t.Fatal(err)
	}
	if err := mk("i2", "open"); err == nil {
		t.Fatal("second active incident for the same monitor was accepted, want unique violation")
	}
	if err := mk("i3", "resolved"); err != nil {
		t.Fatalf("resolved incident rejected: %v", err)
	}
}
