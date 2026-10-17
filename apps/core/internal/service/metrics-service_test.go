package service

import (
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"
)

func TestMetricsRenderQueueDepthOldestPendingAgeAndWorkerHeartbeat(t *testing.T) {
	database := testkit.NewDB(t)
	now := time.Now().UTC()
	diagnostics := NewRuntimeDiagnosticsService(database, utils.NewLogger())
	diagnostics.RecordRequest("/v1/agents/:id/report", 200, 12*time.Millisecond)
	diagnostics.RecordRequest("/v1/agents/:id/report", 200, 15*time.Millisecond)
	diagnostics.RecordRequest("unmatched", 404, time.Millisecond)

	for id, tc := range map[string]struct {
		status string
		age    time.Duration
	}{"d1": {"pending", 90 * time.Second}, "d2": {"pending", 10 * time.Second}, "d3": {"failed", time.Hour}, "d4": {"sent", time.Hour}} {
		database.Create(&db.AlertDelivery{ID: id, IncidentID: "i", EventType: "e", Channel: "c", Type: "webhook", Status: tc.status, MaxAttempts: 3, CreatedAt: now.Add(-tc.age)})
	}
	database.Create(&db.CoreWorkerStatus{WorkerID: "w1", ProcessKind: CoreMonitorWorkerProcessKind, Status: "running", StartedAt: now, LastHeartbeatAt: now.Add(-20 * time.Second)})
	database.Create(&db.Incident{ID: "inc", Status: "open", Severity: "high", Title: "t", AgentID: "a", MonitorID: "m", OpenedAt: now, LastEventAt: now, NotificationStatus: "pending"})

	out, err := NewMetricsService(database, utils.NewLogger(), diagnostics, time.Minute).Render(t.Context(), now)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# TYPE orion_http_requests_total counter",
		`orion_http_requests_total{route="/v1/agents/:id/report",status="200"} 2`,
		`orion_http_requests_total{route="unmatched",status="404"} 1`,
		`orion_alert_deliveries{status="pending"} 2`,
		`orion_alert_deliveries{status="failed"} 1`,
		"orion_alert_delivery_oldest_pending_age_seconds 90",
		`orion_worker_heartbeat_age_seconds{worker_id="w1"} 20`,
		"orion_worker_online 1",
		"orion_incidents_active 1",
		"orion_sqlite_busy_total 0",
		"orion_up 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics missing %q\n%s", want, out)
		}
	}
	if strings.Count(out, "# TYPE orion_alert_deliveries") != 1 {
		t.Error("each metric family must declare its TYPE once")
	}
}

func TestMetricsEscapeLabelValues(t *testing.T) {
	if got := escapeLabelValue("a\"b\\c\nd"); got != `a\"b\\c\nd` {
		t.Fatalf("escapeLabelValue = %q", got)
	}
}

func TestMetricsOldestPendingAgeIsZeroWhenQueueIsEmpty(t *testing.T) {
	database := testkit.NewDB(t)
	out, err := NewMetricsService(database, utils.NewLogger(), NewRuntimeDiagnosticsService(database, utils.NewLogger()), time.Minute).Render(t.Context(), time.Now().UTC())
	if err != nil || !strings.Contains(out, "orion_alert_delivery_oldest_pending_age_seconds 0") {
		t.Fatalf("err=%v out=%s, want age 0 for an empty queue", err, out)
	}
}
