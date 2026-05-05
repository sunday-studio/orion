package collector

import (
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

type InternalServiceMonitorConfig struct {
	Name     string
	Interval time.Duration
	Ping     PingConfig
	Process  PortProcessConfig
}

type PingConfig struct {
	URL     string
	Timeout time.Duration
}

type PortProcessConfig struct {
	Port int
}

type InternalServiceMonitorResult struct {
	Status    string                 `json:"status"` // up | down
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	Error     *MonitorError          `json:"error,omitempty"`
}

type MonitorError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func RunInternalServiceMonitor(cfg InternalServiceMonitorConfig) *InternalServiceMonitorResult {
	start := time.Now()

	client := &http.Client{
		Timeout: cfg.Ping.Timeout,
	}

	req, err := http.NewRequest(http.MethodGet, cfg.Ping.URL, nil)
	if err != nil {
		return internalServiceFailureResult(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return internalServiceFailureResult(err)
	}
	resp.Body.Close()

	latency := time.Since(start).Milliseconds()

	metrics := map[string]interface{}{
		"ping": map[string]interface{}{
			"latency_ms": latency,
		},
	}

	// --- Port → PID → Memory ---
	pid := pidFromPort(cfg.Process.Port)
	if pid == 0 {
		metrics["process"] = map[string]interface{}{
			"found": false,
		}
	} else {
		mem := memoryForPID(pid)
		metrics["process"] = map[string]interface{}{
			"pid":          pid,
			"memory_bytes": mem,
		}
	}

	return &InternalServiceMonitorResult{
		Status:    "up",
		Timestamp: time.Now().UTC(),
		Metrics:   metrics,
	}
}

func internalServiceFailureResult(err error) *InternalServiceMonitorResult {
	return &InternalServiceMonitorResult{
		Status:    "down",
		Timestamp: time.Now().UTC(),
		Error: &MonitorError{
			Message: err.Error(),
		},
	}
}

func pidFromPort(port int) int {
	cmd := exec.Command("lsof", "-t", "-i", fmt.Sprintf(":%d", port))
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	pidStr := strings.TrimSpace(string(out))
	pid, err := strconv.Atoi(pidStr)

	if err != nil {
		return 0
	}

	return pid
}

func memoryForPID(pid int) uint64 {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return 0
	}

	mem, err := p.MemoryInfo()
	if err != nil {
		return 0
	}

	return mem.RSS
}
