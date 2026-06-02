package worker

import (
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"
)

func TestReconcileMissedHeartbeatsLeavesPendingMonitorUnknown(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-heartbeat-pending")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-heartbeat-pending",
		Kind:            "heartbeat",
		ConfigJSON:      `{"grace_seconds":30}`,
		IntervalSeconds: 60,
		NextRunAt:       time.Now().UTC().Add(-time.Hour),
	})

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-heartbeat-test"})
	if err := app.reconcileMissedHeartbeats(t.Context(), time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("reconcileMissedHeartbeats() error = %v", err)
	}

	if countWorkerMonitorReports(t, database, "monitor-heartbeat-pending") != 0 {
		t.Fatalf("pending heartbeat has reports, want none before first signal")
	}
	if countWorkerIncidents(t, database, "monitor-heartbeat-pending") != 0 {
		t.Fatalf("pending heartbeat has incidents, want none before first signal")
	}
	var monitor db.Monitor
	if err := database.Where("id = ?", "monitor-heartbeat-pending").First(&monitor).Error; err != nil {
		t.Fatalf("load pending heartbeat monitor: %v", err)
	}
	if monitor.Health != "unknown" {
		t.Fatalf("pending heartbeat health = %q, want unknown", monitor.Health)
	}
}

func TestReconcileMissedHeartbeatsIgnoresSignalsInsideGraceWindow(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-heartbeat-grace")
	now := time.Date(2026, 5, 28, 9, 0, 0, 0, time.UTC)
	lastSignalAt := now.Add(-80 * time.Second)
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-heartbeat-grace",
		Kind:            "heartbeat",
		ConfigJSON:      `{"grace_seconds":30}`,
		IntervalSeconds: 60,
		NextRunAt:       now.Add(-time.Hour),
		LastSignalAt:    &lastSignalAt,
		LastSuccessAt:   &lastSignalAt,
	})

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-heartbeat-test"})
	if err := app.reconcileMissedHeartbeats(t.Context(), now); err != nil {
		t.Fatalf("reconcileMissedHeartbeats() error = %v", err)
	}
	if countWorkerMonitorReports(t, database, "monitor-heartbeat-grace") != 0 {
		t.Fatalf("heartbeat inside grace window has reports, want none")
	}
	if countWorkerIncidents(t, database, "monitor-heartbeat-grace") != 0 {
		t.Fatalf("heartbeat inside grace window has incidents, want none")
	}
}

func TestReconcileMissedHeartbeatsOpensAndRecoveryResolvesIncident(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-heartbeat-missed")
	now := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	lastSignalAt := now.Add(-3 * time.Minute)
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-heartbeat-missed",
		Kind:            "heartbeat",
		ConfigJSON:      `{"grace_seconds":30}`,
		IntervalSeconds: 60,
		NextRunAt:       now.Add(-time.Hour),
		LastSignalAt:    &lastSignalAt,
		LastSuccessAt:   &lastSignalAt,
	})

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-heartbeat-test"})
	if err := app.reconcileMissedHeartbeats(t.Context(), now); err != nil {
		t.Fatalf("reconcileMissedHeartbeats() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-heartbeat-missed")
	if report.Health != "stale" {
		t.Fatalf("missed heartbeat report health = %q, want stale", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal missed heartbeat payload: %v", err)
	}
	if payload["type"] != "heartbeat" || payload["failure_stage"] != "missed_signal" {
		t.Fatalf("missed heartbeat payload = %+v, want missed_signal heartbeat", payload)
	}
	var incident db.Incident
	if err := database.Where("monitor_id = ?", "monitor-heartbeat-missed").First(&incident).Error; err != nil {
		t.Fatalf("load missed heartbeat incident: %v", err)
	}
	if incident.Status != "open" || incident.Severity != "high" {
		t.Fatalf("missed heartbeat incident = %+v, want open high incident", incident)
	}
	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-heartbeat-missed")
	if completed.LastSignalAt == nil || !completed.LastSignalAt.Equal(lastSignalAt) || completed.LastFailureAt == nil {
		t.Fatalf("missed heartbeat config = %+v, want original signal and failure timestamp", completed)
	}

	if err := app.reconcileMissedHeartbeats(t.Context(), now.Add(time.Second)); err != nil {
		t.Fatalf("second reconcileMissedHeartbeats() error = %v", err)
	}
	if countWorkerMonitorReports(t, database, "monitor-heartbeat-missed") != 1 {
		t.Fatalf("missed heartbeat duplicate reports = %d, want one", countWorkerMonitorReports(t, database, "monitor-heartbeat-missed"))
	}

	recoveredAt := now.Add(time.Minute)
	if _, err := app.reports.StoreMonitorReport("monitor-heartbeat-missed", service.MonitorReportPayload{
		Timestamp: recoveredAt.Format(time.RFC3339),
		Health:    "up",
		Metrics: map[string]any{
			"runner": "core",
			"type":   "heartbeat",
		},
	}); err != nil {
		t.Fatalf("store heartbeat recovery report: %v", err)
	}
	if err := database.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", "monitor-heartbeat-missed").Updates(map[string]any{
		"last_signal_at":  recoveredAt,
		"last_success_at": recoveredAt,
		"updated_at":      recoveredAt,
	}).Error; err != nil {
		t.Fatalf("update heartbeat recovery config: %v", err)
	}
	if err := database.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload recovered heartbeat incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil {
		t.Fatalf("recovered heartbeat incident = %+v, want resolved", incident)
	}
}

