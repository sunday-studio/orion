package registration

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"orion/node/internal/config"
	nodestate "orion/node/internal/state"
	"orion/node/internal/transport"
)

func TestEnrollmentTokenPrefersEnvironment(t *testing.T) {
	cfg := &config.UserConfig{EnrollmentToken: "from-yaml"}
	if got := enrollmentToken(cfg); got != "from-yaml" {
		t.Fatalf("enrollmentToken() = %q, want yaml value", got)
	}
	t.Setenv("ORION_ENROLLMENT_TOKEN", " from-env ")
	if got := enrollmentToken(cfg); got != "from-env" {
		t.Fatalf("enrollmentToken() = %q, want env value to win", got)
	}
}

func newRegistrationFixture(t *testing.T, handler http.HandlerFunc) *RegistrationService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	store, err := nodestate.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return New(&config.UserConfig{CoreURL: server.URL, Interval: "60s"}, "", store)
}

// ORB-015: the node boots while Core is still down and registers once Core returns.
func TestRegisterWithRetryWaitsOutACoreOutage(t *testing.T) {
	var calls atomic.Int32
	svc := newRegistrationFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 4 { // more than one full client retry cycle (3 attempts)
			http.Error(w, "starting up", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":{"node_id":"node-1","token":"tok-1"}}`))
	})

	err := svc.RegisterWithRetry(context.Background(), RegistrationBackoff{Initial: time.Millisecond, Max: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("RegisterWithRetry() error = %v, want success after Core came back", err)
	}
	state, _ := svc.stateStore.Get()
	if state.NodeID != "node-1" || state.Token != "tok-1" {
		t.Fatalf("state = %+v, want the registered node", state)
	}
}

func TestRegisterWithRetryFailsFastOnPermanentErrors(t *testing.T) {
	var calls atomic.Int32
	svc := newRegistrationFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, `{"error":"node_token_reissue_required"}`, http.StatusConflict)
	})
	err := svc.RegisterWithRetry(context.Background(), RegistrationBackoff{Initial: time.Millisecond, Max: time.Millisecond})
	if err == nil || transport.IsTransientError(err) {
		t.Fatalf("error = %v, want a permanent (non-transient) failure", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 (no retry of a 409)", calls.Load())
	}
}

func TestRegisterWithRetryStopsWhenCancelled(t *testing.T) {
	svc := newRegistrationFixture(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := svc.RegisterWithRetry(ctx, RegistrationBackoff{Initial: 20 * time.Millisecond, Max: 20 * time.Millisecond})
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded wrapped", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("RegisterWithRetry kept running after cancel")
	}
}
