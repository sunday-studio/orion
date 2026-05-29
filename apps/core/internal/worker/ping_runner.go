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

const (
	defaultPingMethod = "tcp"
	defaultPingPort   = 443
)

type pingConfig struct {
	Host   string `json:"host"`
	Method string `json:"method"`
	Port   int    `json:"port"`
}

type pingResult struct {
	Health            string
	FinishedAt        time.Time
	Duration          time.Duration
	Host              string
	Method            string
	Port              int
	Address           string
	Strategy          string
	Latency           time.Duration
	Reachable         bool
	ICMPSupported     bool
	RequiresPrivilege bool
	Unsupported       bool
	FallbackStrategy  string
	Error             error
	FailureStage      string
}

func (a *App) runPingCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) pingResult {
	startedAt := time.Now()
	result := pingResult{
		Health:     "down",
		FinishedAt: startedAt.UTC(),
	}

	runnerConfig, err := parsePingConfig(monitorConfig.ConfigJSON)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	result.Host = runnerConfig.Host
	result.Method = runnerConfig.Method
	result.Port = runnerConfig.Port
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

	switch runnerConfig.Method {
	case "tcp":
		result.Strategy = "tcp_connect"
		result.FallbackStrategy = "tcp_connect"
		result.Address = net.JoinHostPort(runnerConfig.Host, strconv.Itoa(runnerConfig.Port))
		conn, err := a.tcpDialContext(checkCtx, "tcp", result.Address)
		result.FinishedAt = time.Now().UTC()
		result.Duration = time.Since(startedAt)
		result.Latency = result.Duration
		if err != nil {
			result.Error = err
			result.FailureStage = pingFailureStage(err)
			return result
		}
		if conn != nil {
			_ = conn.Close()
		}
		result.Reachable = true
		result.Health = "up"
		return result
	case "icmp":
		result.Strategy = "icmp"
		result.FallbackStrategy = "none"
		result.RequiresPrivilege = true
		result.Unsupported = true
		result.FinishedAt = time.Now().UTC()
		result.Duration = time.Since(startedAt)
		result.Error = fmt.Errorf("ICMP ping requires raw socket permissions and is not enabled in this Core worker")
		result.FailureStage = "permission"
		return result
	default:
		result.FinishedAt = time.Now().UTC()
		result.Duration = time.Since(startedAt)
		result.Error = fmt.Errorf("unsupported ping method %s", runnerConfig.Method)
		result.FailureStage = "config"
		return result
	}
}

func parsePingConfig(raw string) (pingConfig, error) {
	var cfg pingConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("parse config json: %w", err)
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Host == "" {
		return cfg, fmt.Errorf("host is required")
	}
	cfg.Method = strings.ToLower(strings.TrimSpace(cfg.Method))
	if cfg.Method == "" {
		cfg.Method = defaultPingMethod
	}
	switch cfg.Method {
	case "tcp":
		if cfg.Port == 0 {
			cfg.Port = defaultPingPort
		}
		if cfg.Port < 1 || cfg.Port > 65535 {
			return cfg, fmt.Errorf("port must be between 1 and 65535")
		}
	case "icmp":
		if cfg.Port != 0 {
			return cfg, fmt.Errorf("port is unsupported for icmp ping")
		}
	default:
		return cfg, fmt.Errorf("method must be one of tcp, icmp")
	}
	return cfg, nil
}

func pingFailureStage(err error) string {
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

func (a *App) storePingReport(monitorID string, result pingResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   pingPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = pingPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func pingPayload(result pingResult, resultErr error) map[string]interface{} {
	payload := map[string]interface{}{
		"runner":             "core",
		"type":               "ping",
		"host":               result.Host,
		"method":             result.Method,
		"port":               result.Port,
		"address":            result.Address,
		"strategy":           result.Strategy,
		"reachable":          result.Reachable,
		"latency_ms":         result.Latency.Milliseconds(),
		"duration_ms":        result.Duration.Milliseconds(),
		"icmp_supported":     result.ICMPSupported,
		"requires_privilege": result.RequiresPrivilege,
		"fallback_strategy":  result.FallbackStrategy,
		"unsupported":        result.Unsupported,
		"ok":                 result.Health == "up",
		"collected_at":       result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage":      result.FailureStage,
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
