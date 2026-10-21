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
		PM2Status     string  `json:"pm2_status"`
		FailureReason string  `json:"failure_reason,omitempty"`
	} `json:"metrics,omitempty"`
	Error *MonitorError `json:"error,omitempty"`
}

func RunPM2Monitor(cfg PM2MonitorConfig) *PM2MonitorResult {
	return runPM2MonitorWithRunner(cfg, func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	})
}

func runPM2MonitorWithRunner(cfg PM2MonitorConfig, runner commandRunner) *PM2MonitorResult {
	out, err := runner("pm2", "jlist")
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

		uptime := uint64(0)
		if p.PM2Env.UptimeMS > 0 {
			uptime = uint64(time.Since(time.UnixMilli(p.PM2Env.UptimeMS)).Seconds())
		}

		status := "up"
		var monitorError *MonitorError
		if p.PM2Env.Status != "online" {
			status = "down"
			monitorError = &MonitorError{Message: "pm2 app is not online"}
		}

		return &PM2MonitorResult{
			Status:    status,
			Timestamp: time.Now().UTC(),
			Metrics: struct {
				CPUPercent    float64 `json:"cpu_percent"`
				MemoryBytes   uint64  `json:"memory_bytes"`
				UptimeSeconds uint64  `json:"uptime_seconds"`
				RestartCount  int     `json:"restart_count"`
				PID           int     `json:"pid"`
				PM2Status     string  `json:"pm2_status"`
				FailureReason string  `json:"failure_reason,omitempty"`
			}{
				CPUPercent:    p.Monit.CPU,
				MemoryBytes:   p.Monit.Memory,
				UptimeSeconds: uptime,
				RestartCount:  p.PM2Env.RestartCount,
				PID:           p.PID,
				PM2Status:     p.PM2Env.Status,
				FailureReason: failureReasonFromError(monitorError),
			},
			Error: monitorError,
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

func failureReasonFromError(err *MonitorError) string {
	if err == nil {
		return ""
	}
	return err.Message
}
