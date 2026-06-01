package worker

import (
	"context"
	"encoding/json"
	"net"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestRunDueChecksStoresUpReportForPingTCPReachability(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	var dialedNetwork, dialedAddress string
	tcpDialContext := func(ctx context.Context, network string, address string) (net.Conn, error) {
		dialedNetwork = network
		dialedAddress = address
		return fakeNetConn{}, nil
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-ping-up")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-ping-up",
		Kind:            "ping",
		ConfigJSON:      `{"host":"example.com","port":443}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-ping-test", TCPDialContext: tcpDialContext})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if dialedNetwork != "tcp" || dialedAddress != "example.com:443" {
		t.Fatalf("dial target = %s %s, want tcp example.com:443", dialedNetwork, dialedAddress)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-ping-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "ping" || payload["method"] != "tcp" || payload["strategy"] != "tcp_connect" || payload["fallback_strategy"] != "tcp_connect" || payload["reachable"] != true || payload["requires_privilege"] != false || payload["ok"] != true {
		t.Fatalf("payload = %+v, want ping TCP success payload", payload)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-ping-up")
	if completed.LastSuccessAt == nil || completed.LastFailureAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want successful ping completion", completed)
	}
}

func TestRunDueChecksStoresDownReportForPingTimeout(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-ping-timeout")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-ping-timeout",
		Kind:            "ping",
		ConfigJSON:      `{"host":"example.com","port":443}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  1,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID: "worker-ping-test",
		TCPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			return nil, timeoutTestError{}
		},
	})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	assertPingFailurePayload(t, database, "monitor-ping-timeout", "timeout")
}

func TestRunDueChecksStoresDownReportForPingICMPPermission(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-ping-icmp")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-ping-icmp",
		Kind:            "ping",
		ConfigJSON:      `{"host":"example.com","method":"icmp"}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  1,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-ping-test"})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-ping-icmp")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "ping" || payload["method"] != "icmp" || payload["strategy"] != "icmp" || payload["failure_stage"] != "permission" || payload["fallback_strategy"] != "none" || payload["icmp_supported"] != false || payload["requires_privilege"] != true || payload["unsupported"] != true || payload["ok"] != false {
		t.Fatalf("payload = %+v, want ICMP permission failure payload", payload)
	}
}

func assertPingFailurePayload(t *testing.T, database *gorm.DB, monitorID string, failureStage string) {
	t.Helper()

	report := loadWorkerMonitorReport(t, database, monitorID)
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "ping" || payload["failure_stage"] != failureStage || payload["ok"] != false {
		t.Fatalf("payload = %+v, want ping %s failure", payload, failureStage)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, monitorID)
	if completed.LastFailureAt == nil || completed.LastSuccessAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want failed ping completion", completed)
	}
}
