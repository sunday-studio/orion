package collector

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

type CommandMonitorConfig struct {
	Command string
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

func RunCommandMonitor(cfg CommandMonitorConfig) *CommandMonitorResult {
	return runCommandMonitorWithRunner(cfg, runShellCommand)
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
		"stdout":      execution.Stdout,
		"stderr":      execution.Stderr,
		"duration_ms": duration,
		"timed_out":   execution.TimedOut,
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
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

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
