package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"time"
)

type httpStatusConfig struct {
	URL               string   `json:"url"`
	Method            string   `json:"method"`
	ExpectedStatus    int      `json:"expected_status"`
	ExpectedStatuses  []int    `json:"expected_statuses"`
	RequiredContains  []string `json:"required_contains"`
	ForbiddenContains []string `json:"forbidden_contains"`
}

type httpStatusResult struct {
	Health           string
	FinishedAt       time.Time
	Duration         time.Duration
	TargetURL        string
	FinalURL         string
	FinalHost        string
	Method           string
	ExpectedStatus   int
	ExpectedStatuses []int
	StatusCode       int
	BodySample       string
	BodyTruncated    bool
	Error            error
	FailureStage     string
}

func (a *App) runHTTPStatusCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) httpStatusResult {
	startedAt := time.Now()
	finishedAt := startedAt.UTC()
	result := httpStatusResult{
		Health:     "down",
		FinishedAt: finishedAt,
		Method:     http.MethodGet,
	}

	runnerConfig, err := parseHTTPStatusConfig(monitorConfig.ConfigJSON)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	result.TargetURL = runnerConfig.URL
	result.Method = runnerConfig.Method
	result.ExpectedStatus = runnerConfig.ExpectedStatus
	result.ExpectedStatuses = runnerConfig.ExpectedStatuses
	if err := a.targetPolicy.ValidateURL(runnerConfig.URL, "url"); err != nil {
		result.TargetURL = service.SanitizeCoreMonitorURL(runnerConfig.URL)
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

	request, err := http.NewRequestWithContext(checkCtx, result.Method, result.TargetURL, nil)
	if err != nil {
		result.Error = err
		result.FailureStage = "http_request"
		return result
	}
	request.Header.Set("User-Agent", "orion-core-monitor-worker")

	response, err := a.httpClient.Do(request)
	finishedAt = time.Now().UTC()
	result.FinishedAt = finishedAt
	result.Duration = time.Since(startedAt)
	if err != nil {
		result.Error = err
		result.FailureStage = "http_request"
		return result
	}
	defer response.Body.Close()

	result.StatusCode = response.StatusCode
	if response.Request != nil && response.Request.URL != nil {
		result.FinalURL = service.SanitizeCoreMonitorURL(response.Request.URL.String())
		result.FinalHost = service.CoreMonitorURLHost(response.Request.URL.String())
	}
	bodySample, truncated, err := captureHTTPBodySample(response.Body)
	if err != nil {
		result.Error = err
		result.FailureStage = "http_body"
		return result
	}
	result.BodySample = bodySample
	result.BodyTruncated = truncated

	if !httpStatusMatches(response.StatusCode, result.ExpectedStatus, result.ExpectedStatuses) {
		result.Error = fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
		result.FailureStage = "http_response"
		return result
	}
	if missing := missingRequiredContains(result.BodySample, runnerConfig.RequiredContains); len(missing) > 0 {
		result.Error = fmt.Errorf("required response content not found: %s", strings.Join(missing, ", "))
		result.FailureStage = "body_required"
		return result
	}
	if found := foundForbiddenContains(result.BodySample, runnerConfig.ForbiddenContains); len(found) > 0 {
		result.Error = fmt.Errorf("forbidden response content found: %s", strings.Join(found, ", "))
		result.FailureStage = "body_forbidden"
		return result
	}

	result.Health = "up"
	return result
}

func captureHTTPBodySample(body io.Reader) (string, bool, error) {
	limited := io.LimitReader(body, maxHTTPBodyCaptureLen+1)
	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return "", false, err
	}
	truncated := len(bodyBytes) > maxHTTPBodyCaptureLen
	if truncated {
		bodyBytes = bodyBytes[:maxHTTPBodyCaptureLen]
	}
	_, _ = io.CopyN(io.Discard, body, maxHTTPResponseDrainLen)
	return string(bodyBytes), truncated, nil
}

