package collector

import (
	"errors"
	"testing"
)

func TestRunSystemdServiceMonitorWithRunner(t *testing.T) {
	t.Run("active service is up", func(t *testing.T) {
		result := runSystemdServiceMonitorWithRunner(
			SystemdServiceConfig{Name: "nginx.service"},
			func(name string, args ...string) ([]byte, error) {
				assertSystemctlShowCommand(t, name, args, "nginx.service")
				return []byte("LoadState=loaded\nActiveState=active\nSubState=running\nResult=success\n"), nil
			},
		)

		if result.Status != "up" {
			t.Fatalf("status = %q, want up", result.Status)
		}
		if result.Metrics["active_state"] != "active" {
			t.Fatalf("active_state = %v, want active", result.Metrics["active_state"])
		}
	})

	t.Run("inactive service is down", func(t *testing.T) {
		result := runSystemdServiceMonitorWithRunner(
			SystemdServiceConfig{Name: "nginx.service"},
			func(name string, args ...string) ([]byte, error) {
				return []byte("LoadState=loaded\nActiveState=failed\nSubState=failed\nResult=exit-code\n"), nil
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Metrics["failure_reason"] != "service is not active" {
			t.Fatalf("failure_reason = %v, want service is not active", result.Metrics["failure_reason"])
		}
	})

	t.Run("systemctl failure is down", func(t *testing.T) {
		result := runSystemdServiceMonitorWithRunner(
			SystemdServiceConfig{Name: "nginx.service"},
			func(name string, args ...string) ([]byte, error) {
				return nil, errors.New("systemctl unavailable")
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Error == nil || result.Error.Message != "systemctl unavailable" {
			t.Fatalf("error = %+v, want systemctl unavailable", result.Error)
		}
	})
}

func assertSystemctlShowCommand(t *testing.T, name string, args []string, service string) {
	t.Helper()

	if name != "systemctl" {
		t.Fatalf("command = %q, want systemctl", name)
	}
	if len(args) < 2 || args[0] != "show" || args[1] != service {
		t.Fatalf("args = %#v, want prefix [show %s]", args, service)
	}
}
