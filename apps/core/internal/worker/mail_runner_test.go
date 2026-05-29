package worker

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestRunDueChecksStoresUpReportForSMTPMonitor(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	conn := newScriptedMailConn("220 mx.example ESMTP ready\r\n250-mx.example\r\n250-PIPELINING\r\n250 STARTTLS\r\n")
	var dialedAddress string

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-smtp-up")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-smtp-up",
		Kind:            "smtp",
		ConfigJSON:      `{"host":"mx.example","expected_banner":"ESMTP","expected_capabilities":["PIPELINING","STARTTLS"]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID: "worker-mail-test",
		TCPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			dialedAddress = address
			return conn, nil
		},
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if dialedAddress != "mx.example:25" {
		t.Fatalf("dialed address = %q, want mx.example:25", dialedAddress)
	}
	if !strings.Contains(conn.Written(), "EHLO orion.local") {
		t.Fatalf("written commands = %q, want EHLO", conn.Written())
	}

	report := loadWorkerMonitorReport(t, database, "monitor-smtp-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "mail" || payload["protocol"] != "smtp" || payload["tls_mode"] != "none" || payload["auth_attempted"] != false || payload["ok"] != true {
		t.Fatalf("payload = %+v, want SMTP success payload", payload)
	}
	assertPayloadContainsString(t, payload["capabilities"], "PIPELINING")

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-smtp-up")
	if completed.LastSuccessAt == nil || completed.LastFailureAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want successful SMTP completion", completed)
	}
}

func TestRunDueChecksStoresUpReportForIMAPMonitor(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	conn := newScriptedMailConn("* OK IMAP4rev1 Service Ready\r\n* CAPABILITY IMAP4rev1 STARTTLS IDLE\r\nA001 OK CAPABILITY completed\r\n")

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-imap-up")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-imap-up",
		Kind:            "imap",
		ConfigJSON:      `{"host":"imap.example","expected_capabilities":["IMAP4rev1","IDLE"]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID: "worker-mail-test",
		TCPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			return conn, nil
		},
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if !strings.Contains(conn.Written(), "A001 CAPABILITY") {
		t.Fatalf("written commands = %q, want IMAP CAPABILITY", conn.Written())
	}

	report := loadWorkerMonitorReport(t, database, "monitor-imap-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["protocol"] != "imap" || payload["port"].(float64) != 143 || payload["ok"] != true {
		t.Fatalf("payload = %+v, want IMAP success payload", payload)
	}
	assertPayloadContainsString(t, payload["capabilities"], "IDLE")
}

func TestRunDueChecksStoresUpReportForPOPMonitor(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	conn := newScriptedMailConn("+OK POP3 server ready\r\n+OK Capability list follows\r\nUSER\r\nUIDL\r\nSTLS\r\n.\r\n")

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-pop-up")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-pop-up",
		Kind:            "pop3",
		ConfigJSON:      `{"host":"pop.example","expected_capabilities":["USER","UIDL"]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID: "worker-mail-test",
		TCPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			return conn, nil
		},
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if !strings.Contains(conn.Written(), "CAPA") {
		t.Fatalf("written commands = %q, want POP CAPA", conn.Written())
	}

	report := loadWorkerMonitorReport(t, database, "monitor-pop-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["protocol"] != "pop" || payload["port"].(float64) != 110 || payload["auth_enabled"] != false || payload["ok"] != true {
		t.Fatalf("payload = %+v, want POP success payload", payload)
	}
	assertPayloadContainsString(t, payload["capabilities"], "UIDL")
}

func TestRunDueChecksStoresDownReportForMailMissingCapability(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	conn := newScriptedMailConn("220 mx.example ESMTP ready\r\n250 mx.example\r\n")

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-mail-capability")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-mail-capability",
		Kind:            "mail",
		ConfigJSON:      `{"protocol":"smtp","host":"mx.example","expected_capabilities":["STARTTLS"]}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID: "worker-mail-test",
		TCPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			return conn, nil
		},
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	assertMailFailurePayload(t, database, "monitor-mail-capability", "capability")
}

func TestRunDueChecksStoresDownReportForMailAuthEnabledConfig(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-mail-auth")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-mail-auth",
		Kind:            "smtp",
		ConfigJSON:      `{"host":"mx.example","auth_enabled":true}`,
		SecretRefJSON:   `{"username":"user@example.com","password":"super-secret"}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-mail-test"})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-mail-auth")
	if strings.Contains(report.Payload, "super-secret") {
		t.Fatalf("mail auth config payload leaked secret: %s", report.Payload)
	}
	assertMailFailurePayload(t, database, "monitor-mail-auth", "config")
}

func TestRunDueChecksStoresDownReportForMailTimeout(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-mail-timeout")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-mail-timeout",
		Kind:            "smtp",
		ConfigJSON:      `{"host":"mx.example"}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  1,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID: "worker-mail-test",
		TCPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			return nil, timeoutTestError{}
		},
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	assertMailFailurePayload(t, database, "monitor-mail-timeout", "timeout")
}

func assertMailFailurePayload(t *testing.T, database *gorm.DB, monitorID string, failureStage string) {
	t.Helper()

	report := loadWorkerMonitorReport(t, database, monitorID)
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "mail" || payload["failure_stage"] != failureStage || payload["auth_attempted"] != false || payload["ok"] != false {
		t.Fatalf("payload = %+v, want mail %s failure", payload, failureStage)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, monitorID)
	if completed.LastFailureAt == nil || completed.LastSuccessAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want failed mail completion", completed)
	}
}

type scriptedMailConn struct {
	reader *strings.Reader
	writes strings.Builder
	closed bool
}

func newScriptedMailConn(script string) *scriptedMailConn {
	return &scriptedMailConn{reader: strings.NewReader(script)}
}

func (c *scriptedMailConn) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

func (c *scriptedMailConn) Write(b []byte) (int, error) {
	return c.writes.Write(b)
}

func (c *scriptedMailConn) Close() error {
	c.closed = true
	return nil
}

func (c *scriptedMailConn) Written() string {
	return c.writes.String()
}

func (c *scriptedMailConn) LocalAddr() net.Addr                { return fakeNetAddr("local") }
func (c *scriptedMailConn) RemoteAddr() net.Addr               { return fakeNetAddr("remote") }
func (c *scriptedMailConn) SetDeadline(t time.Time) error      { return nil }
func (c *scriptedMailConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *scriptedMailConn) SetWriteDeadline(t time.Time) error { return nil }

var _ net.Conn = (*scriptedMailConn)(nil)
var _ io.Reader = (*scriptedMailConn)(nil)