func TestReconcileMissedHeartbeatsDoesNotOverrideFailedSignal(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-heartbeat-failed")
	now := time.Date(2026, 5, 28, 11, 0, 0, 0, time.UTC)
	lastSignalAt := now.Add(-10 * time.Minute)
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-heartbeat-failed",
		Kind:            "heartbeat",
		ConfigJSON:      `{"grace_seconds":30}`,
		IntervalSeconds: 60,
		NextRunAt:       now.Add(-time.Hour),
		LastSignalAt:    &lastSignalAt,
		LastFailureAt:   &lastSignalAt,
	})

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-heartbeat-test"})
	if _, err := app.reports.StoreMonitorReport("monitor-heartbeat-failed", service.MonitorReportPayload{
		Timestamp: lastSignalAt.Format(time.RFC3339),
		Health:    "down",
		Metrics: map[string]any{
			"runner": "core",
			"type":   "heartbeat",
		},
	}); err != nil {
		t.Fatalf("store heartbeat failure report: %v", err)
	}

	if err := app.reconcileMissedHeartbeats(t.Context(), now); err != nil {
		t.Fatalf("reconcileMissedHeartbeats() error = %v", err)
	}
	if count := countWorkerMonitorReports(t, database, "monitor-heartbeat-failed"); count != 1 {
		t.Fatalf("failed heartbeat reports = %d, want only explicit failure report", count)
	}
	var incident db.Incident
	if err := database.Where("monitor_id = ?", "monitor-heartbeat-failed").First(&incident).Error; err != nil {
		t.Fatalf("load failed heartbeat incident: %v", err)
	}
	if incident.Status != "open" || !strings.Contains(incident.LatestEvent, "down") {
		t.Fatalf("failed heartbeat incident = %+v, want open down incident", incident)
	}
}

func TestReconcileMissedHeartbeatsSkipsPausedDeletedAndNonHeartbeatMonitors(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	insertWorkerCoreOwner(t, database)
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	lastSignalAt := now.Add(-10 * time.Minute)

	cases := []struct {
		monitorID string
		kind      string
		lifecycle string
		paused    bool
	}{
		{monitorID: "monitor-heartbeat-paused", kind: "heartbeat", lifecycle: "active", paused: true},
		{monitorID: "monitor-heartbeat-deleted", kind: "heartbeat", lifecycle: "deleted"},
		{monitorID: "monitor-heartbeat-http", kind: "http", lifecycle: "active"},
	}
	for _, tc := range cases {
		insertWorkerMonitor(t, database, tc.monitorID)
		if err := database.Model(&db.Monitor{}).Where("id = ?", tc.monitorID).Update("lifecycle", tc.lifecycle).Error; err != nil {
			t.Fatalf("set monitor lifecycle: %v", err)
		}
		insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
			MonitorID:       tc.monitorID,
			Kind:            tc.kind,
			ConfigJSON:      `{"grace_seconds":30}`,
			IntervalSeconds: 60,
			Paused:          tc.paused,
			NextRunAt:       now.Add(-time.Hour),
			LastSignalAt:    &lastSignalAt,
			LastSuccessAt:   &lastSignalAt,
		})
	}

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-heartbeat-test"})
	if err := app.reconcileMissedHeartbeats(t.Context(), now); err != nil {
		t.Fatalf("reconcileMissedHeartbeats() error = %v", err)
	}
	for _, tc := range cases {
		if countWorkerMonitorReports(t, database, tc.monitorID) != 0 {
			t.Fatalf("%s has reports, want skipped", tc.monitorID)
		}
		if countWorkerIncidents(t, database, tc.monitorID) != 0 {
			t.Fatalf("%s has incidents, want skipped", tc.monitorID)
		}
	}
}