func missingRequiredContains(body string, required []string) []string {
	missing := []string{}
	for _, keyword := range required {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		if !strings.Contains(body, keyword) {
			missing = append(missing, keyword)
		}
	}
	return missing
}

func foundForbiddenContains(body string, forbidden []string) []string {
	found := []string{}
	for _, keyword := range forbidden {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		if strings.Contains(body, keyword) {
			found = append(found, keyword)
		}
	}
	return found
}

func parseHTTPStatusConfig(raw string) (httpStatusConfig, error) {
	var cfg httpStatusConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("parse config json: %w", err)
	}

	cfg.URL = strings.TrimSpace(cfg.URL)
	if cfg.URL == "" {
		return cfg, fmt.Errorf("url is required")
	}
	parsedURL, err := url.ParseRequestURI(cfg.URL)
	if err != nil {
		return cfg, fmt.Errorf("url is invalid: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return cfg, fmt.Errorf("url scheme must be http or https")
	}
	if parsedURL.Host == "" {
		return cfg, fmt.Errorf("url host is required")
	}

	cfg.Method = strings.ToUpper(strings.TrimSpace(cfg.Method))
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	if cfg.Method != http.MethodGet && cfg.Method != http.MethodHead {
		return cfg, fmt.Errorf("method must be GET or HEAD")
	}
	if cfg.ExpectedStatus != 0 && (cfg.ExpectedStatus < 100 || cfg.ExpectedStatus > 599) {
		return cfg, fmt.Errorf("expected_status must be between 100 and 599")
	}
	seenStatuses := map[int]struct{}{}
	normalizedStatuses := []int{}
	for _, status := range cfg.ExpectedStatuses {
		if status < 100 || status > 599 {
			return cfg, fmt.Errorf("expected_statuses must contain values between 100 and 599")
		}
		if _, exists := seenStatuses[status]; exists {
			continue
		}
		seenStatuses[status] = struct{}{}
		normalizedStatuses = append(normalizedStatuses, status)
	}
	if cfg.ExpectedStatus != 0 {
		if _, exists := seenStatuses[cfg.ExpectedStatus]; !exists {
			normalizedStatuses = append([]int{cfg.ExpectedStatus}, normalizedStatuses...)
		}
	}
	cfg.ExpectedStatuses = normalizedStatuses

	return cfg, nil
}

func httpStatusMatches(statusCode int, expectedStatus int, expectedStatuses []int) bool {
	if len(expectedStatuses) == 0 && expectedStatus == 0 {
		return statusCode >= 200 && statusCode <= 299
	}
	for _, expected := range expectedStatuses {
		if statusCode == expected {
			return true
		}
	}
	if expectedStatus == 0 {
		return false
	}
	return statusCode == expectedStatus
}

func (a *App) storeHTTPStatusReport(monitorID string, result httpStatusResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   httpStatusPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = httpStatusPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func httpStatusPayload(result httpStatusResult, resultErr error) map[string]any {
	payload := map[string]any{
		"runner":        "core",
		"type":          "http",
		"method":        result.Method,
		"target_url":    service.SanitizeCoreMonitorURL(result.TargetURL),
		"status_code":   result.StatusCode,
		"duration_ms":   result.Duration.Milliseconds(),
		"ok":            result.Health == "up",
		"collected_at":  result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage": result.FailureStage,
	}
	if result.ExpectedStatus > 0 {
		payload["expected_status"] = result.ExpectedStatus
	}
	if result.FinalURL != "" {
		payload["final_url"] = result.FinalURL
	}
	if result.FinalHost != "" {
		payload["final_host"] = result.FinalHost
	}
	if len(result.ExpectedStatuses) > 0 {
		payload["expected_statuses"] = result.ExpectedStatuses
	}
	if result.ExpectedStatus == 0 && len(result.ExpectedStatuses) == 0 {
		payload["expected_status"] = "2xx"
	}
	if result.FailureStage == "body_required" || result.FailureStage == "body_forbidden" {
		payload["body_sample"] = result.BodySample
		payload["body_truncated"] = result.BodyTruncated
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
