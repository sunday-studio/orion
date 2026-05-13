package collector

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRunHTTPMonitorExpectations(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Body:       io.NopCloser(strings.NewReader(`{"status":"ok","version":"1.2.3"}`)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	tests := []struct {
		name         string
		config       HTTPMonitorConfig
		wantStatus   string
		wantFailure  string
		wantHTTPCode int
	}{
		{
			name: "status body and regex match",
			config: HTTPMonitorConfig{
				URL:               "https://example.com/health",
				Timeout:           time.Second,
				ExpectedStatus:    http.StatusAccepted,
				ExpectedBody:      `"status":"ok"`,
				ExpectedBodyRegex: `"version":"[0-9]+\.[0-9]+\.[0-9]+"`,
			},
			wantStatus:   "up",
			wantHTTPCode: http.StatusAccepted,
		},
		{
			name: "status mismatch",
			config: HTTPMonitorConfig{
				URL:            "https://example.com/health",
				Timeout:        time.Second,
				ExpectedStatus: http.StatusOK,
			},
			wantStatus:   "down",
			wantFailure:  "unexpected status code",
			wantHTTPCode: http.StatusAccepted,
		},
		{
			name: "body mismatch",
			config: HTTPMonitorConfig{
				URL:            "https://example.com/health",
				Timeout:        time.Second,
				ExpectedStatus: http.StatusAccepted,
				ExpectedBody:   "missing",
			},
			wantStatus:  "down",
			wantFailure: "expected body content not found",
		},
		{
			name: "regex mismatch",
			config: HTTPMonitorConfig{
				URL:               "https://example.com/health",
				Timeout:           time.Second,
				ExpectedStatus:    http.StatusAccepted,
				ExpectedBodyRegex: "version=2",
			},
			wantStatus:  "down",
			wantFailure: "expected body regex did not match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := runHTTPMonitorWithClient(tt.config, client)
			if err != nil {
				t.Fatalf("RunHTTPMonitor() error = %v", err)
			}
			if result.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", result.Status, tt.wantStatus)
			}
			if tt.wantHTTPCode != 0 && result.Metrics["status_code"] != tt.wantHTTPCode {
				t.Fatalf("status_code = %v, want %d", result.Metrics["status_code"], tt.wantHTTPCode)
			}
			if tt.wantFailure != "" && result.Metrics["failure_reason"] != tt.wantFailure {
				t.Fatalf("failure_reason = %v, want %q", result.Metrics["failure_reason"], tt.wantFailure)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
