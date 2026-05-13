package agent

import (
	"path/filepath"
	"testing"

	"orion/agent/internal/config"
	agentstate "orion/agent/internal/state"
)

func TestAgentRereadsMaintenanceMode(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	stateStore, err := agentstate.Open(statePath)
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	defer stateStore.Close()

	if err := stateStore.UpdateRegistration("agent-1", "token", "http://localhost:8999"); err != nil {
		t.Fatalf("update registration: %v", err)
	}
	internalState, err := stateStore.Get()
	if err != nil {
		t.Fatalf("get state: %v", err)
	}

	agent := New(&config.UserConfig{CoreURL: "http://localhost:8999"}, stateStore, internalState)
	if agent.isInMaintenanceMode() {
		t.Fatal("isInMaintenanceMode() = true, want false")
	}

	if err := stateStore.SetMaintenanceMode(true, nil); err != nil {
		t.Fatalf("set maintenance: %v", err)
	}

	if !agent.isInMaintenanceMode() {
		t.Fatal("isInMaintenanceMode() = false, want true after state update")
	}
}
