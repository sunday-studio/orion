package worker

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"testing"
	"time"
)

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
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if dialedNetwork != "tcp" || dialedAddress != "example.com:443" {
		t.Fatalf("dial target = %s %s, want tcp example.com:443", dialedNetwork, dialedAddress)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-tcp-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]any
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
		ConfigJSON:      `{"host":"example.com","port":65535}`,
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
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	assertTCPFailurePayload(t, database, "monitor-tcp-refused", "connect")
}

func TestRunDueChecksRejectsBlockedTCPHost(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	calls := 0

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-tcp-blocked")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-tcp-blocked",
		Kind:            "tcp",
		ConfigJSON:      `{"host":"169.254.169.254","port":80}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID: "worker-tcp-test",
		TCPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			calls++
			return fakeNetConn{}, nil
		},
	})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("dial calls = %d, want none for blocked TCP target", calls)
	}

	assertTCPFailurePayload(t, database, "monitor-tcp-blocked", "config")
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
	if err := app.runDueChecks(t.Context()); err != nil {
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
		ConfigJSON:      `{"host":"example.com","port":443}`,
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
	if err := app.runDueChecks(t.Context()); err != nil {
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
			if err := app.runDueChecks(t.Context()); err != nil {
				t.Fatalf("runDueChecks() error = %v", err)
			}

			report := loadWorkerMonitorReport(t, database, monitorID)
			if report.Health != "up" {
				t.Fatalf("report health = %q, want up", report.Health)
			}
			var payload map[string]any
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
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-dns-miss")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]any
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
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-dns-lookup-failure")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "lookup" || payload["record_type"] != "A" {
		t.Fatalf("payload = %+v, want DNS lookup failure", payload)
	}
}
