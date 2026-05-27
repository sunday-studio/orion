package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"time"
)

const defaultDomainExpirationWarningDays = 30

type domainExpirationConfig struct {
	Domain      string `json:"domain"`
	RDAPURL     string `json:"rdap_url"`
	WarningDays int    `json:"warning_days"`
}

type rdapDomainResponse struct {
	Events []struct {
		EventAction string `json:"eventAction"`
		EventDate   string `json:"eventDate"`
	} `json:"events"`
}

type domainExpirationResult struct {
	Health           string
	FinishedAt       time.Time
	Duration         time.Duration
	Domain           string
	RDAPURL          string
	LookupStrategy   string
	FallbackStrategy string
	StatusCode       int
	ExpiresAt        *time.Time
	DaysRemaining    int
	WarningDays      int
	Error            error
	FailureStage     string
}

func (a *App) runDomainExpirationCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) domainExpirationResult {
	startedAt := time.Now()
	result := domainExpirationResult{
		Health:           "down",
		FinishedAt:       startedAt.UTC(),
		LookupStrategy:   "rdap",
		FallbackStrategy: "none",
	}

	runnerConfig, err := parseDomainExpirationConfig(monitorConfig.ConfigJSON)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	result.Domain = runnerConfig.Domain
	result.RDAPURL = runnerConfig.RDAPURL
	result.WarningDays = runnerConfig.WarningDays

	timeout := time.Duration(monitorConfig.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(checkCtx, http.MethodGet, runnerConfig.RDAPURL, nil)
	if err != nil {
		result.Error = err
		result.FailureStage = "rdap_request"
		return result
	}
	request.Header.Set("Accept", "application/rdap+json, application/json")
	request.Header.Set("User-Agent", "orion-core-monitor-worker")

	response, err := a.httpClient.Do(request)
	result.FinishedAt = time.Now().UTC()
	result.Duration = time.Since(startedAt)
	if err != nil {
		result.Error = err
		result.FailureStage = "rdap_transport"
		return result
	}
	defer response.Body.Close()
	result.StatusCode = response.StatusCode

	bodySample, _, err := captureHTTPBodySample(response.Body)
	if err != nil {
		result.Error = err
		result.FailureStage = "rdap_body"
		return result
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		result.Error = fmt.Errorf("RDAP returned HTTP status %d", response.StatusCode)
		result.FailureStage = "rdap_status"
		return result
	}

	expiresAt, err := parseRDAPExpiration(bodySample)
	if err != nil {
		result.Error = err
		result.FailureStage = "unavailable_data"
		return result
	}
	result.ExpiresAt = &expiresAt
	result.DaysRemaining = int(expiresAt.Sub(result.FinishedAt).Hours() / 24)

	if !expiresAt.After(result.FinishedAt) {
		result.Error = fmt.Errorf("domain expired at %s", expiresAt.Format(time.RFC3339))
		result.FailureStage = "expired"
		return result
	}
	if result.WarningDays > 0 && result.DaysRemaining <= result.WarningDays {
		result.Health = "degraded"
		result.Error = fmt.Errorf("domain expires within %d days", result.WarningDays)
		result.FailureStage = "expiry_threshold"
		return result
	}

	result.Health = "up"
	return result
}

func parseDomainExpirationConfig(raw string) (domainExpirationConfig, error) {
	var cfg domainExpirationConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("parse config json: %w", err)
	}
	cfg.Domain = strings.ToLower(strings.TrimSpace(cfg.Domain))
	if cfg.Domain == "" {
		return cfg, fmt.Errorf("domain is required")
	}
	if strings.ContainsAny(cfg.Domain, "/:@") {
		return cfg, fmt.Errorf("domain must be a hostname, not a URL")
	}
	if cfg.WarningDays < 0 {
		return cfg, fmt.Errorf("warning_days must be zero or greater")
	}
	if cfg.WarningDays == 0 {
		cfg.WarningDays = defaultDomainExpirationWarningDays
	}
	cfg.RDAPURL = strings.TrimSpace(cfg.RDAPURL)
	if cfg.RDAPURL == "" {
		cfg.RDAPURL = "https://rdap.org/domain/" + url.PathEscape(cfg.Domain)
	}
	parsedURL, err := url.ParseRequestURI(cfg.RDAPURL)
	if err != nil {
		return cfg, fmt.Errorf("rdap_url is invalid: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return cfg, fmt.Errorf("rdap_url scheme must be http or https")
	}
	if parsedURL.Host == "" {
		return cfg, fmt.Errorf("rdap_url host is required")
	}
	return cfg, nil
}

func parseRDAPExpiration(raw string) (time.Time, error) {
	var response rdapDomainResponse
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		return time.Time{}, fmt.Errorf("parse RDAP json: %w", err)
	}
	for _, event := range response.Events {
		action := strings.ToLower(strings.TrimSpace(event.EventAction))
		if action != "expiration" && action != "registration expiration" {
			continue
		}
		expiresAt, err := time.Parse(time.RFC3339, event.EventDate)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse expiration date: %w", err)
		}
		return expiresAt.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("RDAP response did not include an expiration event")
}

func (a *App) storeDomainExpirationReport(monitorID string, result domainExpirationResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   domainExpirationPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = domainExpirationPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func domainExpirationPayload(result domainExpirationResult, resultErr error) map[string]interface{} {
	payload := map[string]interface{}{
		"runner":            "core",
		"type":              "domain_expiration",
		"domain":            result.Domain,
		"rdap_url":          result.RDAPURL,
		"lookup_strategy":   result.LookupStrategy,
		"fallback_strategy": result.FallbackStrategy,
		"status_code":       result.StatusCode,
		"days_remaining":    result.DaysRemaining,
		"warning_days":      result.WarningDays,
		"duration_ms":       result.Duration.Milliseconds(),
		"ok":                result.Health == "up",
		"collected_at":      result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage":     result.FailureStage,
	}
	if result.ExpiresAt != nil {
		payload["expires_at"] = result.ExpiresAt.Format(time.RFC3339)
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
