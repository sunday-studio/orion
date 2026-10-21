package collector

import (
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/process"
)

// RunDiskUsageMonitor checks the used percent of the filesystem holding path.
func RunDiskUsageMonitor(path string, maxPercent float64) *MonitorResult {
	return runDiskUsage(path, maxPercent, func(p string) (float64, error) {
		u, err := disk.Usage(p)
		if err != nil {
			return 0, err
		}
		return u.UsedPercent, nil
	})
}

func runDiskUsage(path string, maxPercent float64, usage func(string) (float64, error)) *MonitorResult {
	now := time.Now().UTC()
	used, err := usage(path)
	if err != nil {
		return &MonitorResult{Status: "down", Timestamp: now, Metrics: map[string]any{"path": path, "failure_reason": "could not read disk usage"}, Error: &MonitorError{Message: err.Error()}}
	}
	m := map[string]any{"path": path, "used_percent": used, "max_percent": maxPercent}
	status := "up"
	if used > maxPercent {
		status = "down"
		m["failure_reason"] = fmt.Sprintf("disk usage %.1f%% > %.1f%%", used, maxPercent)
	}
	return &MonitorResult{Status: status, Timestamp: now, Metrics: m}
}

// RunProcessMonitor checks that at least one process has the given executable name.
func RunProcessMonitor(name string) *MonitorResult {
	return runProcess(name, func() ([]string, error) {
		ps, err := process.Processes()
		if err != nil {
			return nil, err
		}
		var names []string
		for _, p := range ps {
			if n, err := p.Name(); err == nil {
				names = append(names, n)
			}
		}
		return names, nil
	})
}

func runProcess(name string, list func() ([]string, error)) *MonitorResult {
	now := time.Now().UTC()
	names, err := list()
	if err != nil {
		return &MonitorResult{Status: "down", Timestamp: now, Metrics: map[string]any{"process_name": name, "failure_reason": "could not list processes"}, Error: &MonitorError{Message: err.Error()}}
	}
	count := 0
	for _, n := range names {
		if strings.EqualFold(n, name) {
			count++
		}
	}
	m := map[string]any{"process_name": name, "process_count": count}
	status := "up"
	if count == 0 {
		status = "down"
		m["failure_reason"] = "no process with that name is running"
	}
	return &MonitorResult{Status: status, Timestamp: now, Metrics: m}
}
