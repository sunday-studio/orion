package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	agentstate "orion/agent/internal/state"
)

type maintenanceCore struct {
	mu     sync.Mutex
	bodies []map[string]any
}

func newMaintenanceCore(t *testing.T) (*maintenanceCore, *httptest.Server) {
	t.Helper()
	core := &maintenanceCore{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		core.mu.Lock()
		core.bodies = append(core.bodies, body)
		core.mu.Unlock()
		if r.Method != http.MethodPut || r.URL.Path != "/v1/agents/agent-1/maintenance" || r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return core, server
}

func TestMaintenanceDownSendsDurationToCoreAndRecordsLocalEndTime(t *testing.T) {
	core, server := newMaintenanceCore(t)
	statePath := filepath.Join(t.TempDir(), "state.db")
	store, err := agentstate.Open(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateRegistration("agent-1", "tok", server.URL); err != nil {
		t.Fatal(err)
	}
	store.Close()

	opts := &Options{StatePath: statePath}
	if err := runMaintenance(context.Background(), opts, "-down", "kernel upgrade", 2*time.Hour); err != nil {
		t.Fatalf("runMaintenance(-down) error = %v", err)
	}

	core.mu.Lock()
	got := core.bodies[0]
	core.mu.Unlock()
	if got["maintenance_mode"] != true || got["duration_seconds"] != float64(7200) {
		t.Fatalf("Core received %v, want maintenance_mode true with duration_seconds 7200", got)
	}
	reopened, _ := agentstate.Open(statePath)
	defer reopened.Close()
	state, _ := reopened.Get()
	if !state.MaintenanceMode || state.MaintenanceUntil == nil || state.MaintenanceReason == nil || *state.MaintenanceReason != "kernel upgrade" {
		t.Fatalf("local state = %+v, want maintenance with end time and reason", state)
	}
	if d := time.Until(*state.MaintenanceUntil); d < 119*time.Minute || d > 121*time.Minute {
		t.Fatalf("local maintenance ends in %v, want about 2h", d)
	}

	if err := runMaintenance(context.Background(), opts, "-up", "", 0); err != nil {
		t.Fatalf("runMaintenance(-up) error = %v", err)
	}
	state, _ = reopened.Get()
	if state.MaintenanceMode || state.MaintenanceUntil != nil {
		t.Fatalf("local state after up = %+v, want cleared", state)
	}
	core.mu.Lock()
	last := core.bodies[len(core.bodies)-1]
	core.mu.Unlock()
	if last["maintenance_mode"] != false || last["duration_seconds"] != nil {
		t.Fatalf("Core received %v on up, want maintenance_mode false and no duration", last)
	}
}

func TestMaintenanceDownDefaultsToTwentyFourHours(t *testing.T) {
	core, server := newMaintenanceCore(t)
	statePath := filepath.Join(t.TempDir(), "state.db")
	store, _ := agentstate.Open(statePath)
	store.UpdateRegistration("agent-1", "tok", server.URL)
	store.Close()

	if err := runMaintenance(context.Background(), &Options{StatePath: statePath}, "-down", "", 0); err != nil {
		t.Fatal(err)
	}
	core.mu.Lock()
	defer core.mu.Unlock()
	if core.bodies[0]["duration_seconds"] != float64(86400) {
		t.Fatalf("Core received %v, want the 24h default so maintenance can never be open-ended", core.bodies[0])
	}
}

func TestMaintenanceWorksLocallyWhenAgentIsNotRegistered(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	if err := runMaintenance(context.Background(), &Options{StatePath: statePath}, "-down", "bench", time.Hour); err != nil {
		t.Fatalf("runMaintenance() error = %v, want local-only success", err)
	}
	store, _ := agentstate.Open(statePath)
	defer store.Close()
	state, _ := store.Get()
	if !state.MaintenanceMode || state.MaintenanceUntil == nil {
		t.Fatalf("state = %+v, want local maintenance recorded", state)
	}
}
