package collector

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunCommandMonitorWithRunner(t *testing.T) {
	t.Run("zero exit is up", func(t *testing.T) {
		result := runCommandMonitorWithRunner(
			CommandMonitorConfig{Command: "test -f /tmp/backup-ok", Timeout: time.Second},
			func(ctx context.Context, command string) commandExecution {
				assertCommandContext(t, ctx, command, "test -f /tmp/backup-ok")
				return commandExecution{ExitCode: 0, Stdout: "ok\n"}
			},
		)

		if result.Status != "up" {
			t.Fatalf("status = %q, want up", result.Status)
		}
		if result.Metrics["stdout"] != "ok\n" {
			t.Fatalf("stdout = %v, want ok newline", result.Metrics["stdout"])
		}
	})

	t.Run("non-zero exit is down", func(t *testing.T) {
		result := runCommandMonitorWithRunner(
			CommandMonitorConfig{Command: "exit 2", Timeout: time.Second},
			func(ctx context.Context, command string) commandExecution {
				return commandExecution{ExitCode: 2, Stderr: "missing file\n"}
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Metrics["exit_code"] != 2 {
			t.Fatalf("exit_code = %v, want 2", result.Metrics["exit_code"])
		}
		if result.Metrics["failure_reason"] != "command exited non-zero" {
			t.Fatalf("failure_reason = %v, want command exited non-zero", result.Metrics["failure_reason"])
		}
	})

	t.Run("timeout is down", func(t *testing.T) {
		result := runCommandMonitorWithRunner(
			CommandMonitorConfig{Command: "sleep 10", Timeout: time.Second},
			func(ctx context.Context, command string) commandExecution {
				return commandExecution{ExitCode: -1, TimedOut: true}
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Error == nil || result.Error.Message != "command timed out" {
			t.Fatalf("error = %+v, want command timed out", result.Error)
		}
	})

	t.Run("execution error is down", func(t *testing.T) {
		result := runCommandMonitorWithRunner(
			CommandMonitorConfig{Command: "backup-check", Timeout: time.Second},
			func(ctx context.Context, command string) commandExecution {
				return commandExecution{ExitCode: -1, Err: errors.New("shell unavailable")}
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Error == nil || result.Error.Message != "shell unavailable" {
			t.Fatalf("error = %+v, want shell unavailable", result.Error)
		}
	})
}

func assertCommandContext(t *testing.T, ctx context.Context, command string, wantCommand string) {
	t.Helper()

	if command != wantCommand {
		t.Fatalf("command = %q, want %q", command, wantCommand)
	}
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("context has no deadline")
	}
}
