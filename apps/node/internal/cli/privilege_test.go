package cli

import "testing"

func TestCommandNeedsElevationForInstalledCommands(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"start", "stop", "restart", "update", "maintenance", "reconfigure", "state", "token"} {
		if !commandNeedsElevation(command, nil) {
			t.Fatalf("commandNeedsElevation(%q) = false, want true", command)
		}
	}
}

func TestCommandNeedsElevationRespectsExplicitPaths(t *testing.T) {
	t.Parallel()

	if commandNeedsElevation("maintenance", []string{"up", "-state", "/tmp/state.db"}) {
		t.Fatal("commandNeedsElevation(maintenance -state /tmp/state.db) = true, want false")
	}
	if commandNeedsElevation("token", []string{"apply", "token-2", "--state", "/tmp/state.db"}) {
		t.Fatal("commandNeedsElevation(token apply --state /tmp/state.db) = true, want false")
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

	for _, command := range []string{"help", "version", "--version", "logs", "status", "config"} {
		if commandNeedsElevation(command, nil) {
			t.Fatalf("commandNeedsElevation(%q) = true, want false", command)
		}
	}
}
