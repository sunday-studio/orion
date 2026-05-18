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
