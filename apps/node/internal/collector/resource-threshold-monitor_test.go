package collector

import (
	"errors"
	"testing"
)

func TestRunResourceThresholdMonitorWithCollector(t *testing.T) {
	t.Run("all resources under thresholds", func(t *testing.T) {
		result := runResourceThresholdMonitorWithCollector(
			ResourceThresholdConfig{
				MaxCPUPercent:    90,
				MaxMemoryPercent: 90,
				MaxDiskPercent:   90,
				MaxLoad1:         10,
			},
			func() (*SystemMetrics, error) {
				return sampleSystemMetrics(), nil
			},
		)

		if result.Status != "up" {
			t.Fatalf("status = %q, want up", result.Status)
		}
	})

	t.Run("threshold exceeded", func(t *testing.T) {
		result := runResourceThresholdMonitorWithCollector(
			ResourceThresholdConfig{
				MaxCPUPercent:    50,
				MaxMemoryPercent: 80,
				MaxDiskPercent:   90,
				MaxLoad1:         10,
			},
			func() (*SystemMetrics, error) {
				return sampleSystemMetrics(), nil
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Metrics["failure_reason"] != "resource threshold exceeded" {
			t.Fatalf("failure_reason = %v, want resource threshold exceeded", result.Metrics["failure_reason"])
		}
		violations, ok := result.Metrics["threshold_violations"].([]string)
		if !ok || len(violations) == 0 {
			t.Fatalf("threshold_violations = %#v, want non-empty []string", result.Metrics["threshold_violations"])
		}
	})

	t.Run("collector failure", func(t *testing.T) {
		result := runResourceThresholdMonitorWithCollector(
			ResourceThresholdConfig{MaxCPUPercent: 90},
			func() (*SystemMetrics, error) {
				return nil, errors.New("collector failed")
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Error == nil || result.Error.Message != "collector failed" {
			t.Fatalf("error = %+v, want collector failed", result.Error)
		}
	})
}

func sampleSystemMetrics() *SystemMetrics {
	return &SystemMetrics{
		CPU:    CPUStats{UsagePercent: 75, Load1: 2},
		Memory: MemoryStats{UsedPercent: 70},
		Disk:   DiskStats{UsedPercent: 65},
	}
}
