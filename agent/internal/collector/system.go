// internal/collector/system.go
package collector

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"orion/agent/internal/services"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemMetrics struct {
	Hostname      string                `json:"hostname"`
	OS            string                `json:"os"`
	Platform      string                `json:"platform"`
	Arch          string                `json:"arch"`
	Kernel        string                `json:"kernel_version"`
	UptimeSeconds uint64                `json:"uptime_seconds"`
	Timestamp     string                `json:"timestamp"`
	CPU           CPUStats              `json:"cpu"`
	Memory        MemoryStats           `json:"memory"`
	Disk          DiskStats             `json:"disk"`
	Location      *services.GeoLocation `json:"location,omitempty"`
}

type CPUStats struct {
	Cores        int     `json:"cores"`
	UsagePercent float64 `json:"usage_percent"`
	Load1        float64 `json:"load_1"`
	Load5        float64 `json:"load_5"`
	Load15       float64 `json:"load_15"`
}

type MemoryStats struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type DiskStats struct {
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type GeoLocation struct {
	IP       string `json:"ip"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Location string `json:"loc"` // "lat,long"
	Timezone string `json:"timezone"`
}

// Collect gathers system statistics from the host machine.
func Collect() (*SystemMetrics, error) {
	hostname, _ := os.Hostname()

	hostInfo, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("host info error: %w", err)
	}

	// CPU
	cpuUsage, err := cpu.Percent(0, false)
	if err != nil {
		return nil, fmt.Errorf("cpu usage error: %w", err)
	}

	cpuInfo, _ := cpu.Info()
	loadAvg, _ := load.Avg()

	// Memory
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("memory info error: %w", err)
	}

	// Disk (root filesystem)
	diskInfo, err := disk.Usage("/")
	if err != nil {
		return nil, fmt.Errorf("disk usage error: %w", err)
	}

	metrics := &SystemMetrics{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Platform: hostInfo.Platform,
		Arch:     runtime.GOARCH,
		Kernel:   hostInfo.KernelVersion,

		UptimeSeconds: hostInfo.Uptime,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),

		CPU: CPUStats{
			Cores:        len(cpuInfo),
			UsagePercent: cpuUsage[0],
			Load1:        loadAvg.Load1,
			Load5:        loadAvg.Load5,
			Load15:       loadAvg.Load15,
		},

		Memory: MemoryStats{
			TotalBytes:     memInfo.Total,
			UsedBytes:      memInfo.Used,
			FreeBytes:      memInfo.Free,
			AvailableBytes: memInfo.Available,
			UsedPercent:    memInfo.UsedPercent,
		},

		Disk: DiskStats{
			TotalBytes:  diskInfo.Total,
			UsedBytes:   diskInfo.Used,
			FreeBytes:   diskInfo.Free,
			UsedPercent: diskInfo.UsedPercent,
		},
	}

	if loc, err := services.GetLocation(); err == nil {
		metrics.Location = loc
	}

	return metrics, nil
}
