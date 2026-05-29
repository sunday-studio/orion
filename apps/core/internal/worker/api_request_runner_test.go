package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"strings"
	"testing"
	"time"
)

func TestRunDueChecksStoresUpReportForAPIRequest(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	var seenMethod, seenBody, seenAuth string
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seenMethod = r.Method
		seenAuth = r.Header.Get("Authorization")
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		seenBody = string(bodyBytes)
		return workerHTTPResponse(http.StatusCreated, `{"ok":true,"data":{"id":42}}`), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-api-up")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-api-up",
		Kind:            "api_request",
		ConfigJSON:      `{"url":"https://api.example.com/widgets","method":"POST","headers":{"X-Trace":"trace-1"},"body":"{\"name\":\"widget\"}","expected_status":201,"json_assertions":[{"path":"$.ok","equals":true},{"path":"$.data.id","equals":42}]}`,
		SecretRefJSON:   `{"headers":{"Authorization":"Bearer super-secret"}}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-api-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if seenMethod != http.MethodPost || seenAuth != "Bearer super-secret" || seenBody != `{"name":"widget"}` {
		t.Fatalf("request method=%q auth=%q body=%q, want configured request with secret header", seenMethod, seenAuth, seenBody)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-api-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "api_request" || payload["status_code"].(float64) != 201 || payload["ok"] != true {
		t.Fatalf("payload = %+v, want API success payload", payload)
	}
	assertPayloadContainsString(t, payload["request_headers"], "X-Trace")
	assertPayloadContainsString(t, payload["redacted_headers"], "Authorization")
	if strings.Contains(report.Payload, "super-secret") {
		t.Fatalf("payload leaked secret header: %s", report.Payload)
	}
}

func TestRunDueChecksStoresDownReportForAPIJSONAssertionFailure(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusOK, `{"ok":false}`), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-api-assertion")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-api-assertion",
		Kind:            "api_request",
		ConfigJSON:      `{"url":"https://api.example.com/health","expected_status":200,"json_assertions":[{"path":"$.ok","equals":true}]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-api-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-api-assertion")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "json_assertion" || payload["assertion_path"] != "$.ok" || payload["assertion_actual"] != false {
		t.Fatalf("payload = %+v, want JSON assertion failure", payload)
	}
}

func TestRunDueChecksStoresDownReportForAPITransportFailure(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection reset")
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-api-transport")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-api-transport",
		Kind:            "api_request",
		ConfigJSON:      `{"url":"https://api.example.com/health","expected_status":200}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-api-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-api-transport")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "transport" || payload["ok"] != false {
		t.Fatalf("payload = %+v, want transport failure", payload)
	}
}
