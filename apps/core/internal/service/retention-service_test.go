package service

import (
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func seedOldMonitorReports(t *testing.T, database *gorm.DB, n int) {
	t.Helper()
	sql := fmt.Sprintf(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < %d)
		INSERT INTO monitor_reports (id, monitor_id, payload, collected_at, health, client_report_id, created_at)
		SELECT 'old-' || i, 'm1', '{}', '2020-01-01T00:00:00Z', 'up', '', '2020-01-01 00:00:00+00:00' FROM n`, n)
	if err := database.Exec(sql).Error; err != nil {
		t.Fatalf("seed backlog: %v", err)
	}
}

// ORB-027: a backlog far beyond SQLite's bound-parameter limit archives in batches.
func TestArchiveHandlesHundredThousandRowBacklog(t *testing.T) {
	database := testkit.NewDB(t)
	dataDir := t.TempDir()
	seedOldMonitorReports(t, database, 100_000)
	// One fresh row that must stay in the live database.
	database.Create(&db.MonitorReport{ID: "fresh", MonitorID: "m1", Payload: "{}", CollectedAt: time.Now().UTC().Format(time.RFC3339), Health: "up", CreatedAt: time.Now().UTC()})

	svc := NewArchiveService(database, utils.NewLogger(), dataDir)
	result, err := svc.RunRawReportArchive(time.Now().UTC())
	if err != nil {
		t.Fatalf("RunRawReportArchive() error = %v", err)
	}
	if result.MonitorReportsArchived != 100_000 {
		t.Fatalf("archived = %d, want 100000", result.MonitorReportsArchived)
	}
	var live int64
	database.Model(&db.MonitorReport{}).Count(&live)
	if live != 1 {
		t.Fatalf("live rows after archive = %d, want only the fresh one", live)
	}

	archive, err := gorm.Open(sqlite.Open(result.ArchivePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var archived int64
	archive.Model(&db.MonitorReport{}).Count(&archived)
	if archived != 100_000 {
		t.Fatalf("rows in archive file = %d, want 100000", archived)
	}
	closeArchiveDatabase(archive)

	// Running again is a no-op, not an error.
	again, err := svc.RunRawReportArchive(time.Now().UTC())
	if err != nil || again.MonitorReportsArchived != 0 || !again.SkippedBecauseNoReports {
		t.Fatalf("second run = %+v err %v, want skipped with nothing to move", again, err)
	}
}

func TestRetentionPrunesGrowthTablesButKeepsPendingAndActiveHistory(t *testing.T) {
	database := testkit.NewDB(t)
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -400)
	recent := now.Add(-time.Hour)

	for i, at := range []time.Time{old, recent} {
		database.Create(&db.ServiceLogEntry{ID: fmt.Sprintf("log-%d", i), AgentID: "a", Message: "m", Fingerprint: fmt.Sprintf("fp-%d", i), OccurredAt: at, CollectedAt: at, CreatedAt: at})
		database.Create(&db.AuditEvent{ID: fmt.Sprintf("audit-%d", i), Action: "x", AffectedObjectType: "t", AffectedObjectID: "1", ActorType: "u", ActorID: "u", CreatedAt: at})
		database.Create(&db.CoreWorkerStatus{WorkerID: fmt.Sprintf("w-%d", i), StartedAt: at, LastHeartbeatAt: at})
	}
	for id, tc := range map[string]struct {
		status string
		at     time.Time
	}{"d-old-sent": {"sent", old}, "d-old-failed": {"failed", old}, "d-old-pending": {"pending", old}, "d-recent-sent": {"sent", recent}} {
		database.Create(&db.AlertDelivery{ID: id, IncidentID: "i", EventType: "incident_opened", Channel: "c", Type: "webhook", Status: tc.status, MaxAttempts: 3, CreatedAt: tc.at})
		database.Create(&db.AlertDeliveryAttempt{ID: "att-" + id, AlertDeliveryID: id, AttemptNumber: 1, Status: "sent", Stage: "transport", StartedAt: tc.at, CreatedAt: tc.at})
	}
	database.Create(&db.Incident{ID: "inc-resolved", Status: "resolved", Severity: "high", Title: "t", AgentID: "a", MonitorID: "m-r", OpenedAt: old, LastEventAt: old, NotificationStatus: "sent"})
	database.Create(&db.Incident{ID: "inc-open", Status: "open", Severity: "high", Title: "t", AgentID: "a", MonitorID: "m-o", OpenedAt: old, LastEventAt: old, NotificationStatus: "sent"})
	database.Create(&db.IncidentEvent{ID: "ev-resolved-old", IncidentID: "inc-resolved", Type: "x", CreatedAt: old})
	database.Create(&db.IncidentEvent{ID: "ev-open-old", IncidentID: "inc-open", Type: "x", CreatedAt: old})

	result, err := NewRetentionService(database, utils.NewLogger(), t.TempDir()).Run(now)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	count := func(model any, where string, args ...any) int64 {
		var n int64
		database.Model(model).Where(where, args...).Count(&n)
		return n
	}
	if count(&db.ServiceLogEntry{}, "id = ?", "log-0") != 0 || count(&db.ServiceLogEntry{}, "id = ?", "log-1") != 1 {
		t.Fatal("service logs: old row should be pruned, recent kept")
	}
	if count(&db.AuditEvent{}, "id = ?", "audit-0") != 0 || count(&db.AuditEvent{}, "id = ?", "audit-1") != 1 {
		t.Fatal("audit events: old pruned (365d), recent kept")
	}
	if count(&db.CoreWorkerStatus{}, "worker_id = ?", "w-0") != 0 || count(&db.CoreWorkerStatus{}, "worker_id = ?", "w-1") != 1 {
		t.Fatal("worker statuses: old pruned, recent kept")
	}
	if count(&db.AlertDelivery{}, "id IN ?", []string{"d-old-sent", "d-old-failed"}) != 0 {
		t.Fatal("old terminal deliveries should be pruned")
	}
	if count(&db.AlertDelivery{}, "id IN ?", []string{"d-old-pending", "d-recent-sent"}) != 2 {
		t.Fatal("pending and recent deliveries must be kept")
	}
	if count(&db.AlertDeliveryAttempt{}, "alert_delivery_id IN ?", []string{"d-old-sent", "d-old-failed"}) != 0 || count(&db.AlertDeliveryAttempt{}, "alert_delivery_id = ?", "d-old-pending") != 1 {
		t.Fatal("attempts follow their delivery")
	}
	if count(&db.IncidentEvent{}, "id = ?", "ev-resolved-old") != 0 || count(&db.IncidentEvent{}, "id = ?", "ev-open-old") != 1 {
		t.Fatal("only resolved incidents' old events are pruned; an active incident keeps its timeline")
	}
	if result.ServiceLogEntries != 1 || result.AlertDeliveries != 2 || result.IncidentEvents != 1 {
		t.Fatalf("result = %+v, want counts for pruned rows", result)
	}
}

func TestRetentionDeletesMoreThanOneBatch(t *testing.T) {
	database := testkit.NewDB(t)
	old := time.Now().UTC().AddDate(0, 0, -400)
	if err := database.Exec(fmt.Sprintf(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 2500)
		INSERT INTO audit_events (id, action, status_page_id, affected_object_type, affected_object_id, actor_type, actor_id, metadata_json, created_at)
		SELECT 'a-' || i, 'x', '', 't', '1', 'u', 'u', '{}', '%s' FROM n`, old.Format("2006-01-02 15:04:05+00:00"))).Error; err != nil {
		t.Fatal(err)
	}
	result, err := NewRetentionService(database, utils.NewLogger(), t.TempDir()).Run(time.Now().UTC())
	if err != nil || result.AuditEvents != 2500 {
		t.Fatalf("audit pruned = %d err = %v, want 2500 across multiple batches", result.AuditEvents, err)
	}
}

func TestRetentionSettingsValidation(t *testing.T) {
	database := testkit.NewDB(t)
	svc := NewSettingsService(database, utils.NewLogger(), t.TempDir())
	bad := 0
	payload := DataLifecycleSettingsPayload{RawReportHotDays: 30, RollupsEnabled: true, ArchiveSchedule: "daily", ServiceLogRetentionDays: &bad}
	if _, err := svc.UpdateDataLifecycleSettings(payload); err == nil {
		t.Fatal("retention of 0 days accepted, want rejection")
	}
	ok := 30
	payload.ServiceLogRetentionDays = &ok
	got, err := svc.UpdateDataLifecycleSettings(payload)
	if err != nil || got.ServiceLogRetentionDays != 30 || got.AuditEventRetentionDays != 365 {
		t.Fatalf("settings = %+v err = %v, want service log 30 and audit untouched at 365", got, err)
	}
}
