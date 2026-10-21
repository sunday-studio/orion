package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"orion/node/internal/config"
	"orion/node/internal/registration"
	nodestate "orion/node/internal/state"
)

// ORB-015: the node comes up once Core returns.
func TestRegisterWithRetryRecoversWhenCoreComesBack(t *testing.T) {
	var calls atomic.Int32
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 { // Core "down" for the first two attempts
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":{"node_id":"ag","token":"tk"}}`))
	}))
	defer core.Close()

	store, err := nodestate.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := &config.UserConfig{CoreURL: core.URL, Interval: "60s"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := registerWithRetry(ctx, registration.New(cfg, "", store)); err != nil {
		t.Fatalf("registerWithRetry() = %v", err)
	}
	if st, _ := store.Get(); st.NodeID != "ag" {
		t.Fatalf("state = %+v", st)
	}
}

func TestRegisterWithRetryStopsOnRejectedEnrollment(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer core.Close()
	store, _ := nodestate.Open(filepath.Join(t.TempDir(), "s.db"))
	defer store.Close()
	start := time.Now()
	err := registerWithRetry(context.Background(), registration.New(&config.UserConfig{CoreURL: core.URL, Interval: "60s"}, "", store))
	if err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("err=%v after %s; a rejected token is permanent", err, time.Since(start))
	}
}

func TestRegisterWithRetryHonoursCancellation(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer core.Close()
	store, _ := nodestate.Open(filepath.Join(t.TempDir(), "s.db"))
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := registerWithRetry(ctx, registration.New(&config.UserConfig{CoreURL: core.URL, Interval: "60s"}, "", store)); err == nil {
		t.Fatal("expected cancellation error")
	}
}
