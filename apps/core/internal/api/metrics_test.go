package api

import (
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"strings"
	"testing"
)

func metricsRequest(server *Server, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return rawRequest(server, req)
}

func TestMetricsEndpointRequiresMetricsTokenWhenConfigured(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{MetricsToken: "scrape-me"})
	if rec := metricsRequest(server, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token status = %d, want 401", rec.Code)
	}
	if rec := metricsRequest(server, "wrong"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d, want 401", rec.Code)
	}
	rec := metricsRequest(server, "scrape-me")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "orion_up 1") {
		t.Fatalf("valid token status = %d body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content type = %q, want Prometheus text", ct)
	}
}

func TestMetricsEndpointFollowsAdminAuthWithoutMetricsToken(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{AdminUsername: "admin", AdminPassword: "pw", JWTSecret: "secret-secret-secret-secret-1234", FrontendAuthOn: true})
	if rec := metricsRequest(server, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401 when admin auth is on", rec.Code)
	}
	open := setupTestServerWithConfig(t, &config.Config{})
	if rec := metricsRequest(open, ""); rec.Code != http.StatusOK {
		t.Fatalf("auth-less install status = %d, want 200", rec.Code)
	}
}

func TestMetricsCountsRequestsByRouteTemplate(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{})
	rawRequest(server, httptest.NewRequest(http.MethodGet, "/health", nil))
	rawRequest(server, httptest.NewRequest(http.MethodGet, "/definitely/not/a/route/with-a-secret-token", nil))
	body := metricsRequest(server, "").Body.String()
	if !strings.Contains(body, `orion_http_requests_total{route="/health",status="200"} 1`) {
		t.Fatalf("health request not counted:\n%s", body)
	}
	if strings.Contains(body, "with-a-secret-token") || !strings.Contains(body, `route="unmatched"`) {
		t.Fatalf("unmatched paths must collapse to one label, never leak raw paths:\n%s", body)
	}
}

func TestMetricsIncludeProductLogQueueWhenEnabled(t *testing.T) {
	server := setupLogsServer(t, nil)
	body := metricsRequest(server, "").Body.String()
	for _, want := range []string{"orion_logs_queue_entries 0", "orion_logs_written_total 0", "orion_logs_dropped_total 0"} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q", want)
		}
	}
}
