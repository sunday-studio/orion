package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	client.sleep = func(context.Context, time.Duration) {}
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

func TestTokenAccessIsRaceFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) }))
	defer srv.Close()
	c := NewClient(srv.URL, "t0")
	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			c.SetAuthToken(fmt.Sprintf("t%d", i))
		}
		close(done)
	}()
	for i := 0; i < 50; i++ {
		_ = c.SendReport(SystemReport{}, "a")
	}
	<-done
}

func TestBare403IsTransientNotAuth(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte("<html>blocked by firewall</html>"))
	}))
	defer proxy.Close()
	err := NewClient(proxy.URL, "t").SendReport(SystemReport{}, "a")
	if err == nil || IsAuthError(err) {
		t.Fatalf("err = %v, want non-auth error", err)
	}
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"error":"token revoked"}`))
	}))
	defer core.Close()
	if err := NewClient(core.URL, "t").SendReport(SystemReport{}, "a"); !IsAuthError(err) {
		t.Fatalf("err = %v, want auth error for Core JSON 403", err)
	}
}

func TestCancelledContextStopsRequestsAndRetryWaits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer srv.Close()
	c := NewClient(srv.URL, "t")
	ctx, cancel := context.WithCancel(context.Background())
	c.SetContext(ctx)
	cancel()
	start := time.Now()
	if err := c.SendReport(SystemReport{}, "a"); err == nil {
		t.Fatal("expected error")
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancelled client should return immediately")
	}
}
