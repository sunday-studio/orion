package collector

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

const maxCommandOutputBytes = 16 * 1024

type CommandMonitorConfig struct {
	Command string
	Args    []string
	Timeout time.Duration
}

type CommandMonitorResult struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	Error     *MonitorError          `json:"error,omitempty"`
}

type commandExecution struct {
	ExitCode int
	Stdout   string
	Stderr   string
	TimedOut bool
	Err      error
}

type commandMonitorRunner func(context.Context, string) commandExecution

type commandSpec struct {
	binary string
	args   []string
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		b.truncated = true
		_, err := b.buf.Write(p[:remaining])
		return len(p), err
	}
	_, err := b.buf.Write(p)
	return len(p), err
}

func (b *limitedBuffer) String() string {
	return b.buf.String()
}

func RunCommandMonitor(cfg CommandMonitorConfig) *CommandMonitorResult {
	return runCommandMonitorWithRunner(cfg, func(ctx context.Context, command string) commandExecution {
		if len(cfg.Args) > 0 {
			return runCommandSpec(ctx, commandSpec{binary: command, args: cfg.Args})
		}
		return runShellCommand(ctx, command)
	})
}

func runCommandMonitorWithRunner(cfg CommandMonitorConfig, runner commandMonitorRunner) *CommandMonitorResult {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	execution := runner(ctx, cfg.Command)
	stdout, stdoutTruncated := truncateCommandOutput(execution.Stdout)
	stderr, stderrTruncated := truncateCommandOutput(execution.Stderr)
	duration := time.Since(startedAt).Milliseconds()

	status := "up"
	var monitorError *MonitorError
	failureReason := ""
	if execution.TimedOut {
		status = "down"
		failureReason = "command timed out"
		monitorError = &MonitorError{Message: failureReason}
	} else if execution.Err != nil {
		status = "down"
		failureReason = "command failed"
		monitorError = &MonitorError{Message: execution.Err.Error()}
	} else if execution.ExitCode != 0 {
		status = "down"
		failureReason = "command exited non-zero"
	}

	metrics := map[string]interface{}{
		"exit_code":   execution.ExitCode,
		"stdout":      stdout,
		"stderr":      stderr,
		"duration_ms": duration,
		"timed_out":   execution.TimedOut,
	}
	if stdoutTruncated {
		metrics["stdout_truncated"] = true
	}
	if stderrTruncated {
		metrics["stderr_truncated"] = true
	}
	if failureReason != "" {
		metrics["failure_reason"] = failureReason
	}

	return &CommandMonitorResult{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Metrics:   metrics,
		Error:     monitorError,
	}
}

func runShellCommand(ctx context.Context, command string) commandExecution {
	spec, err := parseCommandLine(command)
	if err != nil {
		return commandExecution{ExitCode: -1, Err: err}
	}
	return runCommandSpec(ctx, spec)
}

func runCommandSpec(ctx context.Context, spec commandSpec) commandExecution {
	cmd := exec.CommandContext(ctx, spec.binary, spec.args...)
	stdout := &limitedBuffer{limit: maxCommandOutputBytes + 1}
	stderr := &limitedBuffer{limit: maxCommandOutputBytes + 1}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
			err = nil
		}
	}

	return commandExecution{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		TimedOut: ctx.Err() == context.DeadlineExceeded,
		Err:      err,
	}
}

func parseCommandLine(command string) (commandSpec, error) {
	fields, err := splitCommandLine(command)
	if err != nil {
		return commandSpec{}, err
	}
	if len(fields) == 0 {
		return commandSpec{}, errors.New("command is required")
	}
	return commandSpec{binary: fields[0], args: fields[1:]}, nil
}

func splitCommandLine(command string) ([]string, error) {
	var fields []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for _, r := range command {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\' && !inSingleQuote:
			escaped = true
		case r == '\'' && !inDoubleQuote:
			inSingleQuote = !inSingleQuote
		case r == '"' && !inSingleQuote:
			inDoubleQuote = !inDoubleQuote
		case isCommandSpace(r) && !inSingleQuote && !inDoubleQuote:
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if escaped {
		current.WriteRune('\\')
	}
	if inSingleQuote || inDoubleQuote {
		return nil, errors.New("command contains an unclosed quote")
	}
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}
	return fields, nil
}

func isCommandSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func truncateCommandOutput(value string) (string, bool) {
	if len(value) <= maxCommandOutputBytes {
		return value, false
	}
	return value[:maxCommandOutputBytes], true
}
