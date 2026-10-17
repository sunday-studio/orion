package service

import (
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"
)

func recordWorkerHeartbeat(t *testing.T, database *gorm.DB, at time.Time) {
	t.Helper()
	row := db.CoreWorkerStatus{WorkerID: "worker-1", ProcessKind: CoreMonitorWorkerProcessKind, Status: "running", StartedAt: at, LastHeartbeatAt: at}
	if err := database.Save(&row).Error; err != nil {
		t.Fatal(err)
	}
}

func selfMonitorFixture(t *testing.T) (*SelfMonitorService, *testkit.Webhook, *gorm.DB) {
	t.Helper()
	database := testkit.NewDB(t)
	hook := testkit.NewWebhook(t)
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops", Type: "webhook", Enabled: true, WebhookURL: hook.URL()})
	cfg := &config.Config{CoreWorkerStaleSeconds: 60, AlertAllowPrivateWebhooks: true, AlertRecoveryNotifications: true}
	return NewSelfMonitorService(database, utils.NewLogger(), cfg, NewRuntimeDiagnosticsService(database, utils.NewLogger())), hook, database
}

// 4.1: stopping the Worker raises an alert from Core itself, with no Worker running to send it.
func TestStoppedWorkerRaisesAnAlertFromTheAPIProcess(t *testing.T) {
	svc, hook, database := selfMonitorFixture(t)
	now := time.Now().UTC()

	recordWorkerHeartbeat(t, database, now.Add(-5*time.Second))
	if err := svc.Check(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	var incidents int64
	database.Model(&db.Incident{}).Where("monitor_id = ?", coreWorkerMonitorID).Count(&incidents)
	if incidents != 0 || hook.Calls() != 0 {
		t.Fatalf("healthy worker: incidents=%d webhook calls=%d, want none", incidents, hook.Calls())
	}

	// The Worker stops: its last heartbeat is now far older than the stale window.
	later := now.Add(5 * time.Minute)
	if err := svc.Check(t.Context(), later); err != nil {
		t.Fatal(err)
	}
	database.Model(&db.Incident{}).Where("monitor_id = ? AND status = ?", coreWorkerMonitorID, "open").Count(&incidents)
	if incidents != 1 {
		t.Fatalf("open incidents for a dead worker = %d, want 1", incidents)
	}
	if hook.Calls() != 1 {
		t.Fatalf("webhook calls = %d, want the page delivered by the API process itself", hook.Calls())
	}
	var body string
	if bodies := hook.Bodies(); len(bodies) == 1 {
		body = string(bodies[0])
	}
	if !strings.Contains(body, "Core worker") {
		t.Fatalf("page payload does not name the Core worker: %s", body)
	}

	// Repeated checks while it stays down do not re-page.
	if err := svc.Check(t.Context(), later.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	if hook.Calls() != 1 {
		t.Fatalf("webhook calls = %d after a second check, want still 1", hook.Calls())
	}

	// The Worker returns: the incident resolves and one recovery page is sent.
	recordWorkerHeartbeat(t, database, later.Add(40*time.Second))
	if err := svc.Check(t.Context(), later.Add(45*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.alerts.ProcessDueDeliveries(10); err != nil { // the Worker's dispatcher is back in real life
		t.Fatal(err)
	}
	database.Model(&db.Incident{}).Where("monitor_id = ? AND status = ?", coreWorkerMonitorID, "resolved").Count(&incidents)
	if incidents != 1 || hook.Calls() != 2 {
		t.Fatalf("resolved incidents=%d webhook calls=%d, want 1 resolved and a recovery page", incidents, hook.Calls())
	}
}

func TestSelfMonitorStaysQuietWhenNoWorkerHasEverRegistered(t *testing.T) {
	svc, _, database := selfMonitorFixture(t)
	if err := svc.Check(t.Context(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var monitors int64
	database.Model(&db.Monitor{}).Where("id = ?", coreWorkerMonitorID).Count(&monitors)
	if monitors != 0 {
		t.Fatalf("worker monitor created without any worker: %d", monitors)
	}
}

func TestDeadManPingHitsConfiguredURL(t *testing.T) {
	var pings atomic.Int32
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("dead-man ping without a User-Agent")
		}
		pings.Add(1)
	}))
	defer receiver.Close()
	svc, _, _ := selfMonitorFixture(t)

	svc.cfg.DeadManURL = ""
	svc.pingDeadMan(t.Context())
	if pings.Load() != 0 {
		t.Fatal("pinged although no URL is configured")
	}
	svc.cfg.DeadManURL = receiver.URL + "/ping/abc"
	svc.pingDeadMan(t.Context())
	if pings.Load() != 1 {
		t.Fatalf("pings = %d, want 1", pings.Load())
	}
	svc.cfg.DeadManURL = "http://127.0.0.1:1/unreachable"
	svc.pingDeadMan(t.Context()) // must log and return, never panic or block
}
