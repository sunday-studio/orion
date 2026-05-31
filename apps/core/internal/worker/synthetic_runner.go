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
	"slices"
	"strings"
	"time"
)

const (
	maxSyntheticSteps          = 10
	maxSyntheticVariables      = 10
	maxSyntheticVariableLength = 1024
)

type syntheticConfig struct {
	Variables     map[string]string     `json:"variables"`
	StopOnFailure *bool                 `json:"stop_on_failure"`
	Steps         []syntheticStepConfig `json:"steps"`
}

type syntheticStepConfig struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Type             string                `json:"type"`
	Request          *syntheticAPIRequest  `json:"request"`
	URL              string                `json:"url"`
	Method           string                `json:"method"`
	Headers          map[string]string     `json:"headers"`
	Body             string                `json:"body"`
	ExpectedStatus   int                   `json:"expected_status"`
	ExpectedStatuses []int                 `json:"expected_statuses"`
	Assertions       []apiJSONAssertion    `json:"assertions"`
	JSONAssertions   []apiJSONAssertion    `json:"json_assertions"`
	Extract          []syntheticExtraction `json:"extract"`
}

type syntheticAPIRequest struct {
	URL              string            `json:"url"`
	Method           string            `json:"method"`
	Headers          map[string]string `json:"headers"`
	Body             string            `json:"body"`
	ExpectedStatus   int               `json:"expected_status"`
	ExpectedStatuses []int             `json:"expected_statuses"`
}

type syntheticExtraction struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type syntheticResult struct {
	Health         string
	FinishedAt     time.Time
	Duration       time.Duration
	Steps          []syntheticStepResult
	StepCount      int
	CompletedSteps int
	Variables      []string
	StopOnFailure  bool
	FailureStep    string
	FailureStepID  string
	FailureIndex   int
	Error          error
	FailureStage   string
}

type syntheticStepResult struct {
	ID                string   `json:"id"`
	Index             int      `json:"index"`
	Name              string   `json:"name"`
	Type              string   `json:"type"`
	Method            string   `json:"method,omitempty"`
	TargetURL         string   `json:"target_url,omitempty"`
	FinalURL          string   `json:"final_url,omitempty"`
	FinalHost         string   `json:"final_host,omitempty"`
	StatusCode        int      `json:"status_code,omitempty"`
	ExpectedStatus    int      `json:"expected_status,omitempty"`
	ExpectedStatuses  []int    `json:"expected_statuses,omitempty"`
	RequestHeaders    []string `json:"request_headers,omitempty"`
	ResponseSample    string   `json:"response_sample,omitempty"`
	ResponseTruncated bool     `json:"response_truncated,omitempty"`
	Extracted         []string `json:"extracted_variables,omitempty"`
	AssertionPath     string   `json:"assertion_path,omitempty"`
	AssertionExpected any      `json:"assertion_expected,omitempty"`
	AssertionActual   any      `json:"assertion_actual,omitempty"`
	DurationMS        int64    `json:"duration_ms"`
	Ok                bool     `json:"ok"`
	FailureStage      string   `json:"failure_stage,omitempty"`
	Error             string   `json:"error,omitempty"`
	startedAt         time.Time
}

