package transport

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestClientRetriesTransientNetworkError(t *testing.T) {
	var attempts int
	client := newRetryTestClient(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("connection refused")
		}
		return responseWithStatus(http.StatusOK), nil
	})

	resp, err := client.makeRequest(http.MethodPost, "/v1/test", map[string]string{"ok": "true"}, nil)
	if err != nil {
		t.Fatalf("makeRequest() error = %v", err)
	}
	defer resp.Body.Close()
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestClientRetriesRetryableStatus(t *testing.T) {
	var attempts int
	client := newRetryTestClient(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return responseWithStatus(http.StatusServiceUnavailable), nil
		}
		return responseWithStatus(http.StatusOK), nil
	})

	resp, err := client.makeRequest(http.MethodPost, "/v1/test", nil, nil)
	if err != nil {
		t.Fatalf("makeRequest() error = %v", err)
	}
	defer resp.Body.Close()
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestClientDoesNotRetryClientError(t *testing.T) {
	var attempts int
	client := newRetryTestClient(func(req *http.Request) (*http.Response, error) {
		attempts++
		return responseWithStatus(http.StatusBadRequest), nil
	})

	resp, err := client.makeRequest(http.MethodPost, "/v1/test", nil, nil)
	if err != nil {
		t.Fatalf("makeRequest() error = %v", err)
	}
	defer resp.Body.Close()
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestClientReturnsAuthErrorForProtectedReport(t *testing.T) {
	client := newRetryTestClient(func(req *http.Request) (*http.Response, error) {
		if got, want := req.Header.Get("Authorization"), "Bearer token"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader("bad token")),
			Header:     make(http.Header),
		}, nil
	})

	err := client.SendReport(SystemReport{}, "agent-1")
	if err == nil {
		t.Fatal("SendReport() error = nil, want auth error")
	}
	if !IsAuthError(err) {
		t.Fatalf("SendReport() error = %T %[1]v, want AuthError", err)
	}
}

func TestAuthErrorRedactsCoreResponseBody(t *testing.T) {
	err := NewAuthError(http.StatusUnauthorized, []byte(`{"error":"agent_token_revoked","message":"authorization: Bearer secret-token","token":"secret-token"}`))

	text := err.Error()
	if !strings.Contains(text, "agent_token_revoked") {
		t.Fatalf("Error() = %q, want stable error code", text)
	}
	if strings.Contains(text, "secret-token") {
		t.Fatalf("Error() = %q, want redacted token", text)
	}
	if !strings.Contains(text, "[redacted]") {
		t.Fatalf("Error() = %q, want redaction marker", text)
	}
}

func newRetryTestClient(roundTrip func(*http.Request) (*http.Response, error)) *Client {
	client := NewClient("https://core.example.com", "token")
	client.httpClient.Transport = roundTripFunc(roundTrip)
	client.retry = RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond,
		JitterRatio: 0,
	}
	client.sleep = func(time.Duration) {}
	return client
}

func responseWithStatus(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRegisterAgentSendsEnrollmentTokenHeader(t *testing.T) {
	var got string
	client := newRetryTestClient(func(req *http.Request) (*http.Response, error) {
		got = req.Header.Get("X-Orion-Enrollment-Token")
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"success":true,"data":{"agent_id":"a1","token":"t1"}}`)),
			Header:     make(http.Header),
		}, nil
	})
	client.SetEnrollmentToken("  enroll-secret ")

	if _, err := client.RegisterAgent(AgentRegistrationRequest{MachineId: "m1"}); err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}
	if got != "enroll-secret" {
		t.Fatalf("enrollment header = %q, want trimmed secret", got)
	}
}

func TestRegisterAgentOmitsEnrollmentHeaderWhenUnset(t *testing.T) {
	var present bool
	client := newRetryTestClient(func(req *http.Request) (*http.Response, error) {
		_, present = req.Header["X-Orion-Enrollment-Token"]
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"success":true,"data":{"agent_id":"a1","token":"t1"}}`)),
			Header:     make(http.Header),
		}, nil
	})
	if _, err := client.RegisterAgent(AgentRegistrationRequest{MachineId: "m1"}); err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}
	if present {
		t.Fatal("enrollment header sent although no token configured")
	}
}

func TestMonitorReportSerializesReportIDForIdempotentReplay(t *testing.T) {
	body, err := json.Marshal(MonitorReport{ReportID: "id-1", Timestamp: "t", Health: "up"})
	if err != nil || !strings.Contains(string(body), `"report_id":"id-1"`) {
		t.Fatalf("payload = %s err = %v, want report_id present", body, err)
	}
	legacy, _ := json.Marshal(MonitorReport{Timestamp: "t", Health: "up"})
	if strings.Contains(string(legacy), "report_id") {
		t.Fatalf("empty ReportID must be omitted, got %s", legacy)
	}
}
