package node

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orion/node/internal/config"
	nodestate "orion/node/internal/state"
	"orion/node/internal/transport"
)

func TestNodeRereadsMaintenanceMode(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	stateStore, err := nodestate.Open(statePath)
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	defer stateStore.Close()

	if err := stateStore.UpdateRegistration("node-1", "token", "http://localhost:8999"); err != nil {
		t.Fatalf("update registration: %v", err)
	}
	internalState, err := stateStore.Get()
	if err != nil {
		t.Fatalf("get state: %v", err)
	}

	node := New(&config.UserConfig{CoreURL: "http://localhost:8999"}, stateStore, internalState)
	if node.isInMaintenanceMode() {
		t.Fatal("isInMaintenanceMode() = true, want false")
	}

	if err := stateStore.SetMaintenanceMode(true, nil); err != nil {
		t.Fatalf("set maintenance: %v", err)
	}

	if !node.isInMaintenanceMode() {
		t.Fatal("isInMaintenanceMode() = false, want true after state update")
	}
}

func TestNodePersistsSystemReportDuringCoreOutage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "core unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	stateStore := openNodeTestStore(t)
	if err := stateStore.UpdateRegistration("node-1", "token", server.URL); err != nil {
		t.Fatalf("update registration: %v", err)
	}
	internalState, err := stateStore.Get()
	if err != nil {
		t.Fatalf("get state: %v", err)
	}

	node := New(&config.UserConfig{
		CoreURL:  server.URL,
		Interval: "60s",
	}, stateStore, internalState)
	err = node.runSystemMetrics(context.Background())
	if err == nil {
		t.Fatal("runSystemMetrics() error = nil, want outage error")
	}
	if transport.IsAuthError(err) {
		t.Fatalf("runSystemMetrics() error = %v, want retryable outage error", err)
	}

	count, err := stateStore.CountSpooledReports()
	if err != nil {
		t.Fatalf("CountSpooledReports() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("spooled reports = %d, want one report after outage", count)
	}
	state, err := stateStore.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if state.NodeID != "node-1" || state.Token != "token" {
		t.Fatalf("identity state = %+v, want unchanged after outage", state)
	}
}

func TestNodeDoesNotSpoolReportsRejectedForAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"node_token_revoked","message":"authorization: Bearer secret-token"}`))
	}))
	defer server.Close()

	stateStore := openNodeTestStore(t)
	if err := stateStore.UpdateRegistration("node-1", "secret-token", server.URL); err != nil {
		t.Fatalf("update registration: %v", err)
	}
	internalState, err := stateStore.Get()
	if err != nil {
		t.Fatalf("get state: %v", err)
	}

	node := New(&config.UserConfig{
		CoreURL:  server.URL,
		Interval: "60s",
	}, stateStore, internalState)
	err = node.runSystemMetrics(context.Background())
	if err == nil {
		t.Fatal("runSystemMetrics() error = nil, want auth error")
	}
	if !transport.IsAuthError(err) {
		t.Fatalf("runSystemMetrics() error = %T %[1]v, want auth error", err)
	}
	if got := err.Error(); got == "" || got == "secret-token" {
		t.Fatalf("auth error = %q, want redacted visible detail", got)
	} else if containsSecret(got, "secret-token") {
		t.Fatalf("auth error = %q, leaked token", got)
	}
	count, err := stateStore.CountSpooledReports()
	if err != nil {
		t.Fatalf("CountSpooledReports() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("spooled reports = %d, want none for rejected credentials", count)
	}
}

func TestNodeDoesNotSpoolReportsAfterAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"node_token_revoked"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	stateStore := openNodeTestStore(t)
	if err := stateStore.UpdateRegistration("node-1", "token", server.URL); err != nil {
		t.Fatalf("update registration: %v", err)
	}
	internalState, err := stateStore.Get()
	if err != nil {
		t.Fatalf("get state: %v", err)
	}

	node := New(&config.UserConfig{
		CoreURL:  server.URL,
		Interval: "60s",
	}, stateStore, internalState)
	err = node.runSystemMetrics(context.Background())
	if err == nil {
		t.Fatal("runSystemMetrics() error = nil, want auth error")
	}
	if !transport.IsAuthError(err) {
		t.Fatalf("runSystemMetrics() error = %T %[1]v, want auth error", err)
	}
	count, err := stateStore.CountSpooledReports()
	if err != nil {
		t.Fatalf("CountSpooledReports() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("spooled reports = %d, want none after auth failure", count)
	}
	state, err := stateStore.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if state.NodeID != "node-1" || state.Token != "token" {
		t.Fatalf("identity state = %+v, want unchanged after auth failure", state)
	}
}

func TestNodeFlushesDurableReportSpoolAfterRestart(t *testing.T) {
	var received bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/nodes/node-1/report" {
			t.Fatalf("path = %q, want system report endpoint", r.URL.Path)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer token"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		var report transport.SystemReport
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			t.Fatalf("decode report: %v", err)
		}
		if report.UptimeSeconds != 42 {
			t.Fatalf("UptimeSeconds = %d, want 42", report.UptimeSeconds)
		}
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	statePath := filepath.Join(t.TempDir(), "state.db")
	stateStore, err := nodestate.Open(statePath)
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	if err := stateStore.UpdateRegistration("node-1", "token", server.URL); err != nil {
		t.Fatalf("update registration: %v", err)
	}
	if _, err := stateStore.EnqueueReport(nodestate.ReportSpoolKindSystem, "node-1", "", "", transport.SystemReport{
		UptimeSeconds: 42,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}, nil); err != nil {
		t.Fatalf("enqueue report: %v", err)
	}
	if err := stateStore.Close(); err != nil {
		t.Fatalf("close first state store: %v", err)
	}

	reopened, err := nodestate.Open(statePath)
	if err != nil {
		t.Fatalf("reopen state store: %v", err)
	}
	defer reopened.Close()
	internalState, err := reopened.Get()
	if err != nil {
		t.Fatalf("get state: %v", err)
	}

	node := New(&config.UserConfig{CoreURL: server.URL}, reopened, internalState)
	if err := node.flushDurableSpool(context.Background()); err != nil {
		t.Fatalf("flushDurableSpool() error = %v", err)
	}
	if !received {
		t.Fatal("server did not receive spooled report")
	}
	if count, err := reopened.CountSpooledReports(); err != nil || count != 0 {
		t.Fatalf("CountSpooledReports() = %d, %v, want 0 nil", count, err)
	}
}

func containsSecret(value string, secret string) bool {
	return len(secret) > 0 && value != "" && strings.Contains(value, secret)
}

func openNodeTestStore(t *testing.T) *nodestate.Store {
	t.Helper()

	stateStore, err := nodestate.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() {
		if err := stateStore.Close(); err != nil {
			t.Fatalf("close state store: %v", err)
		}
	})
	return stateStore
}
