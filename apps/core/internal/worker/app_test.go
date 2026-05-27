package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
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

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-status-set")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	statuses, ok := payload["expected_statuses"].([]interface{})
	if !ok || len(statuses) != 3 || statuses[1].(float64) != 202 {
		t.Fatalf("expected_statuses = %+v, want [200 202 204]", payload["expected_statuses"])
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

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-http-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-required-keyword")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "body_required" || payload["body_sample"] != "ready=false" {
		t.Fatalf("payload = %+v, want bounded required-keyword failure", payload)
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
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-http-forbidden-keyword")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
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

func TestRunDueChecksStoresUpReportForTCPConnection(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	var dialedNetwork, dialedAddress string
	tcpDialContext := func(ctx context.Context, network string, address string) (net.Conn, error) {
		dialedNetwork = network
		dialedAddress = address
		return fakeNetConn{}, nil
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-tcp-up")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-tcp-up",
		Kind:            "tcp",
		ConfigJSON:      `{"host":"example.com","port":443}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-tcp-test", TCPDialContext: tcpDialContext})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if dialedNetwork != "tcp" || dialedAddress != "example.com:443" {
		t.Fatalf("dial target = %s %s, want tcp example.com:443", dialedNetwork, dialedAddress)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-tcp-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "tcp" || payload["host"] != "example.com" || payload["port"].(float64) != 443 || payload["ok"] != true {
		t.Fatalf("payload = %+v, want tcp success payload", payload)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-tcp-up")
	if completed.LastSuccessAt == nil || completed.LastFailureAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want successful TCP completion", completed)
	}
}

func TestRunDueChecksStoresDownReportForTCPRefusedConnection(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-tcp-refused")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-tcp-refused",
		Kind:            "tcp_port",
		ConfigJSON:      `{"host":"127.0.0.1","port":65535}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID: "worker-tcp-test",
		TCPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			return nil, errors.New("connect: connection refused")
		},
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	assertTCPFailurePayload(t, database, "monitor-tcp-refused", "connect")
}

func TestRunDueChecksStoresDownReportForTCPDNSFailure(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-tcp-dns")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-tcp-dns",
		Kind:            "tcp",
		ConfigJSON:      `{"host":"missing.example.invalid","port":443}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID: "worker-tcp-test",
		TCPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			return nil, &net.DNSError{Name: "missing.example.invalid", Err: "no such host"}
		},
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	assertTCPFailurePayload(t, database, "monitor-tcp-dns", "dns")
}

func TestRunDueChecksStoresDownReportForTCPTimeout(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-tcp-timeout")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-tcp-timeout",
		Kind:            "tcp",
		ConfigJSON:      `{"host":"10.0.0.1","port":443}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  1,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID: "worker-tcp-test",
		TCPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			return nil, timeoutTestError{}
		},
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	assertTCPFailurePayload(t, database, "monitor-tcp-timeout", "timeout")
}

func TestRunDueChecksStoresUpReportForDNSRecords(t *testing.T) {
	cases := []struct {
		name       string
		recordType string
		config     string
		expected   string
	}{
		{
			name:       "a",
			recordType: "A",
			config:     `{"host":"example.com","record_type":"A","expected_values":["192.0.2.10"]}`,
			expected:   "192.0.2.10",
		},
		{
			name:       "aaaa",
			recordType: "AAAA",
			config:     `{"host":"example.com","record_type":"AAAA","expected_values":["2001:db8::10"]}`,
			expected:   "2001:db8::10",
		},
		{
			name:       "cname",
			recordType: "CNAME",
			config:     `{"host":"alias.example.com","record_type":"CNAME","expected_values":["target.example.com"]}`,
			expected:   "target.example.com",
		},
		{
			name:       "txt",
			recordType: "TXT",
			config:     `{"host":"example.com","record_type":"TXT","expected_values":["v=spf1 -all"]}`,
			expected:   "v=spf1 -all",
		},
		{
			name:       "mx",
			recordType: "MX",
			config:     `{"host":"example.com","record_type":"MX","expected_values":["mail.example.com"]}`,
			expected:   "mail.example.com",
		},
		{
			name:       "ns",
			recordType: "NS",
			config:     `{"host":"example.com","record_type":"NS","expected_values":["ns1.example.com"]}`,
			expected:   "ns1.example.com",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			database := openWorkerMigratedTestDatabase(t)
			resolver := fakeDNSResolver{
				ipAnswers: map[string][]net.IPAddr{
					"example.com": {
						{IP: net.ParseIP("192.0.2.10")},
						{IP: net.ParseIP("2001:db8::10")},
					},
				},
				cnameAnswers: map[string]string{"alias.example.com": "target.example.com."},
				txtAnswers:   map[string][]string{"example.com": {"v=spf1 -all"}},
				mxAnswers:    map[string][]*net.MX{"example.com": {{Host: "mail.example.com.", Pref: 10}}},
				nsAnswers:    map[string][]*net.NS{"example.com": {{Host: "ns1.example.com."}}},
			}
			monitorID := "monitor-dns-" + tc.name

			insertWorkerCoreOwner(t, database)
			insertWorkerMonitor(t, database, monitorID)
			insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
				MonitorID:       monitorID,
				Kind:            "dns",
				ConfigJSON:      tc.config,
				IntervalSeconds: 60,
				TimeoutSeconds:  5,
				NextRunAt:       time.Now().UTC().Add(-time.Minute),
			})

			app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-dns-test", DNSResolver: resolver})
			if err := app.runDueChecks(context.Background()); err != nil {
				t.Fatalf("runDueChecks() error = %v", err)
			}

			report := loadWorkerMonitorReport(t, database, monitorID)
			if report.Health != "up" {
				t.Fatalf("report health = %q, want up", report.Health)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
				t.Fatalf("unmarshal report payload: %v", err)
			}
			if payload["type"] != "dns" || payload["record_type"] != tc.recordType || payload["ok"] != true {
				t.Fatalf("payload = %+v, want DNS %s success", payload, tc.recordType)
			}
			assertPayloadContainsString(t, payload["answers"], tc.expected)
		})
	}
}

