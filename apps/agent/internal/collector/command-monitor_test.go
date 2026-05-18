package collector

import (
	"context"
	"errors"
	"strings"
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

	t.Run("output is truncated", func(t *testing.T) {
		result := runCommandMonitorWithRunner(
			CommandMonitorConfig{Command: "noisy-check", Timeout: time.Second},
			func(ctx context.Context, command string) commandExecution {
				return commandExecution{
					ExitCode: 0,
					Stdout:   strings.Repeat("a", maxCommandOutputBytes+100),
					Stderr:   strings.Repeat("b", maxCommandOutputBytes+100),
				}
			},
		)

		stdout, ok := result.Metrics["stdout"].(string)
		if !ok {
			t.Fatalf("stdout type = %T, want string", result.Metrics["stdout"])
		}
		if len(stdout) != maxCommandOutputBytes {
			t.Fatalf("stdout length = %d, want %d", len(stdout), maxCommandOutputBytes)
		}
		if result.Metrics["stdout_truncated"] != true {
			t.Fatalf("stdout_truncated = %v, want true", result.Metrics["stdout_truncated"])
		}

		stderr, ok := result.Metrics["stderr"].(string)
		if !ok {
			t.Fatalf("stderr type = %T, want string", result.Metrics["stderr"])
		}
		if len(stderr) != maxCommandOutputBytes {
			t.Fatalf("stderr length = %d, want %d", len(stderr), maxCommandOutputBytes)
		}
		if result.Metrics["stderr_truncated"] != true {
			t.Fatalf("stderr_truncated = %v, want true", result.Metrics["stderr_truncated"])
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

func TestParseCommandLine(t *testing.T) {
	spec, err := parseCommandLine(`test -f "/tmp/backup ok"`)
	if err != nil {
		t.Fatalf("parseCommandLine() error = %v", err)
	}
	if spec.binary != "test" {
		t.Fatalf("binary = %q, want test", spec.binary)
	}
	if len(spec.args) != 2 || spec.args[0] != "-f" || spec.args[1] != "/tmp/backup ok" {
		t.Fatalf("args = %#v, want [-f /tmp/backup ok]", spec.args)
	}

	if _, err := parseCommandLine(`test -f "unterminated`); err == nil {
		t.Fatal("parseCommandLine() error = nil, want unclosed quote error")
	}
}
