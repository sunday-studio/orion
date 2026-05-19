package cli

import "testing"

func TestCommandNeedsElevationForInstalledCommands(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"start", "stop", "restart", "logs", "update", "maintenance", "reconfigure", "state", "status", "config"} {
		if !commandNeedsElevation(command, nil) {
			t.Fatalf("commandNeedsElevation(%q) = false, want true", command)
		}
	}
}

func TestCommandNeedsElevationRespectsExplicitPaths(t *testing.T) {
	t.Parallel()

	if commandNeedsElevation("config", []string{"validate", "-config", "/tmp/config.yaml"}) {
		t.Fatal("commandNeedsElevation(config -config /tmp/config.yaml) = true, want false")
	}
	if commandNeedsElevation("maintenance", []string{"up", "-state", "/tmp/state.db"}) {
		t.Fatal("commandNeedsElevation(maintenance -state /tmp/state.db) = true, want false")
	}
	if commandNeedsElevation("status", []string{"--state=/tmp/state.db"}) {
		t.Fatal("commandNeedsElevation(status --state=/tmp/state.db) = true, want false")
	}
}

func TestCommandNeedsElevationOnlyForRunOnce(t *testing.T) {
	t.Parallel()

	if commandNeedsElevation("run", nil) {
		t.Fatal("commandNeedsElevation(run) = true, want false")
	}
	if !commandNeedsElevation("run", []string{"-once"}) {
		t.Fatal("commandNeedsElevation(run -once) = false, want true")
	}
	if !commandNeedsElevation("run", []string{"--once=true"}) {
		t.Fatal("commandNeedsElevation(run --once=true) = false, want true")
	}
	if commandNeedsElevation("run", []string{"-once", "-config", "/tmp/config.yaml", "-state", "/tmp/state.db"}) {
		t.Fatal("commandNeedsElevation(run -once with explicit paths) = true, want false")
	}
}

func TestCommandNeedsElevationIgnoresReadOnlyCommands(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"help", "version", "--version"} {
		if commandNeedsElevation(command, nil) {
			t.Fatalf("commandNeedsElevation(%q) = true, want false", command)
		}
	}
}
