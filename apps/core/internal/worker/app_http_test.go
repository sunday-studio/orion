package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"
)

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

	app := NewApp(database, utils.NewLogger(), Options{
		WorkerID:      "worker-http-test",
		LeaseDuration: time.Minute,
		HTTPClient:    httpClient,
	})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]any
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

func TestRunDueChecksRecordsSanitizedFinalTarget(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		response := workerHTTPResponse(http.StatusOK, "ok")
		response.Request = r
		return response, nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-http-final")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-http-final",
		Kind:            "http",
		ConfigJSON:      `{"url":"https://example.com/health?token=secret","expected_status":200}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-final")
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["target_url"] != "https://example.com/health" || payload["final_url"] != "https://example.com/health" || payload["final_host"] != "example.com" {
		t.Fatalf("payload = %+v, want sanitized target and final host", payload)
	}
	if strings.Contains(report.Payload, "token=secret") {
		t.Fatalf("payload leaked query secret: %s", report.Payload)
	}
}

func TestRunDueChecksRejectsBlockedHTTPRedirect(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	calls := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		response := workerHTTPResponse(http.StatusFound)
		response.Header.Set("Location", "http://169.254.169.254/latest/meta-data")
		response.Request = r
		return response, nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-http-redirect")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-http-redirect",
		Kind:            "http",
		ConfigJSON:      `{"url":"https://example.com/start","expected_status":200}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("transport calls = %d, want only initial request before blocked redirect", calls)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-redirect")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "http_request" || payload["ok"] != false {
		t.Fatalf("payload = %+v, want blocked redirect failure", payload)
	}
}

func TestRunDueChecksRejectsBlockedPrivateHTTPRedirect(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	calls := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		response := workerHTTPResponse(http.StatusFound)
		response.Header.Set("Location", "http://10.0.0.5/admin")
		response.Request = r
		return response, nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-http-private-redirect")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-http-private-redirect",
		Kind:            "http",
		ConfigJSON:      `{"url":"https://example.com/start","expected_status":200}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("transport calls = %d, want only initial request before blocked redirect", calls)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-private-redirect")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "http_request" || payload["ok"] != false {
		t.Fatalf("payload = %+v, want blocked private redirect failure", payload)
	}
}

func TestRunDueChecksRejectsBlockedHTTPConfigBeforeTransport(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("transport called for blocked target %s", r.URL.String())
		return workerHTTPResponse(http.StatusOK), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-http-metadata")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-http-metadata",
		Kind:            "http",
		ConfigJSON:      `{"url":"http://169.254.169.254/latest/meta-data?token=secret","expected_status":200}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-metadata")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	if strings.Contains(report.Payload, "token=secret") {
		t.Fatalf("blocked report leaked query secret: %s", report.Payload)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "config" || payload["ok"] != false {
		t.Fatalf("payload = %+v, want config failure before transport", payload)
	}
}

func TestRunDueChecksAllowsPrivateHTTPWhenConfigured(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusOK, "ok"), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-http-private")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-http-private",
		Kind:            "http",
		ConfigJSON:      `{"url":"http://10.0.0.5/health","expected_status":200}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, utils.NewLogger(), Options{
		WorkerID:   "worker-http-test",
		HTTPClient: httpClient,
		Config:     &config.Config{CoreMonitorAllowPrivateTargets: true},
	})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-private")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
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

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-down")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]any
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

func TestRunDueChecksAcceptsExpectedStatusSet(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusAccepted), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-http-status-set")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-http-status-set",
		Kind:            "http",
		ConfigJSON:      `{"url":"https://example.com/job","expected_statuses":[200,202,204]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-status-set")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	statuses, ok := payload["expected_statuses"].([]any)
	if !ok || len(statuses) != 3 || statuses[1].(float64) != 202 {
		t.Fatalf("expected_statuses = %+v, want [200 202 204]", payload["expected_statuses"])
	}
}

func TestRunDueChecksAcceptsExpectedStatusKindAlias(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusAccepted), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-expected-status-alias")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-expected-status-alias",
		Kind:            "expected_status",
		ConfigJSON:      `{"url":"https://example.com/job","expected_status":202}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-expected-status-alias")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "http" || payload["status_code"].(float64) != 202 || payload["expected_status"].(float64) != 202 {
		t.Fatalf("payload = %+v, want expected status alias report", payload)
	}
}

func TestRunDueChecksStoresDownReportForMissingRequiredKeyword(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusOK, "ready=false"), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-http-required-keyword")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-http-required-keyword",
		Kind:            "http",
		ConfigJSON:      `{"url":"https://example.com/health","required_contains":["ready=true"]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, utils.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-required-keyword")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "body_required" || payload["body_sample"] != "ready=false" {
		t.Fatalf("payload = %+v, want bounded required-keyword failure", payload)
	}
}
