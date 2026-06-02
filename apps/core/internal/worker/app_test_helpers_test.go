package worker

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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

func countWorkerMonitorReports(t *testing.T, database *gorm.DB, monitorID string) int64 {
	t.Helper()

	var count int64
	if err := database.Model(&db.MonitorReport{}).Where("monitor_id = ?", monitorID).Count(&count).Error; err != nil {
		t.Fatalf("count monitor reports: %v", err)
	}
	return count
}

func countWorkerIncidents(t *testing.T, database *gorm.DB, monitorID string) int64 {
	t.Helper()

	var count int64
	if err := database.Model(&db.Incident{}).Where("monitor_id = ?", monitorID).Count(&count).Error; err != nil {
		t.Fatalf("count incidents: %v", err)
	}
	return count
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
	var payload map[string]any
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

func (fakeNetConn) Read(b []byte) (int, error) { return 0, io.EOF }

func (fakeNetConn) Write(b []byte) (int, error) { return len(b), nil }

func (fakeNetConn) Close() error { return nil }

func (fakeNetConn) LocalAddr() net.Addr { return fakeNetAddr("local") }

func (fakeNetConn) RemoteAddr() net.Addr { return fakeNetAddr("remote") }

func (fakeNetConn) SetDeadline(t time.Time) error { return nil }

func (fakeNetConn) SetReadDeadline(t time.Time) error { return nil }

func (fakeNetConn) SetWriteDeadline(t time.Time) error { return nil }

type fakeNetAddr string

func (a fakeNetAddr) Network() string { return string(a) }

func (a fakeNetAddr) String() string { return string(a) }

type timeoutTestError struct{}

func (timeoutTestError) Error() string { return "dial timeout" }

func (timeoutTestError) Timeout() bool { return true }

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

func assertPayloadContainsString(t *testing.T, raw any, expected string) {
	t.Helper()

	values, ok := raw.([]any)
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
