package worker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"testing"
	"time"
)

func TestRunDueChecksStoresUpReportForHealthyTLSCertificate(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	var checkedTarget tlsTarget
	tlsCheck := func(ctx context.Context, target tlsTarget) (tls.ConnectionState, error) {
		checkedTarget = target
		cert := workerTLSCertificate("api.example.com", time.Now().UTC().Add(45*24*time.Hour))
		return tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
			VerifiedChains:   [][]*x509.Certificate{{cert}},
		}, nil
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-tls-up")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-tls-up",
		Kind:            "tls",
		ConfigJSON:      `{"host":"api.example.com","warning_days":14}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-tls-test", TLSCheck: tlsCheck})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if checkedTarget.Address != "api.example.com:443" || checkedTarget.ServerName != "api.example.com" {
		t.Fatalf("checked target = %+v, want api.example.com:443", checkedTarget)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-tls-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "tls" || payload["chain_verified"] != true || payload["expiring"] != false || payload["ok"] != true {
		t.Fatalf("payload = %+v, want healthy TLS payload", payload)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-tls-up")
	if completed.LastSuccessAt == nil || completed.LastFailureAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want successful TLS completion", completed)
	}
}

func TestRunDueChecksStoresDegradedReportForExpiringTLSCertificate(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	tlsCheck := func(ctx context.Context, target tlsTarget) (tls.ConnectionState, error) {
		cert := workerTLSCertificate("api.example.com", time.Now().UTC().Add(5*24*time.Hour))
		return tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
			VerifiedChains:   [][]*x509.Certificate{{cert}},
		}, nil
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-tls-expiring")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-tls-expiring",
		Kind:            "tls_certificate",
		ConfigJSON:      `{"host":"api.example.com","port":8443,"server_name":"api.example.com","warning_days":14}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-tls-test", TLSCheck: tlsCheck})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-tls-expiring")
	if report.Health != "degraded" {
		t.Fatalf("report health = %q, want degraded", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "expiry_threshold" || payload["expiring"] != true || payload["warning_days"].(float64) != 14 {
		t.Fatalf("payload = %+v, want expiry threshold failure", payload)
	}

	completed := loadWorkerCoreMonitorConfig(t, database, "monitor-tls-expiring")
	if completed.LastFailureAt == nil || completed.LastSuccessAt != nil || completed.LeaseOwner != "" {
		t.Fatalf("completed config = %+v, want degraded TLS completion as failure", completed)
	}
}

func TestRunDueChecksStoresDownReportForInvalidTLSCertificate(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	tlsCheck := func(ctx context.Context, target tlsTarget) (tls.ConnectionState, error) {
		return tls.ConnectionState{}, x509.HostnameError{
			Certificate: workerTLSCertificate("wrong.example.com", time.Now().UTC().Add(45*24*time.Hour)),
			Host:        target.ServerName,
		}
	}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-tls-invalid")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-tls-invalid",
		Kind:            "tls",
		ConfigJSON:      `{"host":"api.example.com","server_name":"api.example.com"}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-tls-test", TLSCheck: tlsCheck})
	if err := app.runDueChecks(t.Context()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-tls-invalid")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "certificate" || payload["chain_verified"] != false || payload["ok"] != false {
		t.Fatalf("payload = %+v, want certificate failure", payload)
	}
}

func workerTLSCertificate(commonName string, notAfter time.Time) *x509.Certificate {
	return &x509.Certificate{
		Subject:   pkixName(commonName),
		Issuer:    pkixName("Orion Test CA"),
		DNSNames:  []string{commonName},
		NotBefore: time.Now().UTC().Add(-time.Hour),
		NotAfter:  notAfter,
	}
}

func pkixName(commonName string) pkix.Name {
	return pkix.Name{CommonName: commonName}
}
