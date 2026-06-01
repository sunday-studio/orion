package worker

import (
	"encoding/json"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"testing"
	"time"
)

func TestRunDueChecksAcceptsHTTPKeywordKindAlias(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusOK, "ready=true"), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-http-keyword-alias")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-http-keyword-alias",
		Kind:            "http_keyword",
		ConfigJSON:      `{"url":"https://example.com/health","required_contains":["ready=true"]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-keyword-alias")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "http" || payload["status_code"].(float64) != 200 || payload["ok"] != true {
		t.Fatalf("payload = %+v, want keyword alias report", payload)
	}
}

func TestRunDueChecksStoresDownReportForForbiddenKeyword(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusOK, "status=ok debug=true"), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-http-forbidden-keyword")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-http-forbidden-keyword",
		Kind:            "http",
		ConfigJSON:      `{"url":"https://example.com/health","forbidden_contains":["debug=true"]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-forbidden-keyword")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "body_forbidden" || payload["body_sample"] != "status=ok debug=true" {
		t.Fatalf("payload = %+v, want bounded forbidden-keyword failure", payload)
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
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-invalid")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]any
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
