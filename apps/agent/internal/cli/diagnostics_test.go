package cli

import (
	"os"
	"path/filepath"
	"testing"

	agentstate "orion/agent/internal/state"
)

func TestInspectAgentStatusDoesNotCreateMissingState(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "missing", "state.db")

	report := InspectAgentStatus(statePath)

	if report.StateCheck.Status != CheckWarn {
		t.Fatalf("StateCheck.Status = %q, want %q", report.StateCheck.Status, CheckWarn)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("InspectAgentStatus created or changed missing state path, stat error = %v", err)
	}
}

func TestInspectAgentStatusReadsExistingStateReadOnly(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "state.db")
	store, err := agentstate.Open(statePath)
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	if err := store.UpdateRegistration("agent_123", "token_123", "https://core.example.com"); err != nil {
		t.Fatalf("update registration: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close state: %v", err)
	}

	report := InspectAgentStatus(statePath)

	if report.StateCheck.Status != CheckOK {
		t.Fatalf("StateCheck.Status = %q, error = %v, want %q", report.StateCheck.Status, report.StateCheck.Error, CheckOK)
	}
	if report.InternalState == nil {
		t.Fatal("InternalState = nil, want state")
	}
	if report.InternalState.AgentID != "agent_123" {
		t.Fatalf("AgentID = %q, want agent_123", report.InternalState.AgentID)
	}
	if !report.InternalState.IsRegistered() {
		t.Fatal("IsRegistered() = false, want true")
	}
}

func TestBuildServicePreflightReportsConfigAndMissingState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	statePath := filepath.Join(dir, "state.db")
	if err := os.WriteFile(configPath, []byte("core_url: https://core.example.com\ninterval: 60s\nmonitors: []\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	report := BuildServicePreflight(configPath, statePath)

	var configCheck DiagnosticCheck
	var stateCheck DiagnosticCheck
	for _, check := range report.Checks {
		switch check.Name {
		case "config":
			configCheck = check
		case "state":
			stateCheck = check
		}
	}
	if configCheck.Status != CheckOK {
		t.Fatalf("config check = %#v, want ok", configCheck)
	}
	if stateCheck.Status != CheckWarn {
		t.Fatalf("state check = %#v, want warn for missing state", stateCheck)
	}
	if report.HasErrors() && report.Service.Manager == ServiceManagerNone {
		t.Fatalf("HasErrors() = true with no service manager; report = %#v", report)
	}
}

func TestJSONLRecentLogReaderAdaptsLogEntries(t *testing.T) {
	t.Parallel()

	path := writeTestLogFile(t, []string{
		`{"time":"2026-05-26T10:00:00Z","level":"ERROR","component":"registration","msg":"failed","monitor":"api","error":"401 unauthorized"}`,
	})

	entries, err := (JSONLRecentLogReader{Path: path}).RecentLogs(RecentLogQuery{Lines: 1})
	if err != nil {
		t.Fatalf("RecentLogs() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("RecentLogs() returned %d entries, want 1", len(entries))
	}
	if entries[0].Level != "ERROR" || entries[0].Component != "registration" || entries[0].Message != "failed" {
		t.Fatalf("RecentLogs() entry = %#v", entries[0])
	}
	if entries[0].Fields["monitor"] != "api" || entries[0].Fields["error"] != "401 unauthorized" {
		t.Fatalf("RecentLogs() fields = %#v", entries[0].Fields)
	}
}