func (a *App) runSyntheticCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) syntheticResult {
	startedAt := time.Now()
	result := syntheticResult{
		Health:       "down",
		FinishedAt:   startedAt.UTC(),
		FailureIndex: -1,
	}

	runnerConfig, err := parseSyntheticConfig(monitorConfig.ConfigJSON)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	variables := copySyntheticVariables(runnerConfig.Variables)
	result.StepCount = len(runnerConfig.Steps)
	result.StopOnFailure = syntheticStopOnFailure(runnerConfig)
	result.Variables = sortedStringKeys(variables)

	timeout := time.Duration(monitorConfig.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for index, step := range runnerConfig.Steps {
		stepResult := syntheticStepResult{
			Index:     index,
			ID:        step.ID,
			Name:      step.Name,
			Type:      step.Type,
			Method:    step.Method,
			startedAt: time.Now(),
		}
		err := a.runSyntheticStep(checkCtx, step, variables, &stepResult)
		stepResult.DurationMS = time.Since(stepResult.startedAt).Milliseconds()
		if err != nil {
			stepResult.Ok = false
			stepResult.Error = err.Error()
			if stepResult.FailureStage == "" {
				stepResult.FailureStage = "step"
			}
			result.Steps = append(result.Steps, stepResult)
			result.FailureStep = step.Name
			result.FailureStepID = step.ID
			result.FailureIndex = index
			result.FailureStage = stepResult.FailureStage
			result.Error = fmt.Errorf("synthetic step %d %q failed: %w", index, step.Name, err)
			if result.StopOnFailure {
				result.FinishedAt = time.Now().UTC()
				result.Duration = time.Since(startedAt)
				result.CompletedSteps = countSyntheticCompletedSteps(result.Steps)
				result.Variables = sortedStringKeys(variables)
				return result
			}
			continue
		}
		stepResult.Ok = true
		result.Steps = append(result.Steps, stepResult)
	}

	result.FinishedAt = time.Now().UTC()
	result.Duration = time.Since(startedAt)
	result.CompletedSteps = countSyntheticCompletedSteps(result.Steps)
	result.Variables = sortedStringKeys(variables)
	if result.Error == nil {
		result.Health = "up"
	}
	return result
}

func parseSyntheticConfig(raw string) (syntheticConfig, error) {
	var cfg syntheticConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("parse config json: %w", err)
	}
	if len(cfg.Steps) == 0 {
		return cfg, fmt.Errorf("steps are required")
	}
	if len(cfg.Steps) > maxSyntheticSteps {
		return cfg, fmt.Errorf("steps must contain at most %d items", maxSyntheticSteps)
	}
	if cfg.Variables == nil {
		cfg.Variables = map[string]string{}
	}
	if len(cfg.Variables) > maxSyntheticVariables {
		return cfg, fmt.Errorf("variables must contain at most %d entries", maxSyntheticVariables)
	}
	for key, value := range cfg.Variables {
		if !syntheticVariableNameValid(key) {
			return cfg, fmt.Errorf("variable %q has invalid name", key)
		}
		if len(value) > maxSyntheticVariableLength {
			return cfg, fmt.Errorf("variable %q exceeds %d bytes", key, maxSyntheticVariableLength)
		}
	}
	for index := range cfg.Steps {
		step := &cfg.Steps[index]
		step.ID = strings.TrimSpace(step.ID)
		step.Name = strings.TrimSpace(step.Name)
		if step.Name == "" && step.ID != "" {
			step.Name = step.ID
		}
		if step.ID == "" && step.Name != "" {
			step.ID = step.Name
		}
		if step.Name == "" {
			step.Name = fmt.Sprintf("step-%d", index+1)
		}
		if step.ID == "" {
			step.ID = step.Name
		}
		step.Type = strings.ToLower(strings.TrimSpace(step.Type))
		if step.Type == "" {
			step.Type = "api"
		}
		switch step.Type {
		case "api", "http":
			step.Type = "api"
			if err := normalizeSyntheticAPIStep(step); err != nil {
				return cfg, fmt.Errorf("step %d %q: %w", index, step.Name, err)
			}
		case "browser":
			continue
		default:
			return cfg, fmt.Errorf("step %d %q: type must be api, http, or browser", index, step.Name)
		}
	}
	return cfg, nil
}

