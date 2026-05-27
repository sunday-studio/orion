package worker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAppRunStopsWhenContextIsCanceled(t *testing.T) {
	database := openWorkerTestDatabase(t)
	app := NewApp(database, logging.NewLogger(), Options{HealthInterval: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := app.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestAppUsesDefaultHealthInterval(t *testing.T) {
	database := openWorkerTestDatabase(t)
	app := NewApp(database, logging.NewLogger(), Options{})

	if app.healthInterval != defaultHealthInterval {
		t.Fatalf("healthInterval = %v, want %v", app.healthInterval, defaultHealthInterval)
	}
}

func TestAppCheckDatabasePassesForOpenDatabase(t *testing.T) {
	database := openWorkerTestDatabase(t)
	app := NewApp(database, logging.NewLogger(), Options{})

	if err := app.checkDatabase(context.Background()); err != nil {
		t.Fatalf("checkDatabase() error = %v", err)
	}
}

func TestRunDueChecksStoresUpReportAndCompletesLease(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("request method = %s, want GET", r.Method)
		}
		return workerHTTPResponse(http.StatusNoContent), nil
	})}

	now := time.Now().UTC()
	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-http-up")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-http-up",
		Kind:            "http",
		ConfigJSON:      `{"url":"https://example.com/health","expected_status":204}`,
		IntervalSeconds: 90,
		TimeoutSeconds:  5,
		NextRunAt:       now.Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID:      "worker-http-test",
		LeaseDuration: time.Minute,
		HTTPClient:    httpClient,
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["runner"] != "core" || payload["status_code"].(float64) != 204 || payload["expected_status"].(float64) != 204 {
		t.Fatalf("payload = %+v, want core HTTP 204 report", payload)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-http-up")
	if completed.LeaseOwner != "" || completed.LeaseExpiresAt != nil {
		t.Fatalf("lease = owner:%q expires:%v, want cleared", completed.LeaseOwner, completed.LeaseExpiresAt)
	}
	if completed.LastRunAt == nil || completed.LastSuccessAt == nil || completed.LastFailureAt != nil {
		t.Fatalf("completed timestamps = run:%v success:%v failure:%v, want successful completion", completed.LastRunAt, completed.LastSuccessAt, completed.LastFailureAt)
	}
	if !completed.NextRunAt.After(*completed.LastRunAt) {
		t.Fatalf("next_run_at = %v, want after last_run_at %v", completed.NextRunAt, completed.LastRunAt)
	}
}

func TestRunDueChecksStoresDownReportForUnexpectedStatus(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusInternalServerError), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-http-down")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-http-down",
		Kind:            "http",
		ConfigJSON:      `{"url":"https://example.com/health","expected_status":200}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-down")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "http_response" || payload["status_code"].(float64) != 500 {
		t.Fatalf("payload = %+v, want http_response failure with status 500", payload)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-http-down")
	if completed.LastFailureAt == nil || completed.LastSuccessAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want failed completion and cleared lease", completed)
	}
}

func TestRunDueChecksStoresDownReportForInvalidConfig(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-http-invalid")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-http-invalid",
		Kind:            "http",
		ConfigJSON:      `{"url":"ftp://example.com"}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-http-test"})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-invalid")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "config" {
		t.Fatalf("payload = %+v, want config failure", payload)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-http-invalid")
	if completed.LastFailureAt == nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want failed completion and cleared lease", completed)
	}
}

func openWorkerTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return database
}

func openWorkerMigratedTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	database := openWorkerTestDatabase(t)
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return database
}

func insertWorkerCoreOwner(t *testing.T, database *gorm.DB) {
	t.Helper()

	agent := db.Agent{
		ID:        "agent_core",
		MachineId: "core",
		Name:      "Orion Core",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "core-token",
		LastSeen:  time.Now().UTC(),
	}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create core owner: %v", err)
	}
}

func insertWorkerMonitor(t *testing.T, database *gorm.DB, monitorID string) {
	t.Helper()

	monitor := db.Monitor{
		ID:                       monitorID,
		AgentID:                  "agent_core",
		Name:                     monitorID,
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		ReportingIntervalSeconds: 60,
	}
	if err := database.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}
}

func insertWorkerCoreMonitorConfig(t *testing.T, database *gorm.DB, config db.CoreMonitorConfig) {
	t.Helper()

	if config.SecretRefJSON == "" {
		config.SecretRefJSON = "{}"
	}
	if config.IntervalSeconds == 0 {
		config.IntervalSeconds = 60
	}
	if config.TimeoutSeconds == 0 {
		config.TimeoutSeconds = 10
	}
	if config.CreatedAt.IsZero() {
		config.CreatedAt = time.Now().UTC()
	}
	if config.UpdatedAt.IsZero() {
		config.UpdatedAt = config.CreatedAt
	}
	if err := database.Create(&config).Error; err != nil {
		t.Fatalf("create core monitor config: %v", err)
	}
}

func loadWorkerMonitorReport(t *testing.T, database *gorm.DB, monitorID string) db.MonitorReport {
	t.Helper()

	var report db.MonitorReport
	if err := database.Where("monitor_id = ?", monitorID).First(&report).Error; err != nil {
		t.Fatalf("load monitor report: %v", err)
	}
	return report
}

func loadWorkerCoreMonitorConfig(t *testing.T, database *gorm.DB, monitorID string) db.CoreMonitorConfig {
	t.Helper()

	var config db.CoreMonitorConfig
	if err := database.Where("monitor_id = ?", monitorID).First(&config).Error; err != nil {
		t.Fatalf("load core monitor config: %v", err)
	}
	return config
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func workerHTTPResponse(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}
}
