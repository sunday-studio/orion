package worker

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"strings"
	"testing"
	"time"
)

func TestRunDueChecksStoresUpReportForDomainExpiration(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	expiresAt := time.Now().UTC().Add(90 * 24 * time.Hour).Format(time.RFC3339)
	var requestedURL string
	var requestedAccept string
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestedURL = r.URL.String()
		requestedAccept = r.Header.Get("Accept")
		return workerHTTPResponse(http.StatusOK, rdapDomainBody(expiresAt)), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-domain-up")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-domain-up",
		Kind:            "domain_expiration",
		ConfigJSON:      `{"domain":"Example.COM","warning_days":30}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-domain-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if requestedURL != "https://rdap.org/domain/example.com" {
		t.Fatalf("requested URL = %q, want default RDAP URL", requestedURL)
	}
	if requestedAccept != "application/rdap+json, application/json" {
		t.Fatalf("Accept header = %q, want RDAP JSON preference", requestedAccept)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-domain-up")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["type"] != "domain_expiration" || payload["domain"] != "example.com" || payload["lookup_strategy"] != "rdap" || payload["fallback_strategy"] != "none" || payload["ok"] != true {
		t.Fatalf("payload = %+v, want domain expiration success payload", payload)
	}
	expiresAtValue, ok := payload["expires_at"].(string)
	if !ok || expiresAtValue == "" || payload["days_remaining"].(float64) < 80 {
		t.Fatalf("payload = %+v, want expiration date and days remaining", payload)
	}
}

func TestRunDueChecksStoresDegradedReportForExpiringDomain(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	expiresAt := time.Now().UTC().Add(5 * 24 * time.Hour).Format(time.RFC3339)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusOK, rdapDomainBody(expiresAt)), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-domain-expiring")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-domain-expiring",
		Kind:            "domain_expiration",
		ConfigJSON:      `{"domain":"example.com","rdap_url":"https://rdap.example.test/domain/example.com","warning_days":30}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-domain-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-domain-expiring")
	if report.Health != "degraded" {
		t.Fatalf("report health = %q, want degraded", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "expiry_threshold" || payload["warning_days"].(float64) != 30 || payload["ok"] != false {
		t.Fatalf("payload = %+v, want expiry threshold payload", payload)
	}
}

func TestRunDueChecksStoresDownReportForUnavailableDomainExpirationData(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusOK, `{"events":[{"eventAction":"registration","eventDate":"2026-05-27T00:00:00Z"}]}`), nil
	})}

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-domain-unavailable")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-domain-unavailable",
		Kind:            "domain_expiration",
		ConfigJSON:      `{"domain":"example.invalid","warning_days":30}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{WorkerID: "worker-domain-test", HTTPClient: httpClient})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-domain-unavailable")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "unavailable_data" || payload["fallback_strategy"] != "none" {
		t.Fatalf("payload = %+v, want unavailable RDAP data with no fallback", payload)
	}
}

func TestRunDueChecksFallsBackToWHOISForUnavailableRDAPDomainExpirationData(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	expiresAt := time.Now().UTC().Add(90 * 24 * time.Hour).Format(time.RFC3339)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusOK, `{"events":[{"eventAction":"registration","eventDate":"2026-05-27T00:00:00Z"}]}`), nil
	})}
	conn := newScriptedMailConn("Domain Name: EXAMPLE.COM\r\nRegistry Expiry Date: " + expiresAt + "\r\n")
	var dialedAddress string

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-domain-whois")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-domain-whois",
		Kind:            "domain_expiration",
		ConfigJSON:      `{"domain":"example.com","warning_days":30}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID:   "worker-domain-test",
		HTTPClient: httpClient,
		TCPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			dialedAddress = address
			return conn, nil
		},
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}
	if dialedAddress != "whois.verisign-grs.com:43" {
		t.Fatalf("dialed address = %q, want Verisign WHOIS server", dialedAddress)
	}
	if !strings.Contains(conn.Written(), "example.com") {
		t.Fatalf("written WHOIS query = %q, want domain", conn.Written())
	}

	report := loadWorkerMonitorReport(t, database, "monitor-domain-whois")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["fallback_strategy"] != "whois" ||
		payload["whois_server"] != "whois.verisign-grs.com:43" ||
		payload["failure_stage"] != "" ||
		payload["ok"] != true {
		t.Fatalf("payload = %+v, want successful WHOIS fallback payload", payload)
	}
	expiresAtValue, ok := payload["expires_at"].(string)
	if !ok || expiresAtValue == "" || payload["days_remaining"].(float64) < 80 {
		t.Fatalf("payload = %+v, want WHOIS expiration date and days remaining", payload)
	}
}

func TestRunDueChecksStoresDownReportWhenRDAPAndWHOISDomainExpirationDataAreUnavailable(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return workerHTTPResponse(http.StatusNotFound, `{"error":"not found"}`), nil
	})}
	conn := newScriptedMailConn("Domain Name: EXAMPLE.COM\r\nNo match for expiration\r\n")

	insertWorkerCoreOwner(t, database)
	insertWorkerMonitor(t, database, "monitor-domain-no-data")
	insertWorkerCoreMonitorConfig(t, database, db.CoreMonitorConfig{
		MonitorID:       "monitor-domain-no-data",
		Kind:            "domain_expiration",
		ConfigJSON:      `{"domain":"example.com","whois_server":"whois.example.test:43","warning_days":30}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  5,
		NextRunAt:       time.Now().UTC().Add(-time.Minute),
	})

	app := NewApp(database, logging.NewLogger(), Options{
		WorkerID:   "worker-domain-test",
		HTTPClient: httpClient,
		TCPDialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			return conn, nil
		},
	})
	if err := app.runDueChecks(context.Background()); err != nil {
		t.Fatalf("runDueChecks() error = %v", err)
	}

	report := loadWorkerMonitorReport(t, database, "monitor-domain-no-data")
	if report.Health != "down" {
		t.Fatalf("report health = %q, want down", report.Health)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report.Payload), &payload); err != nil {
		t.Fatalf("unmarshal report payload: %v", err)
	}
	if payload["failure_stage"] != "unavailable_data" ||
		payload["fallback_strategy"] != "whois" ||
		payload["whois_server"] != "whois.example.test:43" ||
		payload["ok"] != false {
		t.Fatalf("payload = %+v, want RDAP and WHOIS unavailable payload", payload)
	}
	if !strings.Contains(payload["error"].(string), "WHOIS fallback failed") {
		t.Fatalf("payload = %+v, want fallback failure error", payload)
	}
}

func rdapDomainBody(expiresAt string) string {
	return `{"events":[{"eventAction":"registration","eventDate":"2026-05-27T00:00:00Z"},{"eventAction":"expiration","eventDate":"` + expiresAt + `"}]}`
}
