package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type apiRequestConfig struct {
	URL              string             `json:"url"`
	Method           string             `json:"method"`
	Headers          map[string]string  `json:"headers"`
	Body             string             `json:"body"`
	ExpectedStatus   int                `json:"expected_status"`
	ExpectedStatuses []int              `json:"expected_statuses"`
	JSONAssertions   []apiJSONAssertion `json:"json_assertions"`
}

type apiRequestSecrets struct {
	Headers map[string]string `json:"headers"`
}

type apiJSONAssertion struct {
	Path   string      `json:"path"`
	Equals interface{} `json:"equals"`
}

type apiRequestResult struct {
	Health            string
	FinishedAt        time.Time
	Duration          time.Duration
	TargetURL         string
	Method            string
	ExpectedStatus    int
	ExpectedStatuses  []int
	StatusCode        int
	RequestHeaders    []string
	RedactedHeaders   []string
	ResponseSample    string
	ResponseTruncated bool
	AssertionPath     string
	AssertionExpected interface{}
	AssertionActual   interface{}
	Error             error
	FailureStage      string
}

func (a *App) runAPIRequestCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) apiRequestResult {
	startedAt := time.Now()
	result := apiRequestResult{
		Health:     "down",
		FinishedAt: startedAt.UTC(),
		Method:     http.MethodGet,
	}

	runnerConfig, secrets, err := parseAPIRequestConfig(monitorConfig.ConfigJSON, monitorConfig.SecretRefJSON)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	result.TargetURL = runnerConfig.URL
	result.Method = runnerConfig.Method
	result.ExpectedStatus = runnerConfig.ExpectedStatus
	result.ExpectedStatuses = runnerConfig.ExpectedStatuses
	result.RequestHeaders = sortedHeaderKeys(runnerConfig.Headers)
	result.RedactedHeaders = sortedHeaderKeys(secrets.Headers)

	timeout := time.Duration(monitorConfig.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(checkCtx, runnerConfig.Method, runnerConfig.URL, bytes.NewBufferString(runnerConfig.Body))
	if err != nil {
		result.Error = err
		result.FailureStage = "http_request"
		return result
	}
	request.Header.Set("User-Agent", "orion-core-monitor-worker")
	for key, value := range runnerConfig.Headers {
		request.Header.Set(key, value)
	}
	for key, value := range secrets.Headers {
		request.Header.Set(key, value)
	}
	if runnerConfig.Body != "" && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := a.httpClient.Do(request)
	result.FinishedAt = time.Now().UTC()
	result.Duration = time.Since(startedAt)
	if err != nil {
		result.Error = err
		result.FailureStage = "transport"
		return result
	}
	defer response.Body.Close()

	result.StatusCode = response.StatusCode
	responseSample, truncated, err := captureHTTPBodySample(response.Body)
	if err != nil {
		result.Error = err
		result.FailureStage = "response_body"
		return result
	}
	result.ResponseSample = responseSample
	result.ResponseTruncated = truncated

	if !httpStatusMatches(response.StatusCode, result.ExpectedStatus, result.ExpectedStatuses) {
		result.Error = fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
		result.FailureStage = "status"
		return result
	}

	if len(runnerConfig.JSONAssertions) > 0 {
		if err := evaluateAPIJSONAssertions(responseSample, runnerConfig.JSONAssertions, &result); err != nil {
			result.Error = err
			result.FailureStage = "json_assertion"
			return result
		}
	}

	result.Health = "up"
	return result
}

func parseAPIRequestConfig(rawConfig string, rawSecrets string) (apiRequestConfig, apiRequestSecrets, error) {
	var cfg apiRequestConfig
	if err := json.Unmarshal([]byte(rawConfig), &cfg); err != nil {
		return cfg, apiRequestSecrets{}, fmt.Errorf("parse config json: %w", err)
	}
	cfg.URL = strings.TrimSpace(cfg.URL)
	if cfg.URL == "" {
		return cfg, apiRequestSecrets{}, fmt.Errorf("url is required")
	}
	parsedURL, err := url.ParseRequestURI(cfg.URL)
	if err != nil {
		return cfg, apiRequestSecrets{}, fmt.Errorf("url is invalid: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return cfg, apiRequestSecrets{}, fmt.Errorf("url scheme must be http or https")
	}
	if parsedURL.Host == "" {
		return cfg, apiRequestSecrets{}, fmt.Errorf("url host is required")
	}

	cfg.Method = strings.ToUpper(strings.TrimSpace(cfg.Method))
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	if !apiRequestMethodAllowed(cfg.Method) {
		return cfg, apiRequestSecrets{}, fmt.Errorf("method must be GET, POST, PUT, PATCH, DELETE, HEAD, or OPTIONS")
	}
	if cfg.ExpectedStatus != 0 && (cfg.ExpectedStatus < 100 || cfg.ExpectedStatus > 599) {
		return cfg, apiRequestSecrets{}, fmt.Errorf("expected_status must be between 100 and 599")
	}
	seenStatuses := map[int]struct{}{}
	normalizedStatuses := []int{}
	for _, status := range cfg.ExpectedStatuses {
		if status < 100 || status > 599 {
			return cfg, apiRequestSecrets{}, fmt.Errorf("expected_statuses must contain values between 100 and 599")
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
	cfg.Headers = normalizeHeaderMap(cfg.Headers)

	secrets := apiRequestSecrets{}
	if strings.TrimSpace(rawSecrets) != "" && strings.TrimSpace(rawSecrets) != "{}" {
		if err := json.Unmarshal([]byte(rawSecrets), &secrets); err != nil {
			return cfg, secrets, fmt.Errorf("parse secret ref json: %w", err)
		}
	}
	secrets.Headers = normalizeHeaderMap(secrets.Headers)
	return cfg, secrets, nil
}

func apiRequestMethodAllowed(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func normalizeHeaderMap(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	normalized := map[string]string{}
	for key, value := range headers {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		normalized[http.CanonicalHeaderKey(key)] = value
	}
	return normalized
}

func sortedHeaderKeys(headers map[string]string) []string {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func evaluateAPIJSONAssertions(body string, assertions []apiJSONAssertion, result *apiRequestResult) error {
	var decoded interface{}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		return fmt.Errorf("parse response json: %w", err)
	}
	for _, assertion := range assertions {
		actual, ok := valueAtJSONPath(decoded, assertion.Path)
		result.AssertionPath = assertion.Path
		result.AssertionExpected = assertion.Equals
		result.AssertionActual = actual
		if !ok {
			return fmt.Errorf("json path %s was not found", assertion.Path)
		}
		if !jsonValuesEqual(actual, assertion.Equals) {
			return fmt.Errorf("json path %s = %v, want %v", assertion.Path, actual, assertion.Equals)
		}
	}
	result.AssertionPath = ""
	result.AssertionExpected = nil
	result.AssertionActual = nil
	return nil
}

func valueAtJSONPath(root interface{}, path string) (interface{}, bool) {
	path = strings.TrimSpace(path)
	if path == "" || path == "$" {
		return root, true
	}
	path = strings.TrimPrefix(path, "$.")
	if strings.HasPrefix(path, "$") {
		return nil, false
	}
	current := root
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			return nil, false
		}
		switch value := current.(type) {
		case map[string]interface{}:
			next, ok := value[segment]
			if !ok {
				return nil, false
			}
			current = next
		case []interface{}:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(value) {
				return nil, false
			}
			current = value[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func jsonValuesEqual(actual interface{}, expected interface{}) bool {
	actual = normalizeJSONScalar(actual)
	expected = normalizeJSONScalar(expected)
	return reflect.DeepEqual(actual, expected)
}

func normalizeJSONScalar(value interface{}) interface{} {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		if number, err := typed.Float64(); err == nil {
			return number
		}
		return typed.String()
	default:
		return typed
	}
}

func (a *App) storeAPIRequestReport(monitorID string, result apiRequestResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   apiRequestPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = apiRequestPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func apiRequestPayload(result apiRequestResult, resultErr error) map[string]interface{} {
	payload := map[string]interface{}{
		"runner":             "core",
		"type":               "api_request",
		"method":             result.Method,
		"target_url":         result.TargetURL,
		"status_code":        result.StatusCode,
		"expected_status":    result.ExpectedStatus,
		"expected_statuses":  result.ExpectedStatuses,
		"request_headers":    result.RequestHeaders,
		"redacted_headers":   result.RedactedHeaders,
		"response_sample":    result.ResponseSample,
		"response_truncated": result.ResponseTruncated,
		"duration_ms":        result.Duration.Milliseconds(),
		"ok":                 result.Health == "up",
		"collected_at":       result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage":      result.FailureStage,
	}
	if result.AssertionPath != "" {
		payload["assertion_path"] = result.AssertionPath
		payload["assertion_expected"] = result.AssertionExpected
		payload["assertion_actual"] = result.AssertionActual
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