func normalizeSyntheticAPIStep(step *syntheticStepConfig) error {
	if step.Request != nil {
		if step.Request.URL != "" {
			step.URL = step.Request.URL
		}
		if step.Request.Method != "" {
			step.Method = step.Request.Method
		}
		if step.Request.Headers != nil {
			step.Headers = step.Request.Headers
		}
		if step.Request.Body != "" {
			step.Body = step.Request.Body
		}
		if step.Request.ExpectedStatus != 0 {
			step.ExpectedStatus = step.Request.ExpectedStatus
		}
		if len(step.Request.ExpectedStatuses) > 0 {
			step.ExpectedStatuses = step.Request.ExpectedStatuses
		}
	}
	if len(step.Assertions) > 0 {
		step.JSONAssertions = append(step.JSONAssertions, step.Assertions...)
	}
	step.URL = strings.TrimSpace(step.URL)
	if step.URL == "" {
		return fmt.Errorf("url is required")
	}
	if !strings.Contains(step.URL, "{{") {
		parsedURL, err := url.ParseRequestURI(step.URL)
		if err != nil {
			return fmt.Errorf("url is invalid: %w", err)
		}
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return fmt.Errorf("url scheme must be http or https")
		}
		if parsedURL.Host == "" {
			return fmt.Errorf("url host is required")
		}
	}
	step.Method = strings.ToUpper(strings.TrimSpace(step.Method))
	if step.Method == "" {
		step.Method = http.MethodGet
	}
	if !apiRequestMethodAllowed(step.Method) {
		return fmt.Errorf("method must be GET, POST, PUT, PATCH, DELETE, HEAD, or OPTIONS")
	}
	if step.ExpectedStatus != 0 && (step.ExpectedStatus < 100 || step.ExpectedStatus > 599) {
		return fmt.Errorf("expected_status must be between 100 and 599")
	}
	seenStatuses := map[int]struct{}{}
	normalizedStatuses := []int{}
	for _, status := range step.ExpectedStatuses {
		if status < 100 || status > 599 {
			return fmt.Errorf("expected_statuses must contain values between 100 and 599")
		}
		if _, exists := seenStatuses[status]; exists {
			continue
		}
		seenStatuses[status] = struct{}{}
		normalizedStatuses = append(normalizedStatuses, status)
	}
	if step.ExpectedStatus != 0 {
		if _, exists := seenStatuses[step.ExpectedStatus]; !exists {
			normalizedStatuses = append([]int{step.ExpectedStatus}, normalizedStatuses...)
		}
	}
	step.ExpectedStatuses = normalizedStatuses
	step.Headers = normalizeHeaderMap(step.Headers)
	for index := range step.Extract {
		step.Extract[index].Name = strings.TrimSpace(step.Extract[index].Name)
		step.Extract[index].Path = strings.TrimSpace(step.Extract[index].Path)
		if step.Extract[index].Name == "" || step.Extract[index].Path == "" {
			return fmt.Errorf("extract entries require name and path")
		}
		if !syntheticVariableNameValid(step.Extract[index].Name) {
			return fmt.Errorf("extract variable %q has invalid name", step.Extract[index].Name)
		}
	}
	return nil
}

func (a *App) runSyntheticStep(ctx context.Context, step syntheticStepConfig, variables map[string]string, result *syntheticStepResult) error {
	if step.Type == "browser" {
		result.FailureStage = "unsupported_step"
		return fmt.Errorf("browser synthetic steps require the Playwright transaction runner")
	}
	targetURL, err := substituteSyntheticVariables(step.URL, variables)
	if err != nil {
		result.FailureStage = "variable_substitution"
		return err
	}
	body, err := substituteSyntheticVariables(step.Body, variables)
	if err != nil {
		result.FailureStage = "variable_substitution"
		return err
	}
	headers, err := substituteSyntheticHeaderVariables(step.Headers, variables)
	if err != nil {
		result.FailureStage = "variable_substitution"
		return err
	}
	result.TargetURL = service.SanitizeCoreMonitorURL(targetURL)
	if err := a.targetPolicy.ValidateURL(targetURL, "step url"); err != nil {
		result.FailureStage = "config"
		return err
	}
	result.RequestHeaders = sortedHeaderKeys(headers)

	request, err := http.NewRequestWithContext(ctx, step.Method, targetURL, bytes.NewBufferString(body))
	if err != nil {
		result.FailureStage = "http_request"
		return err
	}
	request.Header.Set("User-Agent", "orion-core-monitor-worker")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if body != "" && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := a.httpClient.Do(request)
	if err != nil {
		result.FailureStage = "transport"
		return err
	}
	defer response.Body.Close()
	result.StatusCode = response.StatusCode
	if response.Request != nil && response.Request.URL != nil {
		result.FinalURL = service.SanitizeCoreMonitorURL(response.Request.URL.String())
		result.FinalHost = service.CoreMonitorURLHost(response.Request.URL.String())
	}
	result.ExpectedStatus = step.ExpectedStatus
	result.ExpectedStatuses = step.ExpectedStatuses

	responseSample, truncated, err := captureHTTPBodySample(response.Body)
	if err != nil {
		result.FailureStage = "response_body"
		return err
	}
	result.ResponseSample = responseSample
	result.ResponseTruncated = truncated

	if !httpStatusMatches(response.StatusCode, step.ExpectedStatus, step.ExpectedStatuses) {
		result.FailureStage = "step_status"
		return fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	if len(step.JSONAssertions) > 0 {
		assertions, err := substituteSyntheticAssertionVariables(step.JSONAssertions, variables)
		if err != nil {
			result.FailureStage = "variable_substitution"
			return err
		}
		if err := evaluateSyntheticJSONAssertions(responseSample, assertions, result); err != nil {
			result.FailureStage = "json_assertion"
			return err
		}
	}
	if len(step.Extract) > 0 {
		extracted, err := extractSyntheticVariables(responseSample, step.Extract, variables)
		if err != nil {
			result.FailureStage = "extract"
			return err
		}
		result.Extracted = extracted
	}
	return nil
}

func evaluateSyntheticJSONAssertions(body string, assertions []apiJSONAssertion, result *syntheticStepResult) error {
	var decoded any
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

func extractSyntheticVariables(body string, extractions []syntheticExtraction, variables map[string]string) ([]string, error) {
	var decoded any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		return nil, fmt.Errorf("parse response json: %w", err)
	}
	extracted := []string{}
	for _, extraction := range extractions {
		if _, exists := variables[extraction.Name]; !exists && len(variables) >= maxSyntheticVariables {
			return extracted, fmt.Errorf("variable limit of %d would be exceeded", maxSyntheticVariables)
		}
		value, ok := valueAtJSONPath(decoded, extraction.Path)
		if !ok {
			return extracted, fmt.Errorf("json path %s was not found", extraction.Path)
		}
		extractedValue := fmt.Sprint(normalizeJSONScalar(value))
		if len(extractedValue) > maxSyntheticVariableLength {
			return extracted, fmt.Errorf("extracted variable %s exceeds %d bytes", extraction.Name, maxSyntheticVariableLength)
		}
		variables[extraction.Name] = extractedValue
		extracted = append(extracted, extraction.Name)
	}
	slices.Sort(extracted)
	return extracted, nil
}

func substituteSyntheticHeaderVariables(headers map[string]string, variables map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(headers))
	for key, value := range headers {
		substituted, err := substituteSyntheticVariables(value, variables)
		if err != nil {
			return nil, err
		}
		result[key] = substituted
	}
	return result, nil
}

