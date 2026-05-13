package collector

import (
	"errors"
	"testing"
)

func TestRunDockerContainerMonitorWithRunner(t *testing.T) {
	t.Run("running container is up", func(t *testing.T) {
		result := runDockerContainerMonitorWithRunner(
			DockerContainerConfig{Name: "postgres"},
			func(name string, args ...string) ([]byte, error) {
				assertDockerInspectCommand(t, name, args, "postgres")
				return []byte(`[{"State":{"Status":"running","Running":true,"Restarting":false,"ExitCode":0,"StartedAt":"2026-01-01T00:00:00Z"}}]`), nil
			},
		)

		if result.Status != "up" {
			t.Fatalf("status = %q, want up", result.Status)
		}
		if result.Metrics["docker_status"] != "running" {
			t.Fatalf("docker_status = %v, want running", result.Metrics["docker_status"])
		}
	})

	t.Run("stopped container is down", func(t *testing.T) {
		result := runDockerContainerMonitorWithRunner(
			DockerContainerConfig{Name: "postgres"},
			func(name string, args ...string) ([]byte, error) {
				assertDockerInspectCommand(t, name, args, "postgres")
				return []byte(`[{"State":{"Status":"exited","Running":false,"Restarting":false,"ExitCode":1,"FinishedAt":"2026-01-01T00:01:00Z"}}]`), nil
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Metrics["failure_reason"] != "container is not running" {
			t.Fatalf("failure_reason = %v, want container is not running", result.Metrics["failure_reason"])
		}
	})

	t.Run("inspect failure is down", func(t *testing.T) {
		result := runDockerContainerMonitorWithRunner(
			DockerContainerConfig{Name: "postgres"},
			func(name string, args ...string) ([]byte, error) {
				return nil, errors.New("docker unavailable")
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Error == nil || result.Error.Message != "docker unavailable" {
			t.Fatalf("error = %+v, want docker unavailable", result.Error)
		}
	})
}

func assertDockerInspectCommand(t *testing.T, name string, args []string, container string) {
	t.Helper()

	if name != "docker" {
		t.Fatalf("command = %q, want docker", name)
	}
	if len(args) != 2 || args[0] != "inspect" || args[1] != container {
		t.Fatalf("args = %#v, want [inspect %s]", args, container)
	}
}
