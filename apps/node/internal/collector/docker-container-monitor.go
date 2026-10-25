package collector

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type DockerContainerConfig struct {
	Name string
}

type DockerContainerResult struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	Error     *MonitorError          `json:"error,omitempty"`
}

type commandRunner func(name string, args ...string) ([]byte, error)

type dockerInspectState struct {
	State struct {
		Status     string `json:"Status"`
		Running    bool   `json:"Running"`
		Restarting bool   `json:"Restarting"`
		ExitCode   int    `json:"ExitCode"`
		Error      string `json:"Error"`
		StartedAt  string `json:"StartedAt"`
		FinishedAt string `json:"FinishedAt"`
	} `json:"State"`
}

func RunDockerContainerMonitor(cfg DockerContainerConfig) *DockerContainerResult {
	return runDockerContainerMonitorWithRunner(cfg, func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).CombinedOutput()
	})
}

func runDockerContainerMonitorWithRunner(cfg DockerContainerConfig, runner commandRunner) *DockerContainerResult {
	output, err := runner("docker", "inspect", cfg.Name)
	if err != nil {
		if details := strings.TrimSpace(string(output)); details != "" {
			err = fmt.Errorf("%w: %s", err, details)
		}
		return dockerContainerFailure("docker inspect failed", err)
	}

	var inspected []dockerInspectState
	if err := json.Unmarshal(output, &inspected); err != nil {
		return dockerContainerFailure("docker inspect returned invalid JSON", err)
	}
	if len(inspected) == 0 {
		return dockerContainerFailure("docker inspect returned no containers", errors.New("container not found"))
	}

	state := inspected[0].State
	status := "up"
	var failureReason string
	if !state.Running || state.Restarting {
		status = "down"
		failureReason = "container is not running"
	}

	metrics := map[string]interface{}{
		"container_name": cfg.Name,
		"docker_status":  state.Status,
		"running":        state.Running,
		"restarting":     state.Restarting,
		"exit_code":      state.ExitCode,
		"started_at":     state.StartedAt,
		"finished_at":    state.FinishedAt,
	}
	if strings.TrimSpace(state.Error) != "" {
		metrics["container_error"] = state.Error
	}
	if failureReason != "" {
		metrics["failure_reason"] = failureReason
	}

	return &DockerContainerResult{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Metrics:   metrics,
	}
}

func dockerContainerFailure(reason string, err error) *DockerContainerResult {
	return &DockerContainerResult{
		Status:    "down",
		Timestamp: time.Now().UTC(),
		Metrics: map[string]interface{}{
			"failure_reason": reason,
		},
		Error: &MonitorError{Message: err.Error()},
	}
}
