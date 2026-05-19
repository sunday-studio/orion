package cli

import (
	"strings"
	"testing"
)

func TestServiceRootErrorMentionsPrivilegePrompt(t *testing.T) {
	t.Parallel()

	err := serviceRootError("start")
	if err == nil {
		t.Fatal("serviceRootError() returned nil")
	}

	want := "orion-agent start"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("serviceRootError() = %q, want it to contain %q", err.Error(), want)
	}
	if strings.Contains(err.Error(), "sudo orion-agent") {
		t.Fatalf("serviceRootError() = %q, should not ask users to type sudo", err.Error())
	}
}

func TestServiceCommandErrorExplainsMissingSystemdUnit(t *testing.T) {
	t.Parallel()

	err := serviceCommandError("start", "Failed to start orion-agent.service: Unit orion-agent.service not found.\n")
	if err == nil {
		t.Fatal("serviceCommandError() returned nil")
	}

	for _, want := range []string{
		"systemd service is not installed",
		"/etc/systemd/system/orion-agent.service",
		"orion-agent run -once",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("serviceCommandError() = %q, want it to contain %q", err.Error(), want)
		}
	}
}
