package service

import (
	"encoding/json"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"testing"
	"time"
)

// D1 + D2: a silent node raises an incident and a delivery; when it resumes the
// incident resolves and exactly one recovery delivery is sent.
func TestScenarioD1D2SilentNodeAlertsThenRecovers(t *testing.T) {
	database := testkit.NewDB(t)
	hook := testkit.NewWebhook(t)
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops", Type: "webhook", Enabled: true, WebhookURL: hook.URL()})
	cfg := &config.Config{AlertAllowPrivateWebhooks: true, AlertRecoveryNotifications: true}
	reportedAt := time.Now().UTC().Add(-time.Hour)
	a, _ := testkit.TwoNodes(t, database, reportedAt)
	incidents := NewIncidentService(database, utils.NewLogger(), cfg)
	alerts := NewAlertService(database, utils.NewLogger(), cfg)

	// Node A goes silent: within window + one sweep interval an incident and a page exist.
	if n, err := incidents.SweepSilentMonitors(reportedAt.Add(6*time.Minute + 30*time.Second)); err != nil || n != 2 {
		t.Fatalf("sweep flagged %d (err %v), want both silent monitors", n, err)
	}
	if _, err := alerts.ProcessDueDeliveries(10); err != nil {
		t.Fatal(err)
	}
	var opened int64
	database.Model(&db.AlertDelivery{}).Where("event_type = ? AND status = ?", "incident_opened", "sent").Count(&opened)
	if opened != 2 || hook.Calls() != 2 {
		t.Fatalf("opened deliveries sent = %d, webhook calls = %d, want 2/2", opened, hook.Calls())
	}

	// Node A resumes reporting: its incident resolves and one recovery page is sent.
	database.Create(&db.MonitorReport{ID: "report-back", MonitorID: a.Monitor.ID, Payload: "{}", CollectedAt: time.Now().UTC().Format(time.RFC3339), Health: "up", CreatedAt: time.Now().UTC()})
	if err := incidents.ReconcileMonitorReport(a.Monitor.ID, "report-back", MonitorReportPayload{Health: "up", Timestamp: time.Now().UTC().Format(time.RFC3339), Metrics: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := alerts.ProcessDueDeliveries(10); err != nil {
		t.Fatal(err)
	}
	if _, err := alerts.ProcessDueDeliveries(10); err != nil {
		t.Fatal(err)
	}
	var resolved int64
	database.Model(&db.AlertDelivery{}).Where("event_type = ? AND status = ?", "incident_resolved", "sent").Count(&resolved)
	if resolved != 1 {
		t.Fatalf("recovery deliveries sent = %d, want exactly 1", resolved)
	}
	var bodies []map[string]any
	for _, raw := range hook.Bodies() {
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		bodies = append(bodies, m)
	}
	if len(bodies) != 3 {
		t.Fatalf("webhook received %d payloads, want 3 (2 opened + 1 resolved)", len(bodies))
	}
}

// D3: a future-dated collected_at must not make an old report look fresh.
func TestScenarioD3FutureDatedCollectedAtIsStillStale(t *testing.T) {
	database := testkit.NewDB(t)
	old := time.Now().UTC().Add(-2 * time.Hour)
	a, _ := testkit.TwoNodes(t, database, old)
	database.Model(&db.MonitorReport{}).Where("monitor_id = ?", a.Monitor.ID).Update("collected_at", time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339))

	health, err := NewHealthService(database, utils.NewLogger()).ComputeMonitorHealth(a.Monitor.ID, DefaultHealthConfig())
	if err != nil || health != "stale" {
		t.Fatalf("health = %q, err = %v, want stale by server time", health, err)
	}
	stale, err := NewHealthService(database, utils.NewLogger()).DetectStaleMonitorsForNode(a.Node.ID, DefaultHealthConfig())
	if err != nil || len(stale) != 1 {
		t.Fatalf("stale monitors for node = %d, err = %v, want 1", len(stale), err)
	}
}

// D4: a latest `down` report is down even with a flappy history.
func TestScenarioD4LatestDownIsNotSoftenedToDegraded(t *testing.T) {
	database := testkit.NewDB(t)
	now := time.Now().UTC()
	a, _ := testkit.TwoNodes(t, database, now.Add(-30*time.Minute))
	database.Where("monitor_id = ?", a.Monitor.ID).Delete(&db.MonitorReport{})
	for i := 0; i < 10; i++ {
		health := "up"
		if i%2 == 1 {
			health = "down"
		}
		at := now.Add(-time.Duration(10-i) * 10 * time.Second)
		database.Create(&db.MonitorReport{ID: "r" + string(rune('a'+i)), MonitorID: a.Monitor.ID, Payload: "{}", CollectedAt: at.Format(time.RFC3339), Health: health, CreatedAt: at})
	}
	got, err := NewHealthService(database, utils.NewLogger()).ComputeMonitorHealth(a.Monitor.ID, DefaultHealthConfig())
	if err != nil || got != "down" {
		t.Fatalf("health = %q, err = %v, want down (latest report is down)", got, err)
	}
}
