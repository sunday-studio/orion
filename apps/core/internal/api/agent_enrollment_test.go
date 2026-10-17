package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"testing"
)

func registerWithHeader(t *testing.T, server *Server, machineID string, header string, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"machine_id": machineID, "name": "host", "os": "linux", "arch": "amd64", "reporting_interval_seconds": 30,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if header != "" {
		req.Header.Set(enrollmentTokenHeader, header)
	}
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, req)
	return recorder
}

func TestRegisterRequiresEnrollmentTokenWhenConfigured(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{EnrollmentToken: "enroll-secret-123"})

	for name, header := range map[string]string{"missing": "", "wrong": "nope", "prefix": "enroll-secret-12"} {
		if resp := registerWithHeader(t, server, "machine-"+name, header, ""); resp.Code != http.StatusUnauthorized {
			t.Fatalf("%s token status = %d, body = %s, want 401", name, resp.Code, resp.Body.String())
		}
	}
	var agents int64
	server.db.Table("agents").Count(&agents)
	if agents != 0 {
		t.Fatalf("agents created by rejected registrations = %d, want 0", agents)
	}

	if resp := registerWithHeader(t, server, "machine-ok", "enroll-secret-123", ""); resp.Code != http.StatusOK {
		t.Fatalf("valid token status = %d, body = %s, want 200", resp.Code, resp.Body.String())
	}
}

func TestRegisterStaysOpenWithoutEnrollmentTokenConfigured(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{})
	if resp := registerWithHeader(t, server, "machine-open", "", ""); resp.Code != http.StatusOK {
		t.Fatalf("open registration status = %d, body = %s, want 200", resp.Code, resp.Body.String())
	}
}

func TestRegisterIsRateLimitedPerClientIP(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{RegisterRateLimitPerMinute: 3})

	for i := 0; i < 3; i++ {
		if resp := registerWithHeader(t, server, "machine-rl-"+string(rune('a'+i)), "", "203.0.113.7:4000"); resp.Code != http.StatusOK {
			t.Fatalf("attempt %d status = %d, want 200", i+1, resp.Code)
		}
	}
	if resp := registerWithHeader(t, server, "machine-rl-d", "", "203.0.113.7:4001"); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit status = %d, want 429", resp.Code)
	}
	if resp := registerWithHeader(t, server, "machine-rl-other", "", "198.51.100.2:4000"); resp.Code != http.StatusOK {
		t.Fatalf("other IP status = %d, want 200", resp.Code)
	}
}

func TestRejectedEnrollmentStillConsumesRateLimitBudget(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{EnrollmentToken: "s", RegisterRateLimitPerMinute: 2})
	for i := 0; i < 2; i++ {
		registerWithHeader(t, server, "m", "bad", "203.0.113.50:1")
	}
	if resp := registerWithHeader(t, server, "m", "bad", "203.0.113.50:1"); resp.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 after repeated bad tokens (brute-force guard)", resp.Code)
	}
}
