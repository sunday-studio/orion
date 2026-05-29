package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strconv"
	"strings"
	"time"
)

type tcpConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type tcpResult struct {
	Health       string
	FinishedAt   time.Time
	Duration     time.Duration
	Host         string
	Port         int
	Address      string
	Error        error
	FailureStage string
}

func (a *App) runTCPCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) tcpResult {
	startedAt := time.Now()
	result := tcpResult{
		Health:     "down",
		FinishedAt: startedAt.UTC(),
	}

	runnerConfig, err := parseTCPConfig(monitorConfig.ConfigJSON)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	result.Host = runnerConfig.Host
	result.Port = runnerConfig.Port
	result.Address = net.JoinHostPort(runnerConfig.Host, strconv.Itoa(runnerConfig.Port))
	if err := a.targetPolicy.ValidateHost(runnerConfig.Host, "host"); err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}

	timeout := time.Duration(monitorConfig.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := a.tcpDialContext(checkCtx, "tcp", result.Address)
	result.FinishedAt = time.Now().UTC()
	result.Duration = time.Since(startedAt)
	if err != nil {
		result.Error = err
		result.FailureStage = tcpFailureStage(err)
		return result
	}
	if conn != nil {
		_ = conn.Close()
	}
	result.Health = "up"
	return result
}

func parseTCPConfig(raw string) (tcpConfig, error) {
	var cfg tcpConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("parse config json: %w", err)
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Host == "" {
		return cfg, fmt.Errorf("host is required")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return cfg, fmt.Errorf("port must be between 1 and 65535")
	}
	return cfg, nil
}

func tcpFailureStage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	return "connect"
}

func (a *App) storeTCPReport(monitorID string, result tcpResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   tcpPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = tcpPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func tcpPayload(result tcpResult, resultErr error) map[string]interface{} {
	payload := map[string]interface{}{
		"runner":        "core",
		"type":          "tcp",
		"host":          result.Host,
		"port":          result.Port,
		"address":       result.Address,
		"duration_ms":   result.Duration.Milliseconds(),
		"ok":            result.Health == "up",
		"collected_at":  result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage": result.FailureStage,
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
