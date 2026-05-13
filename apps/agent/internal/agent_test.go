package agent

import (
	"path/filepath"
	"testing"

	"orion/agent/internal/config"
)

func TestAgentRereadsMaintenanceMode(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.yaml")
	state := &config.InternalState{AgentID: "agent-1", Token: "token", MaintenanceMode: false}
	if err := state.Save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}

	agent := NewWithStatePath(&config.UserConfig{CoreURL: "http://localhost:8999"}, state, statePath)
	if agent.isInMaintenanceMode() {
		t.Fatal("isInMaintenanceMode() = true, want false")
	}

	state.MaintenanceMode = true
	if err := state.Save(statePath); err != nil {
		t.Fatalf("save updated state: %v", err)
	}

	if !agent.isInMaintenanceMode() {
		t.Fatal("isInMaintenanceMode() = false, want true after state update")
	}
}
