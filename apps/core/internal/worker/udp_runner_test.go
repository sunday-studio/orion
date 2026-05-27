package worker

import (
	"context"
	"encoding/json"
	"net"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestRunDueChecksStoresUpReportForUDPResponse(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	conn := &fakeUDPConn{response: "pong"}
	var dialedNetwork, dialedAddress string
	udpDialContext := func(ctx context.Context, network string, address string) (net.Conn, error) {
		dialedNetwork = network
		dialedAddress = address
		return conn, nil
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-udp-up")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-udp-up",
		Kind:            "udp",
		ConfigJSON:      `{"host":"example.com","port":8125,"payload":"ping","expected_response":"pong"}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-udp-test", UDPDialContext: udpDialContext})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if dialedNetwork != "udp" || dialedAddress != "example.com:8125" {
		t.Fatalf("dial target = %s %s, want udp example.com:8125", dialedNetwork, dialedAddress)
	}
	if string(conn.written) != "ping" || conn.closed != true {
		t.Fatalf("conn written=%q closed=%v, want ping and closed", string(conn.written), conn.closed)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-udp-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "udp" || payload["response"] != "pong" || payload["payload_bytes"].(float64) != 4 || payload["ok"] != true {
		t.Fatalf("payload = %+v, want UDP success payload", payload)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-udp-up")
	if completed.LastSuccessAt == nil || completed.LastFailureAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want successful UDP completion", completed)
	}
}

func TestRunDueChecksStoresDownReportForUDPTimeout(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-udp-timeout")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-udp-timeout",
		Kind:            "udp",
		ConfigJSON:      `{"host":"example.com","port":8125,"payload":"ping","expected_response":"pong"}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  1,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID: "worker-udp-test",
		UDPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			return &fakeUDPConn{readErr: timeoutTestError{}}, nil
		},
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	assertUDPFailurePayload(t, database, "monitor-udp-timeout", "timeout")
}

func TestRunDueChecksStoresDownReportForUDPResponseMismatch(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-udp-mismatch")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-udp-mismatch",
		Kind:            "udp",
		ConfigJSON:      `{"host":"example.com","port":8125,"payload":"ping","expected_response":"pong"}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  1,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID: "worker-udp-test",
		UDPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			return &fakeUDPConn{response: "nope"}, nil
		},
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	assertUDPFailurePayload(t, database, "monitor-udp-mismatch", "response_mismatch")
}

func TestRunDueChecksStoresDownReportForUDPNoResponseConfig(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-udp-config")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-udp-config",
		Kind:            "udp",
		ConfigJSON:      `{"host":"example.com","port":8125,"payload":"ping"}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  1,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-udp-test"})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	assertUDPFailurePayload(t, database, "monitor-udp-config", "config")
}

func assertUDPFailurePayload(t *testing.T, database *gorm.DB, monitorID string, failureStage string) {
	t.Helper()

	report := loadWorkerMonitorReport(t, database, monitorID)
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "udp" || payload["failure_stage"] != failureStage || payload["ok"] != false {
		t.Fatalf("payload = %+v, want UDP %s failure", payload, failureStage)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, monitorID)
	if completed.LastFailureAt == nil || completed.LastSuccessAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want failed UDP completion", completed)
	}
}

type fakeUDPConn struct {
	response string
	readErr  error
	written  []byte
	closed   bool
}

func (c *fakeUDPConn) Read(b []byte) (int, error) {
	if c.readErr != nil {
		return 0, c.readErr
	}
	return strings.NewReader(c.response).Read(b)
}

func (c *fakeUDPConn) Write(b []byte) (int, error) {
	c.written = append([]byte(nil), b...)
	return len(b), nil
}

func (c *fakeUDPConn) Close() error {
	c.closed = true
	return nil
}

func (c *fakeUDPConn) LocalAddr() net.Addr                { return fakeNetAddr("local") }
func (c *fakeUDPConn) RemoteAddr() net.Addr               { return fakeNetAddr("remote") }
func (c *fakeUDPConn) SetDeadline(t time.Time) error      { return nil }
func (c *fakeUDPConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *fakeUDPConn) SetWriteDeadline(t time.Time) error { return nil }

var _ net.Conn = (*fakeUDPConn)(nil)
