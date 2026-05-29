package worker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

const defaultTLSWarningDays = 14

type tlsCheckFunc func(context.Context, tlsTarget) (tls.ConnectionState, error)

type tlsConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	ServerName  string `json:"server_name"`
	WarningDays int    `json:"warning_days"`
}

type tlsTarget struct {
	Host       string
	Port       int
	ServerName string
	Address    string
}

type tlsResult struct {
	Health        string
	FinishedAt    time.Time
	Duration      time.Duration
	Host          string
	Port          int
	ServerName    string
	Address       string
	Subject       string
	Issuer        string
	NotAfter      *time.Time
	DaysRemaining int
	WarningDays   int
	Expiring      bool
	ChainVerified bool
	Error         error
	FailureStage  string
}

func (a *App) runTLSCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) tlsResult {
	startedAt := time.Now()
	result := tlsResult{
		Health:      "down",
		FinishedAt:  startedAt.UTC(),
		WarningDays: a.tlsWarningDays,
	}

	runnerConfig, err := parseTLSConfig(monitorConfig.ConfigJSON, a.tlsWarningDays)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	result.Host = runnerConfig.Host
	result.Port = runnerConfig.Port
	result.ServerName = runnerConfig.ServerName
	result.WarningDays = runnerConfig.WarningDays
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

	state, err := a.tlsCheck(checkCtx, tlsTarget{
		Host:       result.Host,
		Port:       result.Port,
		ServerName: result.ServerName,
		Address:    result.Address,
	})
	result.FinishedAt = time.Now().UTC()
	result.Duration = time.Since(startedAt)
	if err != nil {
		result.Error = err
		result.FailureStage = tlsFailureStage(err)
		return result
	}
	result.ChainVerified = len(state.VerifiedChains) > 0
	if len(state.PeerCertificates) == 0 {
		result.Error = fmt.Errorf("peer certificate is missing")
		result.FailureStage = "certificate"
		return result
	}

	leaf := state.PeerCertificates[0]
	result.Subject = leaf.Subject.String()
	result.Issuer = leaf.Issuer.String()
	result.NotAfter = &leaf.NotAfter
	result.DaysRemaining = int(leaf.NotAfter.Sub(result.FinishedAt).Hours() / 24)

	if leaf.NotAfter.Before(result.FinishedAt) {
		result.Error = fmt.Errorf("certificate expired at %s", leaf.NotAfter.Format(time.RFC3339))
		result.FailureStage = "expired"
		return result
	}
	if result.WarningDays > 0 && result.DaysRemaining <= result.WarningDays {
		result.Health = "degraded"
		result.Expiring = true
		result.Error = fmt.Errorf("certificate expires within %d days", result.WarningDays)
		result.FailureStage = "expiry_threshold"
		return result
	}

	result.Health = "up"
	return result
}

func parseTLSConfig(raw string, defaultWarningDays int) (tlsConfig, error) {
	var cfg tlsConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("parse config json: %w", err)
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Host == "" {
		return cfg, fmt.Errorf("host is required")
	}
	if cfg.Port == 0 {
		cfg.Port = 443
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return cfg, fmt.Errorf("port must be between 1 and 65535")
	}
	cfg.ServerName = strings.TrimSpace(cfg.ServerName)
	if cfg.ServerName == "" {
		cfg.ServerName = cfg.Host
	}
	if cfg.WarningDays < 0 {
		return cfg, fmt.Errorf("warning_days must be zero or greater")
	}
	if cfg.WarningDays == 0 {
		cfg.WarningDays = defaultWarningDays
	}
	return cfg, nil
}

func defaultTLSCheck(ctx context.Context, target tlsTarget) (tls.ConnectionState, error) {
	dialer := &net.Dialer{}
	return tlsCheckWithDialContext(ctx, target, dialer.DialContext)
}

func defaultTLSCheckWithDialContext(dialContext dialContextFunc) tlsCheckFunc {
	return func(ctx context.Context, target tlsTarget) (tls.ConnectionState, error) {
		return tlsCheckWithDialContext(ctx, target, dialContext)
	}
}

func tlsCheckWithDialContext(ctx context.Context, target tlsTarget, dialContext dialContextFunc) (tls.ConnectionState, error) {
	rawConn, err := dialContext(ctx, "tcp", target.Address)
	if err != nil {
		return tls.ConnectionState{}, err
	}
	defer rawConn.Close()

	tlsConn := tls.Client(rawConn, &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: target.ServerName,
	})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return tls.ConnectionState{}, err
	}
	return tlsConn.ConnectionState(), nil
}

func tlsFailureStage(err error) string {
	var dnsErr *net.DNSError
	var netErr net.Error
	var hostnameErr x509.HostnameError
	var unknownAuthorityErr x509.UnknownAuthorityError
	var certInvalidErr x509.CertificateInvalidError
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	if errors.As(err, &dnsErr) {
		return "dns"
	}
	if errors.As(err, &hostnameErr) || errors.As(err, &unknownAuthorityErr) || errors.As(err, &certInvalidErr) {
		return "certificate"
	}
	return "connect"
}

func (a *App) storeTLSReport(monitorID string, result tlsResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   tlsPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = tlsPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func tlsPayload(result tlsResult, resultErr error) map[string]any {
	payload := map[string]any{
		"runner":             "core",
		"type":               "tls",
		"host":               result.Host,
		"port":               result.Port,
		"server_name":        result.ServerName,
		"address":            result.Address,
		"subject":            result.Subject,
		"issuer":             result.Issuer,
		"tls_days_remaining": result.DaysRemaining,
		"warning_days":       result.WarningDays,
		"expiring":           result.Expiring,
		"chain_verified":     result.ChainVerified,
		"duration_ms":        result.Duration.Milliseconds(),
		"ok":                 result.Health == "up",
		"collected_at":       result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage":      result.FailureStage,
	}
	if result.NotAfter != nil {
		payload["not_after"] = result.NotAfter.Format(time.RFC3339)
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
