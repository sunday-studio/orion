package cli

import "testing"

func TestParseStateCommand(t *testing.T) {
	t.Parallel()

	subcommand, statePath, err := parseStateCommand([]string{"init", "-state", "/tmp/orion-state.db"}, "state.db")
	if err != nil {
		t.Fatalf("parse state command: %v", err)
	}
	if subcommand != "init" {
		t.Fatalf("subcommand = %q, want init", subcommand)
	}
	if statePath != "/tmp/orion-state.db" {
		t.Fatalf("state path = %q, want /tmp/orion-state.db", statePath)
	}
}

func TestParseStateCommandRejectsUnexpectedArgument(t *testing.T) {
	t.Parallel()

	_, _, err := parseStateCommand([]string{"init", "extra"}, "state.db")
	if err == nil {
		t.Fatal("expected unexpected argument error")
	}
}

func TestParseMaintenanceCommandAcceptsPlainActions(t *testing.T) {
	t.Parallel()

	action, reason, statePath, err := parseMaintenanceCommand([]string{"down", "updating", "configs"}, "/var/lib/orion/state.db")
	if err != nil {
		t.Fatalf("parseMaintenanceCommand() error = %v", err)
	}
	if action != "-down" {
		t.Fatalf("action = %q, want -down", action)
	}
	if reason != "updating configs" {
		t.Fatalf("reason = %q, want updating configs", reason)
	}
	if statePath != "/var/lib/orion/state.db" {
		t.Fatalf("statePath = %q, want default path", statePath)
	}

	action, reason, _, err = parseMaintenanceCommand([]string{"up"}, "/var/lib/orion/state.db")
	if err != nil {
		t.Fatalf("parseMaintenanceCommand() error = %v", err)
	}
	if action != "-up" {
		t.Fatalf("action = %q, want -up", action)
	}
	if reason != "" {
		t.Fatalf("reason = %q, want empty", reason)
	}
}

func TestParseMaintenanceCommandStillAcceptsLegacyDashActions(t *testing.T) {
	t.Parallel()

	action, reason, statePath, err := parseMaintenanceCommand([]string{"-down", "maintenance", "-state", "/tmp/state.db"}, "/var/lib/orion/state.db")
	if err != nil {
		t.Fatalf("parseMaintenanceCommand() error = %v", err)
	}
	if action != "-down" {
		t.Fatalf("action = %q, want -down", action)
	}
	if reason != "maintenance" {
		t.Fatalf("reason = %q, want maintenance", reason)
	}
	if statePath != "/tmp/state.db" {
		t.Fatalf("statePath = %q, want /tmp/state.db", statePath)
	}
}
