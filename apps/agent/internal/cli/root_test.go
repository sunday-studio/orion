package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"orion/agent/internal/config"
	agentstate "orion/agent/internal/state"
)

func TestNormalizeLegacyArgsRewritesSingleDashLongFlags(t *testing.T) {
	t.Parallel()

	got := NormalizeLegacyArgs([]string{"run", "-once", "-config", "/tmp/config.yaml", "-state=/tmp/state.db"})
	want := []string{"run", "--once", "--config", "/tmp/config.yaml", "--state=/tmp/state.db"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeLegacyArgs() = %#v, want %#v", got, want)
	}
}

func TestNormalizeLegacyArgsRewritesVersionShortcut(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"-v"}, {"-version"}, {"--version"}} {
		got := NormalizeLegacyArgs(args)
		want := []string{"version"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("NormalizeLegacyArgs(%#v) = %#v, want %#v", args, got, want)
		}
	}
}

func TestNormalizeLegacyArgsRewritesMaintenanceDashActions(t *testing.T) {
	t.Parallel()

	got := NormalizeLegacyArgs([]string{"maintenance", "-down", "deploying", "-state", "/tmp/state.db"})
	want := []string{"maintenance", "down", "deploying", "--state", "/tmp/state.db"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeLegacyArgs() = %#v, want %#v", got, want)
	}
}

func TestCommandArgsIncludesGlobalFlagsBeforeCommand(t *testing.T) {
	t.Parallel()

	opts := &Options{normalizedArgs: []string{"--state", "/tmp/state.db", "maintenance", "up"}}

	got := commandArgs(opts, "maintenance")
	want := []string{"--state", "/tmp/state.db", "up"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commandArgs() = %#v, want %#v", got, want)
	}
	if commandNeedsElevation("maintenance", got) {
		t.Fatal("commandNeedsElevation() = true, want false with global --state")
	}
}

func TestNewRootCommandParsesGlobalFlagsBeforeCommand(t *testing.T) {
	t.Parallel()

	opts := &Options{}
	cmd := NewRootCommand(context.Background(), opts, nil, nil)
	cmd.SetArgs([]string{"--state", "/tmp/state.db", "version"})
	if err := cmd.ParseFlags([]string{"--state", "/tmp/state.db"}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}
	if opts.StatePath != "/tmp/state.db" {
		t.Fatalf("StatePath = %q, want /tmp/state.db", opts.StatePath)
	}
}

func TestExecuteDisablesColorForNonFileWriters(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Execute(context.Background(), []string{"config", "validate", "--config", "/tmp/missing-orion-config.yaml"}, &out, &errOut)
	if code == 0 {
		t.Fatal("Execute() code = 0, want failure for missing config")
	}
	combined := out.String() + errOut.String()
	if strings.Contains(combined, "\x1b[") {
		t.Fatalf("Execute() output contains ANSI escapes for buffer writer: %q", combined)
	}
}

func TestExecuteSetupWritesConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	statePath := filepath.Join(dir, "state.db")
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Execute(context.Background(), []string{
		"--config", configPath,
		"--state", statePath,
		"setup",
		"--core-url", "https://core.example.com",
		"--init-state",
		"--no-color",
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("Execute(setup) code = %d, err = %s, out = %s", code, errOut.String(), out.String())
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "core_url: https://core.example.com") {
		t.Fatalf("config = %s, want core_url", string(data))
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state was not initialized: %v", err)
	}
}

func TestExecuteStatusJSON(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	statePath := filepath.Join(t.TempDir(), "missing-state.db")

	code := Execute(context.Background(), []string{"--json", "--state", statePath, "status", "--no-color"}, &out, &errOut)
	_ = code
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("status output is not JSON: %v\n%s", err, out.String())
	}
	if payload["state_path"] != statePath {
		t.Fatalf("state_path = %v, want %s", payload["state_path"], statePath)
	}
	if _, ok := payload["service_running"].(bool); !ok {
		t.Fatalf("service_running missing or not bool: %#v", payload)
	}
}

func TestExecuteConfigShowJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("core_url: https://core.example.com\ninterval: 60s\nmonitors: []\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Execute(context.Background(), []string{"--json", "--config", configPath, "config", "show", "--no-color"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("Execute(config show --json) code = %d, err = %s", code, errOut.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("config output is not JSON: %v\n%s", err, out.String())
	}
	if payload["core_url"] != "https://core.example.com" {
		t.Fatalf("core_url = %v", payload["core_url"])
	}
}

func TestExecuteTokenApplyPreservesLocalState(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.db")
	tokenPath := filepath.Join(dir, "replacement-token")
	store, err := agentstate.Open(statePath)
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	if err := store.UpdateRegistration("agent-1", "token-1", "https://core.example.com"); err != nil {
		t.Fatalf("UpdateRegistration() error = %v", err)
	}
	if err := store.ReplaceMonitors([]config.InternalStateMonitor{
		{Name: "homepage", ID: "monitor-1", Status: "running"},
	}); err != nil {
		t.Fatalf("ReplaceMonitors() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := os.WriteFile(tokenPath, []byte("token-2\n"), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Execute(context.Background(), []string{
		"--state", statePath,
		"--no-color",
		"token", "apply", "--token-file", tokenPath,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("Execute(token apply) code = %d, err = %s, out = %s", code, errOut.String(), out.String())
	}
	if strings.Contains(out.String(), "token-2") || strings.Contains(errOut.String(), "token-2") {
		t.Fatalf("token apply output leaked replacement token: out=%q err=%q", out.String(), errOut.String())
	}

	reopened, err := agentstate.Open(statePath)
	if err != nil {
		t.Fatalf("reopen state: %v", err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	state, err := reopened.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if state.AgentID != "agent-1" || state.Token != "token-2" || state.CoreURL != "https://core.example.com" {
		t.Fatalf("state = %+v, want preserved identity with replacement token", state)
	}
	if len(state.Monitors) != 1 || state.Monitors[0].ID != "monitor-1" {
		t.Fatalf("monitors = %+v, want preserved mappings", state.Monitors)
	}
}
