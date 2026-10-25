package collector

import (
	"os/exec"
	"strings"
	"time"
)

type SystemdServiceConfig struct {
	Name string
}

type SystemdServiceResult struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	Error     *MonitorError          `json:"error,omitempty"`
}

func RunSystemdServiceMonitor(cfg SystemdServiceConfig) *SystemdServiceResult {
	return runSystemdServiceMonitorWithRunner(cfg, func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	})
}

func runSystemdServiceMonitorWithRunner(cfg SystemdServiceConfig, runner commandRunner) *SystemdServiceResult {
	output, err := runner("systemctl", "show", cfg.Name, "--property=LoadState,ActiveState,SubState,Result", "--no-page")
	if err != nil {
		return systemdServiceFailure("systemctl show failed", err)
	}

	properties := parseSystemdProperties(string(output))
	activeState := properties["ActiveState"]
	status := "up"
	var failureReason string
	if activeState != "active" {
		status = "down"
		failureReason = "service is not active"
	}

	metrics := map[string]interface{}{
		"service_name": cfg.Name,
		"load_state":   properties["LoadState"],
		"active_state": activeState,
		"sub_state":    properties["SubState"],
		"result":       properties["Result"],
	}
	if failureReason != "" {
		metrics["failure_reason"] = failureReason
	}

	return &SystemdServiceResult{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Metrics:   metrics,
	}
}

func parseSystemdProperties(output string) map[string]string {
	properties := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		properties[key] = value
	}
	return properties
}

func systemdServiceFailure(reason string, err error) *SystemdServiceResult {
	return &SystemdServiceResult{
		Status:    "down",
		Timestamp: time.Now().UTC(),
		Metrics: map[string]interface{}{
			"failure_reason": reason,
		},
		Error: &MonitorError{Message: err.Error()},
	}
}
