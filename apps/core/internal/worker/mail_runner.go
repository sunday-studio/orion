package worker

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"orion/core/internal/db"
	"strconv"
	"strings"
	"time"
)

type mailConfig struct {
	Protocol             string   `json:"protocol"`
	Host                 string   `json:"host"`
	Port                 int      `json:"port"`
	TLSMode              string   `json:"tls_mode"`
	ServerName           string   `json:"server_name"`
	ExpectedBanner       string   `json:"expected_banner"`
	ExpectedCapabilities []string `json:"expected_capabilities"`
	AuthEnabled          bool     `json:"auth_enabled"`
}

type mailResult struct {
	Health               string
	FinishedAt           time.Time
	Duration             time.Duration
	Protocol             string
	Host                 string
	Port                 int
	Address              string
	TLSMode              string
	ServerName           string
	TLSNegotiated        bool
	Banner               string
	Capabilities         []string
	ExpectedBanner       string
	ExpectedCapabilities []string
	MissingCapabilities  []string
	AuthEnabled          bool
	AuthAttempted        bool
	Error                error
	FailureStage         string
}

func (a *App) runMailCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) mailResult {
	startedAt := time.Now()
	result := mailResult{
		Health:     "down",
		FinishedAt: startedAt.UTC(),
	}

	runnerConfig, err := parseMailConfig(monitorConfig.Kind, monitorConfig.ConfigJSON)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	result.Protocol = runnerConfig.Protocol
	result.Host = runnerConfig.Host
	result.Port = runnerConfig.Port
	result.Address = net.JoinHostPort(runnerConfig.Host, strconv.Itoa(runnerConfig.Port))
	result.TLSMode = runnerConfig.TLSMode
	result.ServerName = runnerConfig.ServerName
	result.ExpectedBanner = runnerConfig.ExpectedBanner
	result.ExpectedCapabilities = normalizeMailCapabilities(runnerConfig.ExpectedCapabilities)
	result.AuthEnabled = runnerConfig.AuthEnabled
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
	if err != nil {
		result.FinishedAt = time.Now().UTC()
		result.Duration = time.Since(startedAt)
		result.Error = err
		result.FailureStage = mailFailureStage(err, "connect")
		return result
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	if runnerConfig.TLSMode == "implicit" {
		tlsConn := tls.Client(conn, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: runnerConfig.ServerName})
		if err := tlsConn.HandshakeContext(checkCtx); err != nil {
			result.FinishedAt = time.Now().UTC()
			result.Duration = time.Since(startedAt)
			result.Error = err
			result.FailureStage = mailFailureStage(err, "tls")
			return result
		}
		conn = tlsConn
		result.TLSNegotiated = true
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	if err := a.runMailProtocol(checkCtx, reader, writer, conn, runnerConfig, &result); err != nil {
		result.FinishedAt = time.Now().UTC()
		result.Duration = time.Since(startedAt)
		result.Error = err
		if result.FailureStage == "" {
			result.FailureStage = "protocol"
		}
		return result
	}

	result.FinishedAt = time.Now().UTC()
	result.Duration = time.Since(startedAt)
	result.Health = "up"
	return result
}

func parseMailConfig(kind string, raw string) (mailConfig, error) {
	var cfg mailConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("parse config json: %w", err)
	}
	cfg.Protocol = normalizeMailProtocol(cfg.Protocol)
	if cfg.Protocol == "" {
		cfg.Protocol = normalizeMailProtocol(kind)
	}
	if cfg.Protocol == "" || cfg.Protocol == "mail" {
		return cfg, fmt.Errorf("protocol must be one of smtp, imap, pop")
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Host == "" {
		return cfg, fmt.Errorf("host is required")
	}
	cfg.TLSMode = strings.ToLower(strings.TrimSpace(cfg.TLSMode))
	if cfg.TLSMode == "" {
		cfg.TLSMode = "none"
	}
	switch cfg.TLSMode {
	case "none", "implicit", "starttls":
	default:
		return cfg, fmt.Errorf("tls_mode must be one of none, implicit, starttls")
	}
	if cfg.Port == 0 {
		cfg.Port = defaultMailPort(cfg.Protocol, cfg.TLSMode)
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return cfg, fmt.Errorf("port must be between 1 and 65535")
	}
	cfg.ServerName = strings.TrimSpace(cfg.ServerName)
	if cfg.ServerName == "" {
		cfg.ServerName = cfg.Host
	}
	cfg.ExpectedBanner = strings.TrimSpace(cfg.ExpectedBanner)
	if cfg.AuthEnabled {
		return cfg, fmt.Errorf("auth_enabled is not supported for mail monitors yet")
	}
	return cfg, nil
}

func normalizeMailProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "smtp", "smtps":
		return "smtp"
	case "imap", "imaps":
		return "imap"
	case "pop", "pop3", "pop3s":
		return "pop"
	default:
		return strings.ToLower(strings.TrimSpace(protocol))
	}
}

func defaultMailPort(protocol string, tlsMode string) int {
	switch protocol {
	case "smtp":
		if tlsMode == "implicit" {
			return 465
		}
		return 25
	case "imap":
		if tlsMode == "implicit" {
			return 993
		}
		return 143
	case "pop":
		if tlsMode == "implicit" {
			return 995
		}
		return 110
	default:
		return 0
	}
}
