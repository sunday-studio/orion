// internal/collector/system.go
package collector

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemMetrics struct {
	Hostname  string  `json:"hostname"`
	OS        string  `json:"os"`
	Platform  string  `json:"platform"`
	CPUUsage  float64 `json:"cpu_usage"`
	MemUsage  float64 `json:"mem_usage"`
	DiskUsage float64 `json:"disk_usage"`
	Uptime    uint64  `json:"uptime"`
	Timestamp string  `json:"timestamp"`
}

// Collect gathers system statistics from the host machine.
func Collect() (*SystemMetrics, error) {
	hostInfo, err := host.Info()
	if err != nil {
		return nil, err
	}

	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		return nil, err
	}

	memStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	diskStats, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}

	metrics := &SystemMetrics{
		Hostname:  hostInfo.Hostname,
		OS:        runtime.GOOS,
		Platform:  hostInfo.Platform,
		CPUUsage:  round(cpuPercent[0], 2),
		MemUsage:  round(memStats.UsedPercent, 2),
		MemFree:   round(memStats.Free), 2),
		DiskUsage: round(diskStats.UsedPercent, 2),
		Uptime:    hostInfo.Uptime,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return metrics, nil
}

// round trims float64 to n decimal places.
func round(f float64, n int) float64 {
	pow := 1.0
	for i := 0; i < n; i++ {
		pow *= 10
	}
	return float64(int(f*pow+0.5)) / pow
}