func TestRunDueChecksStoresDownReportForDNSExpectedValueMiss(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	resolver := fakeDNSResolver{
		txtAnswers: map[string][]string{"example.com": {"actual-token"}},
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-dns-miss")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-dns-miss",
		Kind:            "dns",
		ConfigJSON:      `{"host":"example.com","record_type":"TXT","expected_values":["required-token"]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-dns-test", DNSResolver: resolver})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-dns-miss")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "expected_values" || payload["ok"] != false {
		t.Fatalf("payload = %+v, want expected_values failure", payload)
	}
	assertPayloadContainsString(t, payload["missing_values"], "required-token")
}

func TestRunDueChecksStoresDownReportForDNSLookupFailure(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	resolver := fakeDNSResolver{
		lookupErr: &net.DNSError{Name: "missing.example.invalid", Err: "no such host"},
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-dns-lookup-failure")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-dns-lookup-failure",
		Kind:            "dns",
		ConfigJSON:      `{"host":"missing.example.invalid","record_type":"A"}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-dns-test", DNSResolver: resolver})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-dns-lookup-failure")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "lookup" || payload["record_type"] != "A" {
		t.Fatalf("payload = %+v, want DNS lookup failure", payload)
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

func workerHTTPResponse(statusCode int, body ...string) *http.Response {
	responseBody := ""
	if len(body) > 0 {
		responseBody = body[0]
	}
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
		Header:     make(http.Header),
	}
}

func assertTCPFailurePayload(t *testing.T, database *gorm.DB, monitorID string, failureStage string) {
	t.Helper()

	report := loadWorkerMonitorReport(t, database, monitorID)
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "tcp" || payload["failure_stage"] != failureStage || payload["ok"] != false {
		t.Fatalf("payload = %+v, want tcp %s failure", payload, failureStage)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, monitorID)
	if completed.LastFailureAt == nil || completed.LastSuccessAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want failed TCP completion", completed)
	}
}

type fakeNetConn struct{}

func (fakeNetConn) Read(b []byte) (int, error)         { return 0, io.EOF }
func (fakeNetConn) Write(b []byte) (int, error)        { return len(b), nil }
func (fakeNetConn) Close() error                       { return nil }
func (fakeNetConn) LocalAddr() net.Addr                { return fakeNetAddr("local") }
func (fakeNetConn) RemoteAddr() net.Addr               { return fakeNetAddr("remote") }
func (fakeNetConn) SetDeadline(t time.Time) error      { return nil }
func (fakeNetConn) SetReadDeadline(t time.Time) error  { return nil }
func (fakeNetConn) SetWriteDeadline(t time.Time) error { return nil }

type fakeNetAddr string

func (a fakeNetAddr) Network() string { return string(a) }
func (a fakeNetAddr) String() string  { return string(a) }

type timeoutTestError struct{}

func (timeoutTestError) Error() string   { return "dial timeout" }
func (timeoutTestError) Timeout() bool   { return true }
func (timeoutTestError) Temporary() bool { return true }

type fakeDNSResolver struct {
	ipAnswers    map[string][]net.IPAddr
	cnameAnswers map[string]string
	txtAnswers   map[string][]string
	mxAnswers    map[string][]*net.MX
	nsAnswers    map[string][]*net.NS
	lookupErr    error
}

func (r fakeDNSResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if r.lookupErr != nil {
		return nil, r.lookupErr
	}
	answers, ok := r.ipAnswers[host]
	if !ok {
		return nil, &net.DNSError{Name: host, Err: "no such host"}
	}
	return answers, nil
}

func (r fakeDNSResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	if r.lookupErr != nil {
		return "", r.lookupErr
	}
	answer, ok := r.cnameAnswers[host]
	if !ok {
		return "", &net.DNSError{Name: host, Err: "no such host"}
	}
	return answer, nil
}

func (r fakeDNSResolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	if r.lookupErr != nil {
		return nil, r.lookupErr
	}
	answers, ok := r.txtAnswers[host]
	if !ok {
		return nil, &net.DNSError{Name: host, Err: "no such host"}
	}
	return answers, nil
}

func (r fakeDNSResolver) LookupMX(ctx context.Context, host string) ([]*net.MX, error) {
	if r.lookupErr != nil {
		return nil, r.lookupErr
	}
	answers, ok := r.mxAnswers[host]
	if !ok {
		return nil, &net.DNSError{Name: host, Err: "no such host"}
	}
	return answers, nil
}

func (r fakeDNSResolver) LookupNS(ctx context.Context, host string) ([]*net.NS, error) {
	if r.lookupErr != nil {
		return nil, r.lookupErr
	}
	answers, ok := r.nsAnswers[host]
	if !ok {
		return nil, &net.DNSError{Name: host, Err: "no such host"}
	}
	return answers, nil
}

func assertPayloadContainsString(t *testing.T, raw interface{}, expected string) {
	t.Helper()

	values, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("payload values = %T(%v), want array containing %q", raw, raw, expected)
	}
	for _, value := range values {
		if value == expected {
			return
		}
	}
	t.Fatalf("payload values = %+v, want %q", values, expected)
}
