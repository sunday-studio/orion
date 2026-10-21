package collector

import (
	"fmt"
	"time"
)

type ResourceThresholdConfig struct {
	MaxCPUPercent    float64
	MaxMemoryPercent float64
	MaxDiskPercent   float64
	MaxLoad1         float64
}

type ResourceThresholdResult struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	Error     *MonitorError          `json:"error,omitempty"`
}

type systemMetricsFunc func() (*SystemMetrics, error)

func RunResourceThresholdMonitor(cfg ResourceThresholdConfig) *ResourceThresholdResult {
	return runResourceThresholdMonitorWithCollector(cfg, Collect)
}

func runResourceThresholdMonitorWithCollector(cfg ResourceThresholdConfig, collect systemMetricsFunc) *ResourceThresholdResult {
	metrics, err := collect()
	if err != nil {
		return &ResourceThresholdResult{
			Status:    "down",
			Timestamp: time.Now().UTC(),
			Metrics: map[string]interface{}{
				"failure_reason": "system metrics collection failed",
			},
			Error: &MonitorError{Message: err.Error()},
		}
	}

	violations := make([]string, 0, 4)
	if cfg.MaxCPUPercent > 0 && metrics.CPU.UsagePercent > cfg.MaxCPUPercent {
		violations = append(violations, fmt.Sprintf("cpu usage %.2f%% > %.2f%%", metrics.CPU.UsagePercent, cfg.MaxCPUPercent))
	}
	if cfg.MaxMemoryPercent > 0 && metrics.Memory.UsedPercent > cfg.MaxMemoryPercent {
		violations = append(violations, fmt.Sprintf("memory usage %.2f%% > %.2f%%", metrics.Memory.UsedPercent, cfg.MaxMemoryPercent))
	}
	if cfg.MaxDiskPercent > 0 && metrics.Disk.UsedPercent > cfg.MaxDiskPercent {
		violations = append(violations, fmt.Sprintf("disk usage %.2f%% > %.2f%%", metrics.Disk.UsedPercent, cfg.MaxDiskPercent))
	}
	if cfg.MaxLoad1 > 0 && metrics.CPU.Load1 > cfg.MaxLoad1 {
		violations = append(violations, fmt.Sprintf("load1 %.2f > %.2f", metrics.CPU.Load1, cfg.MaxLoad1))
	}

	status := "up"
	resultMetrics := map[string]interface{}{
		"cpu_usage_percent":    metrics.CPU.UsagePercent,
		"memory_used_percent":  metrics.Memory.UsedPercent,
		"disk_used_percent":    metrics.Disk.UsedPercent,
		"load_1":               metrics.CPU.Load1,
		"threshold_violations": violations,
	}
	if len(violations) > 0 {
		status = "down"
		resultMetrics["failure_reason"] = "resource threshold exceeded"
	}

	return &ResourceThresholdResult{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Metrics:   resultMetrics,
	}
}
