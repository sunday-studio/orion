package worker

import (
	"context"
	"encoding/json"
	"errors"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"strings"
	"testing"
	"time"
)

func TestRunDueChecksStoresUpReportForPlaywrightTransaction(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	var received playwrightTransactionRequest
	playwrightRun := func(ctx context.Context, request playwrightTransactionRequest) (playwrightTransactionRunResult, error) {
		received = request
		return playwrightTransactionRunResult{
			Ok: true,
			StepResults: []playwrightStepResult{
				{Index: 0, Name: "open login", Action: "goto", Ok: true, DurationMS: 12},
				{Index: 1, Name: "fill login", Action: "fill", Ok: true, DurationMS: 8},
			},
			FailureIndex: -1,
		}, nil
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-playwright-up")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID: "monitor-playwright-up",
		Kind:      "playwright_transaction",
		ConfigJSON: `{
			"start_url":"https://app.example.com/login",
			"browser":"chromium",
			"viewport":{"width":1440,"height":900},
			"screenshot":"on_failure",
			"steps":[
				{"name":"open login","action":"goto","url":"https://app.example.com/login"},
				{"name":"fill login","action":"fill","selector":"#password","value":"{{secret.password}}"}
			]
		}`,
		SecretRefJSON:   `{"variables":{"password":"super-secret"}}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-playwright-test", PlaywrightRun: playwrightRun})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if received.Config.Browser != "chromium" || received.Config.Viewport.Width != 1440 || received.Secrets.Variables["password"] != "super-secret" {
		t.Fatalf("received request = %+v, want normalized config and secret variables", received)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-playwright-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "playwright_transaction" || payload["browser"] != "chromium" || payload["step_count"].(float64) != 2 || payload["completed_steps"].(float64) != 2 || payload["ok"] != true {
		t.Fatalf("payload = %+v, want Playwright success payload", payload)
	}
	if payload["target_url"] != "https://app.example.com/login" || payload["screenshot_on_failure"] != true {
		t.Fatalf("payload = %+v, want target URL and screenshot policy", payload)
	}
	assertPayloadContainsString(t, payload["redacted_variables"], "password")
	assertPayloadContainsString(t, payload["redacted_values"], "password")
	if strings.Contains(report.Payload, "super-secret") {
		t.Fatalf("payload leaked secret value: %s", report.Payload)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-playwright-up")
	if completed.LastSuccessAt == nil || completed.LastFailureAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want successful Playwright completion", completed)
	}
}

func TestRunDueChecksStoresDownReportForPlaywrightFailureArtifact(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	playwrightRun := func(ctx context.Context, request playwrightTransactionRequest) (playwrightTransactionRunResult, error) {
		return playwrightTransactionRunResult{
			Ok:           false,
			FailureStage: "step",
			FailureStep:  "submit login",
			FailureIndex: 1,
			Error:        "locator contained super-secret",
			StepResults: []playwrightStepResult{
				{Index: 0, Name: "open login", Action: "goto", Ok: true, DurationMS: 12},
				{Index: 1, Name: "submit login", Action: "click", Ok: false, DurationMS: 9, FailureStage: "step", Error: "could not click super-secret"},
			},
			Artifacts: []playwrightArtifact{
				{Name: "submit-login-failure", Type: "screenshot", MimeType: "image/png", DataBase64: strings.Repeat("a", 96), Bytes: 96},
			},
		}, nil
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-playwright-fail")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-playwright-fail",
		Kind:            "playwright",
		ConfigJSON:      `{"url":"https://app.example.com/login","artifact_limit_bytes":32,"steps":[{"name":"open login","action":"goto","url":"https://app.example.com/login"},{"name":"submit login","action":"click","selector":"button"}]}`,
		SecretRefJSON:   `{"variables":{"password":"super-secret"}}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-playwright-test", PlaywrightRun: playwrightRun})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-playwright-fail")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	if strings.Contains(report.Payload, "super-secret") {
		t.Fatalf("payload leaked secret value: %s", report.Payload)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "step" || payload["failure_step"] != "submit login" || payload["failure_index"].(float64) != 1 || payload["artifact_bytes"].(float64) != 32 || payload["ok"] != false {
		t.Fatalf("payload = %+v, want bounded Playwright failure payload", payload)
	}
	artifacts := payload["artifacts"].([]interface{})
	firstArtifact := artifacts[0].(map[string]interface{})
	if firstArtifact["truncated"] != true || len(firstArtifact["data_base64"].(string)) != 32 {
		t.Fatalf("artifact = %+v, want truncated screenshot artifact", firstArtifact)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-playwright-fail")
	if completed.LastFailureAt == nil || completed.LastSuccessAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want failed Playwright completion", completed)
	}
}

func TestRunDueChecksStoresDownReportForPlaywrightRuntimeFailure(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	playwrightRun := func(ctx context.Context, request playwrightTransactionRequest) (playwrightTransactionRunResult, error) {
		return playwrightTransactionRunResult{}, errors.New("playwright module not installed")
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-playwright-runtime")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-playwright-runtime",
		Kind:            "playwright",
		ConfigJSON:      `{"url":"https://app.example.com/login"}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-playwright-test", PlaywrightRun: playwrightRun})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-playwright-runtime")
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "runtime_unavailable" || payload["ok"] != false {
		t.Fatalf("payload = %+v, want runtime unavailable payload", payload)
	}
}

func TestRunDueChecksStoresDownReportForPlaywrightInvalidConfig(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	called := false
	playwrightRun := func(ctx context.Context, request playwrightTransactionRequest) (playwrightTransactionRunResult, error) {
		called = true
		return playwrightTransactionRunResult{Ok: true}, nil
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-playwright-config")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-playwright-config",
		Kind:            "playwright_transaction",
		ConfigJSON:      `{"steps":[{"name":"bad scheme","action":"goto","url":"ftp://app.example.com/login"}]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-playwright-test", PlaywrightRun: playwrightRun})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if called {
		t.Fatalf("playwright runner was called for invalid config")
	}

	report := loadWorkerMonitorReport(t, database, "monitor-playwright-config")
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "config" || payload["ok"] != false {
		t.Fatalf("payload = %+v, want config failure payload", payload)
	}
}

func TestRunDueChecksRejectsPlaywrightPrivateTarget(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	called := false
	playwrightRun := func(ctx context.Context, request playwrightTransactionRequest) (playwrightTransactionRunResult, error) {
		called = true
		return playwrightTransactionRunResult{Ok: true}, nil
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-playwright-private")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-playwright-private",
		Kind:            "playwright",
		ConfigJSON:      `{"url":"http://10.0.0.5/login"}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-playwright-test", PlaywrightRun: playwrightRun})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if called {
		t.Fatal("playwright runner was called for blocked target")
	}

	report := loadWorkerMonitorReport(t, database, "monitor-playwright-private")
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "config" || payload["target_url"] != "http://10.0.0.5/login" || payload["ok"] != false {
		t.Fatalf("payload = %+v, want blocked target config failure", payload)
	}
}
