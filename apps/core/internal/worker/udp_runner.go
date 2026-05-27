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

const maxUDPResponseBytes = 4096

type udpConfig struct {
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Payload          string `json:"payload"`
	ExpectedResponse string `json:"expected_response"`
}

type udpResult struct {
	Health           string
	FinishedAt       time.Time
	Duration         time.Duration
	Host             string
	Port             int
	Address          string
	PayloadBytes     int
	ExpectedResponse string
	Response         string
	ResponseBytes    int
	Error            error
	FailureStage     string
}

func (a *App) runUDPCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) udpResult {
	startedAt := time.Now()
	result := udpResult{
		Health:     "down",
		FinishedAt: startedAt.UTC(),
	}

	runnerConfig, err := parseUDPConfig(monitorConfig.ConfigJSON)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	result.Host = runnerConfig.Host
	result.Port = runnerConfig.Port
	result.Address = net.JoinHostPort(runnerConfig.Host, strconv.Itoa(runnerConfig.Port))
	result.PayloadBytes = len([]byte(runnerConfig.Payload))
	result.ExpectedResponse = runnerConfig.ExpectedResponse

	timeout := time.Duration(monitorConfig.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := a.udpDialContext(checkCtx, "udp", result.Address)
	if err != nil {
		result.FinishedAt = time.Now().UTC()
		result.Duration = time.Since(startedAt)
		result.Error = err
		result.FailureStage = udpFailureStage(err)
		return result
	}
	defer conn.Close()
	deadline := time.Now().Add(timeout)
	_ = conn.SetDeadline(deadline)

	if _, err = conn.Write([]byte(runnerConfig.Payload)); err != nil {
		result.FinishedAt = time.Now().UTC()
		result.Duration = time.Since(startedAt)
		result.Error = err
		result.FailureStage = "write"
		return result
	}

	buffer := make([]byte, maxUDPResponseBytes)
	n, err := conn.Read(buffer)
	result.FinishedAt = time.Now().UTC()
	result.Duration = time.Since(startedAt)
	if err != nil {
		result.Error = err
		result.FailureStage = udpFailureStage(err)
		return result
	}
	result.Response = string(buffer[:n])
	result.ResponseBytes = n
	if result.Response != runnerConfig.ExpectedResponse {
		result.Error = fmt.Errorf("unexpected UDP response")
		result.FailureStage = "response_mismatch"
		return result
	}

	result.Health = "up"
	return result
}

func parseUDPConfig(raw string) (udpConfig, error) {
	var cfg udpConfig
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
	if cfg.Payload == "" {
		return cfg, fmt.Errorf("payload is required")
	}
	if cfg.ExpectedResponse == "" {
		return cfg, fmt.Errorf("expected_response is required because UDP no-response success is unsupported")
	}
	return cfg, nil
}

func udpFailureStage(err error) string {
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

func (a *App) storeUDPReport(monitorID string, result udpResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   udpPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = udpPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func udpPayload(result udpResult, resultErr error) map[string]interface{} {
	payload := map[string]interface{}{
		"runner":            "core",
		"type":              "udp",
		"host":              result.Host,
		"port":              result.Port,
		"address":           result.Address,
		"payload_bytes":     result.PayloadBytes,
		"expected_response": result.ExpectedResponse,
		"response":          result.Response,
		"response_bytes":    result.ResponseBytes,
		"duration_ms":       result.Duration.Milliseconds(),
		"ok":                result.Health == "up",
		"collected_at":      result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage":     result.FailureStage,
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
