package collector

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"orion/agent/internal/config"
)

func TestRunWebsiteMonitorIncludesDNSAndTLSMetrics(t *testing.T) {
	expiresAt := time.Now().Add(45 * 24 * time.Hour).UTC()
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("<html>ok</html>")),
				Header:     make(http.Header),
				Request:    req,
				TLS: &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{{NotAfter: expiresAt}},
				},
			}, nil
		}),
	}

	result := runWebsiteMonitorWithClientAndResolver(
		config.WebsiteMonitorConfig{
			URL:            "https://example.com",
			ExpectedStatus: http.StatusOK,
		},
		client,
		func(host string) ([]string, error) {
			if host != "example.com" {
				t.Fatalf("lookup host = %q, want example.com", host)
			}
			return []string{"203.0.113.10"}, nil
		},
	)

	if result.Status != "up" {
		t.Fatalf("status = %q, want up", result.Status)
	}
	if result.Metrics["tls_expires_at"] != expiresAt.Format(time.RFC3339) {
		t.Fatalf("tls_expires_at = %v, want %s", result.Metrics["tls_expires_at"], expiresAt.Format(time.RFC3339))
	}
	if result.Metrics["tls_days_remaining"] == nil {
		t.Fatalf("tls_days_remaining missing from metrics: %+v", result.Metrics)
	}
	resolvedIPs, ok := result.Metrics["resolved_ips"].([]string)
	if !ok || len(resolvedIPs) != 1 || resolvedIPs[0] != "203.0.113.10" {
		t.Fatalf("resolved_ips = %#v, want [203.0.113.10]", result.Metrics["resolved_ips"])
	}
}

func TestRunWebsiteMonitorReportsDNSError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	result := runWebsiteMonitorWithClientAndResolver(
		config.WebsiteMonitorConfig{
			URL:            "https://example.com",
			ExpectedStatus: http.StatusOK,
		},
		client,
		func(host string) ([]string, error) {
			return nil, &netDNSError{host: host}
		},
	)

	if result.Status != "down" {
		t.Fatalf("status = %q, want down", result.Status)
	}
	if result.Metrics["failure_reason"] != "dns resolution failed" {
		t.Fatalf("failure_reason = %v, want dns resolution failed", result.Metrics["failure_reason"])
	}
}

type netDNSError struct {
	host string
}

func (e *netDNSError) Error() string {
	return "lookup " + e.host + ": no such host"
}
