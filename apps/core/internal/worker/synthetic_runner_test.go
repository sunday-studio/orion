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
)

func TestRunDueChecksStoresUpReportForSyntheticAPISteps(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	requests := []string{}
	var seenAuth string
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.Path)
		switch r.URL.Path {
		case "/login":
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read login body: %v", err)
			}
			if string(bodyBytes) != `{"account":"demo"}` {
				t.Fatalf("login body = %q, want substituted account", string(bodyBytes))
			}
			return workerHTTPResponse(http.StatusOK, `{"token":"abc123"}`), nil
		case "/widgets/abc123":
			seenAuth = r.Header.Get("Authorization")
			return workerHTTPResponse(http.StatusOK, `{"ok":true,"id":"abc123","count":2}`), nil
		default:
			return workerHTTPResponse(http.StatusNotFound, `{}`), nil
		}
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-synthetic-up")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID: "monitor-synthetic-up",
		Kind:      "synthetic",
		ConfigJSON: `{
			"variables":{"account":"demo"},
			"steps":[
				{"name":"login","type":"api","url":"https://api.example.com/login","method":"POST","body":"{\"account\":\"{{account}}\"}","expected_status":200,"extract":[{"name":"token","path":"$.token"}]},
				{"id":"view_widget","name":"view widget","type":"api","url":"https://api.example.com/widgets/{{token}}","headers":{"Authorization":"Bearer {{token}}"},"expected_status":200,"json_assertions":[{"path":"$.ok","equals":true},{"path":"$.id","equals":"{{token}}"},{"path":"$.count","equals":2}]}
			]
		}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-synthetic-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if len(requests) != 2 || requests[0] != "/login" || requests[1] != "/widgets/abc123" {
		t.Fatalf("requests = %+v, want login then extracted widget path", requests)
	}
	if seenAuth != "Bearer abc123" {
		t.Fatalf("Authorization = %q, want extracted token", seenAuth)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-synthetic-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "synthetic" || payload["step_count"].(float64) != 2 || payload["completed_steps"].(float64) != 2 || payload["ok"] != true {
		t.Fatalf("payload = %+v, want synthetic success payload", payload)
	}
	assertPayloadContainsString(t, payload["variables"], "token")
	steps := payload["steps"].([]interface{})
	first := steps[0].(map[string]interface{})
	if first["name"] != "login" || first["ok"] != true {
		t.Fatalf("first step = %+v, want successful login step", first)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-synthetic-up")
	if completed.LastSuccessAt == nil || completed.LastFailureAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want successful synthetic completion", completed)
	}
}

func TestRunDueChecksStopsSyntheticOnStepFailure(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	calls := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return workerHTTPResponse(http.StatusInternalServerError, `{"ok":false}`), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-synthetic-failure")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID: "monitor-synthetic-failure",
		Kind:      "synthetic",
		ConfigJSON: `{
			"steps":[
				{"name":"failing status","type":"api","url":"https://api.example.com/fail","expected_status":200},
				{"name":"should not run","type":"api","url":"https://api.example.com/next","expected_status":200}
			]
		}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-synthetic-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want stop after first failure", calls)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-synthetic-failure")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "step_status" || payload["failure_step"] != "failing status" || payload["failure_index"].(float64) != 0 || payload["completed_steps"].(float64) != 0 || payload["ok"] != false {
		t.Fatalf("payload = %+v, want first step status failure", payload)
	}
	steps := payload["steps"].([]interface{})
	if len(steps) != 1 {
		t.Fatalf("steps = %+v, want one recorded failed step", steps)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-synthetic-failure")
	if completed.LastFailureAt == nil || completed.LastSuccessAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want failed synthetic completion", completed)
	}
}

func TestRunDueChecksStoresDownReportForSyntheticAssertionFailure(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusOK, `{"ok":false}`), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-synthetic-assertion")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-synthetic-assertion",
		Kind:            "synthetic",
		ConfigJSON:      `{"steps":[{"name":"assert ok","type":"api","url":"https://api.example.com/health","expected_status":200,"json_assertions":[{"path":"$.ok","equals":true}]}]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-synthetic-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-synthetic-assertion")
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "json_assertion" || payload["ok"] != false {
		t.Fatalf("payload = %+v, want assertion failure", payload)
	}
	steps := payload["steps"].([]interface{})
	first := steps[0].(map[string]interface{})
	if first["assertion_path"] != "$.ok" || first["assertion_actual"] != false {
		t.Fatalf("first step = %+v, want assertion context", first)
	}
}

func TestRunDueChecksStoresDownReportForSyntheticBrowserStepUnsupported(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-synthetic-browser")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-synthetic-browser",
		Kind:            "synthetic",
		ConfigJSON:      `{"steps":[{"name":"open login","type":"browser"}]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-synthetic-test"})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-synthetic-browser")
	if strings.Contains(report.Payload, "password") {
		t.Fatalf("payload unexpectedly contains sensitive fixture text: %s", report.Payload)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "unsupported_step" || payload["failure_step"] != "open login" || payload["ok"] != false {
		t.Fatalf("payload = %+v, want unsupported browser step", payload)
	}
}

func TestRunDueChecksStoresDownReportForSyntheticMissingVariable(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	calls := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return workerHTTPResponse(http.StatusOK, `{}`), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-synthetic-missing-variable")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-synthetic-missing-variable",
		Kind:            "synthetic_multi_step",
		ConfigJSON:      `{"steps":[{"id":"uses_missing","type":"api","url":"https://api.example.com/widgets/{{missing_id}}","expected_status":200}]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-synthetic-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want no HTTP call for missing variable", calls)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-synthetic-missing-variable")
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "variable_substitution" || payload["failure_step_id"] != "uses_missing" || payload["ok"] != false {
		t.Fatalf("payload = %+v, want missing variable failure", payload)
	}
}

func TestRunDueChecksRejectsSyntheticSubstitutedPrivateTarget(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	calls := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return workerHTTPResponse(http.StatusOK, `{}`), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-synthetic-private")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-synthetic-private",
		Kind:            "synthetic",
		ConfigJSON:      `{"variables":{"host":"10.0.0.5"},"steps":[{"name":"private","url":"http://{{host}}/health"}]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-synthetic-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("HTTP calls = %d, want none for blocked substituted target", calls)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-synthetic-private")
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	steps := payload["steps"].([]interface{})
	first := steps[0].(map[string]interface{})
	if payload["failure_stage"] != "config" || first["failure_stage"] != "config" || first["target_url"] != "http://10.0.0.5/health" {
		t.Fatalf("payload = %+v, want blocked substituted target", payload)
	}
}

func TestRunDueChecksStoresTruncatedSyntheticResponseSample(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	body := `{"data":"` + strings.Repeat("x", maxHTTPBodyCaptureLen+128) + `"}`
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusOK, body), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-synthetic-truncated")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-synthetic-truncated",
		Kind:            "synthetic",
		ConfigJSON:      `{"steps":[{"id":"large_response","type":"api","url":"https://api.example.com/large","expected_status":200}]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-synthetic-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-synthetic-truncated")
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	steps := payload["steps"].([]interface{})
	first := steps[0].(map[string]interface{})
	if first["response_truncated"] != true || len(first["response_sample"].(string)) > maxHTTPBodyCaptureLen {
		t.Fatalf("first step = %+v, want truncated response sample", first)
	}
}
