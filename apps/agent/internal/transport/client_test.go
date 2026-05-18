package transport

import (
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