func substituteSyntheticAssertionVariables(assertions []apiJSONAssertion, variables map[string]string) ([]apiJSONAssertion, error) {
	substituted := make([]apiJSONAssertion, 0, len(assertions))
	for _, assertion := range assertions {
		if expected, ok := assertion.Equals.(string); ok {
			value, err := substituteSyntheticVariables(expected, variables)
			if err != nil {
				return nil, err
			}
			assertion.Equals = value
		}
		substituted = append(substituted, assertion)
	}
	return substituted, nil
}

func substituteSyntheticVariables(value string, variables map[string]string) (string, error) {
	for {
		start := strings.Index(value, "{{")
		if start < 0 {
			return value, nil
		}
		end := strings.Index(value[start+2:], "}}")
		if end < 0 {
			return "", fmt.Errorf("unterminated variable template")
		}
		end += start + 2
		name := strings.TrimSpace(value[start+2 : end])
		if !syntheticVariableNameValid(name) {
			return "", fmt.Errorf("variable %q has invalid name", name)
		}
		replacement, ok := variables[name]
		if !ok {
			return "", fmt.Errorf("variable %s is not defined", name)
		}
		value = value[:start] + replacement + value[end+2:]
	}
}

func copySyntheticVariables(values map[string]string) map[string]string {
	copied := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		copied[key] = value
	}
	return copied
}

func syntheticStopOnFailure(cfg syntheticConfig) bool {
	if cfg.StopOnFailure == nil {
		return true
	}
	return *cfg.StopOnFailure
}

func countSyntheticCompletedSteps(steps []syntheticStepResult) int {
	count := 0
	for _, step := range steps {
		if step.Ok {
			count++
		}
	}
	return count
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func syntheticVariableNameValid(name string) bool {
	if name == "" {
		return false
	}
	for index, char := range name {
		if index == 0 {
			if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || char == '_' {
				continue
			}
			return false
		}
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			continue
		}
		return false
	}
	return true
}

func (a *App) storeSyntheticReport(monitorID string, result syntheticResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   syntheticPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = syntheticPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func syntheticPayload(result syntheticResult, resultErr error) map[string]any {
	payload := map[string]any{
		"runner":               "core",
		"type":                 "synthetic",
		"step_count":           result.StepCount,
		"completed_steps":      result.CompletedSteps,
		"completed_step_count": result.CompletedSteps,
		"steps":                result.Steps,
		"variables":            result.Variables,
		"stop_on_failure":      result.StopOnFailure,
		"duration_ms":          result.Duration.Milliseconds(),
		"ok":                   result.Health == "up",
		"collected_at":         result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage":        result.FailureStage,
		"failure_step":         result.FailureStep,
		"failure_step_id":      result.FailureStepID,
		"failure_index":        result.FailureIndex,
		"failure_step_index":   result.FailureIndex,
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
