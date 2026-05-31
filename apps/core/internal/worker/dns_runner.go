package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"slices"
	"strings"
	"time"
)

type dnsResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
	LookupCNAME(context.Context, string) (string, error)
	LookupTXT(context.Context, string) ([]string, error)
	LookupMX(context.Context, string) ([]*net.MX, error)
	LookupNS(context.Context, string) ([]*net.NS, error)
}

type dnsConfig struct {
	Host           string   `json:"host"`
	RecordType     string   `json:"record_type"`
	ExpectedValues []string `json:"expected_values"`
	Resolver       string   `json:"resolver"`
}

type dnsResult struct {
	Health         string
	FinishedAt     time.Time
	Duration       time.Duration
	Host           string
	RecordType     string
	Resolver       string
	Answers        []string
	ExpectedValues []string
	MissingValues  []string
	Error          error
	FailureStage   string
}

func (a *App) runDNSCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) dnsResult {
	startedAt := time.Now()
	result := dnsResult{
		Health:     "down",
		FinishedAt: startedAt.UTC(),
	}

	runnerConfig, err := parseDNSConfig(monitorConfig.ConfigJSON)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	result.Host = runnerConfig.Host
	result.RecordType = runnerConfig.RecordType
	result.Resolver = runnerConfig.Resolver
	result.ExpectedValues = normalizeDNSValues(runnerConfig.ExpectedValues, runnerConfig.RecordType)

	timeout := time.Duration(monitorConfig.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	answers, err := a.lookupDNSAnswers(checkCtx, runnerConfig)
	result.FinishedAt = time.Now().UTC()
	result.Duration = time.Since(startedAt)
	if err != nil {
		result.Error = err
		result.FailureStage = dnsFailureStage(err)
		return result
	}
	result.Answers = normalizeDNSValues(answers, runnerConfig.RecordType)
	slices.Sort(result.Answers)

	result.MissingValues = missingDNSValues(result.Answers, result.ExpectedValues)
	if len(result.MissingValues) > 0 {
		result.Error = fmt.Errorf("expected DNS values missing: %s", strings.Join(result.MissingValues, ", "))
		result.FailureStage = "expected_values"
		return result
	}

	result.Health = "up"
	return result
}

func parseDNSConfig(raw string) (dnsConfig, error) {
	var cfg dnsConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("parse config json: %w", err)
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Host == "" {
		return cfg, fmt.Errorf("host is required")
	}
	cfg.RecordType = strings.ToUpper(strings.TrimSpace(cfg.RecordType))
	if cfg.RecordType == "" {
		cfg.RecordType = "A"
	}
	switch cfg.RecordType {
	case "A", "AAAA", "CNAME", "TXT", "MX", "NS":
	default:
		return cfg, fmt.Errorf("record_type must be one of A, AAAA, CNAME, TXT, MX, NS")
	}
	cfg.Resolver = strings.TrimSpace(cfg.Resolver)
	if cfg.Resolver == "" {
		cfg.Resolver = "system"
	}
	return cfg, nil
}

func (a *App) lookupDNSAnswers(ctx context.Context, cfg dnsConfig) ([]string, error) {
	switch cfg.RecordType {
	case "A", "AAAA":
		records, err := a.dnsResolver.LookupIPAddr(ctx, cfg.Host)
		if err != nil {
			return nil, err
		}
		answers := []string{}
		for _, record := range records {
			if cfg.RecordType == "A" && record.IP.To4() != nil {
				answers = append(answers, record.IP.String())
			}
			if cfg.RecordType == "AAAA" && record.IP.To4() == nil && record.IP.To16() != nil {
				answers = append(answers, record.IP.String())
			}
		}
		if len(answers) == 0 {
			return nil, fmt.Errorf("no %s records returned", cfg.RecordType)
		}
		return answers, nil
	case "CNAME":
		answer, err := a.dnsResolver.LookupCNAME(ctx, cfg.Host)
		if err != nil {
			return nil, err
		}
		return []string{answer}, nil
	case "TXT":
		return a.dnsResolver.LookupTXT(ctx, cfg.Host)
	case "MX":
		records, err := a.dnsResolver.LookupMX(ctx, cfg.Host)
		if err != nil {
			return nil, err
		}
		answers := make([]string, 0, len(records))
		for _, record := range records {
			answers = append(answers, record.Host)
		}
		return answers, nil
	case "NS":
		records, err := a.dnsResolver.LookupNS(ctx, cfg.Host)
		if err != nil {
			return nil, err
		}
		answers := make([]string, 0, len(records))
		for _, record := range records {
			answers = append(answers, record.Host)
		}
		return answers, nil
	default:
		return nil, fmt.Errorf("unsupported DNS record type %s", cfg.RecordType)
	}
}

func normalizeDNSValues(values []string, recordType string) []string {
	normalized := []string{}
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch recordType {
		case "CNAME", "MX", "NS":
			value = strings.TrimSuffix(strings.ToLower(value), ".")
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	slices.Sort(normalized)
	return normalized
}

func missingDNSValues(answers []string, expected []string) []string {
	if len(expected) == 0 {
		return nil
	}
	answerSet := map[string]struct{}{}
	for _, answer := range answers {
		answerSet[answer] = struct{}{}
	}
	missing := []string{}
	for _, expectedValue := range expected {
		if _, exists := answerSet[expectedValue]; !exists {
			missing = append(missing, expectedValue)
		}
	}
	return missing
}

func dnsFailureStage(err error) string {
	var dnsErr *net.DNSError
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	if errors.As(err, &dnsErr) {
		return "lookup"
	}
	return "lookup"
}

func (a *App) storeDNSReport(monitorID string, result dnsResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   dnsPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = dnsPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func dnsPayload(result dnsResult, resultErr error) map[string]any {
	payload := map[string]any{
		"runner":          "core",
		"type":            "dns",
		"host":            result.Host,
		"record_type":     result.RecordType,
		"resolver":        result.Resolver,
		"answers":         result.Answers,
		"expected_values": result.ExpectedValues,
		"missing_values":  result.MissingValues,
		"duration_ms":     result.Duration.Milliseconds(),
		"ok":              result.Health == "up",
		"collected_at":    result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage":   result.FailureStage,
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
