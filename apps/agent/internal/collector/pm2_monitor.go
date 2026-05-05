package collector

import (
	"encoding/json"
	"errors"
	"os/exec"
	"time"
)

type PM2MonitorConfig struct {
	AppName string
}

type pm2Process struct {
	Name  string `json:"name"`
	PID   int    `json:"pid"`
	Monit struct {
		Memory uint64  `json:"memory"`
		CPU    float64 `json:"cpu"`
	} `json:"monit"`
	PM2Env struct {
		Status       string `json:"status"`
		RestartCount int    `json:"restart_time"`
		UptimeMS     int64  `json:"pm_uptime"`
	} `json:"pm2_env"`
}

type PM2MonitorResult struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Metrics   struct {
		CPUPercent    float64 `json:"cpu_percent"`
		MemoryBytes   uint64  `json:"memory_bytes"`
		UptimeSeconds uint64  `json:"uptime_seconds"`
		RestartCount  int     `json:"restart_count"`
		PID           int     `json:"pid"`
	} `json:"metrics,omitempty"`
	Error *MonitorError `json:"error,omitempty"`
}

func RunPM2Monitor(cfg PM2MonitorConfig) *PM2MonitorResult {
	cmd := exec.Command("pm2", "jlist")
	out, err := cmd.Output()

	if err != nil {
		return pm2Failure(err)
	}

	var processes []pm2Process
	if err := json.Unmarshal(out, &processes); err != nil {
		return pm2Failure(err)
	}

	for _, p := range processes {
		if p.Name != cfg.AppName {
			continue
		}

		if p.PM2Env.Status != "online" {
			return pm2Failure(errors.New("pm2 app not found"))
		}

		uptime := uint64(0)
		if p.PM2Env.UptimeMS > 0 {
			uptime = uint64(time.Since(time.UnixMilli(p.PM2Env.UptimeMS)).Seconds())
		}

		return &PM2MonitorResult{
			Status:    "up",
			Timestamp: time.Now().UTC(),
			Metrics: struct {
				CPUPercent    float64 `json:"cpu_percent"`
				MemoryBytes   uint64  `json:"memory_bytes"`
				UptimeSeconds uint64  `json:"uptime_seconds"`
				RestartCount  int     `json:"restart_count"`
				PID           int     `json:"pid"`
			}{
				CPUPercent:    p.Monit.CPU,
				MemoryBytes:   p.Monit.Memory,
				UptimeSeconds: uptime,
				RestartCount:  p.PM2Env.RestartCount,
				PID:           p.PID,
			},
		}
	}

	return pm2Failure(errors.New("pm2 app not found"))
}

func pm2Failure(err error) *PM2MonitorResult {
	return &PM2MonitorResult{
		Status:    "down",
		Timestamp: time.Now().UTC(),
		Error: &MonitorError{
			Message: err.Error(),
		},
	}
}
