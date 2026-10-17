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

// A8: the same report ID posted 5 times stores one report and returns the same ID each time.
func TestScenarioA8ReplayedReportIDStoresOneRow(t *testing.T) {
	database := testkit.NewDB(t)
	a, _ := testkit.TwoAgents(t, database, time.Now().UTC())
	reports := NewReportService(database, utils.NewLogger(), &config.Config{})
	payload := downPayload()
	payload.ReportID = "0b0c9d56-7a40-4b0e-9f1a-aaaaaaaaaaaa"

	var firstID string
	for i := range 5 {
		id, err := reports.StoreMonitorReport(a.Monitor.ID, payload)
		if err != nil || id == nil {
			t.Fatalf("replay %d: id=%v err=%v, want success every time", i, id, err)
		}
		if i == 0 {
			firstID = *id
		} else if *id != firstID {
			t.Fatalf("replay %d returned %q, want original %q", i, *id, firstID)
		}
	}
	var stored, incidents int64
	database.Model(&db.MonitorReport{}).Where("monitor_id = ? AND client_report_id = ?", a.Monitor.ID, payload.ReportID).Count(&stored)
	database.Model(&db.Incident{}).Where("monitor_id = ?", a.Monitor.ID).Count(&incidents)
	if stored != 1 || incidents != 1 {
		t.Fatalf("stored = %d incidents = %d, want 1 and 1", stored, incidents)
	}

	// A different report ID is a new report, and reports without an ID are never deduplicated.
	payload.ReportID = "0b0c9d56-7a40-4b0e-9f1a-bbbbbbbbbbbb"
	if _, err := reports.StoreMonitorReport(a.Monitor.ID, payload); err != nil {
		t.Fatal(err)
	}
	noID := downPayload()
	for range 2 {
		if _, err := reports.StoreMonitorReport(a.Monitor.ID, noID); err != nil {
			t.Fatal(err)
		}
	}
	database.Model(&db.MonitorReport{}).Where("monitor_id = ? AND id LIKE ?", a.Monitor.ID, "monitor_report-%").Count(&stored)
	if stored != 4 {
		t.Fatalf("total new reports = %d, want 4 (1 + 1 + 2 without ids)", stored)
	}
}

// A failed reconcile must roll the report back so the agent's retry is not a silent no-op.
func TestFailedReconcileRollsBackReportSoRetryRecovers(t *testing.T) {
	database := testkit.NewDB(t)
	a, _ := testkit.TwoAgents(t, database, time.Now().UTC())
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	reports := NewReportService(database, utils.NewLogger(), &config.Config{})
	payload := downPayload()
	payload.ReportID = "retry-me"

	database.Exec("ALTER TABLE alert_deliveries RENAME TO alert_deliveries_broken")
	if _, err := reports.StoreMonitorReport(a.Monitor.ID, payload); err == nil {
		t.Fatal("StoreMonitorReport() = nil error with broken delivery table")
	}
	var stored int64
	database.Model(&db.MonitorReport{}).Where("client_report_id = ?", "retry-me").Count(&stored)
	if stored != 0 {
		t.Fatalf("report rows after failed ingest = %d, want 0 (rolled back)", stored)
	}

	database.Exec("ALTER TABLE alert_deliveries_broken RENAME TO alert_deliveries")
	if _, err := reports.StoreMonitorReport(a.Monitor.ID, payload); err != nil {
		t.Fatalf("retry error = %v", err)
	}
	var incidents int64
	database.Model(&db.Incident{}).Where("monitor_id = ?", a.Monitor.ID).Count(&incidents)
	if incidents != 1 {
		t.Fatalf("incidents after retry = %d, want 1 (page not lost)", incidents)
	}
}
