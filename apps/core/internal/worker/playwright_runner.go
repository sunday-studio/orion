package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultPlaywrightBrowser       = "chromium"
	defaultPlaywrightArtifactLimit = 64 * 1024
	playwrightRunnerEnv            = "ORION_PLAYWRIGHT_RUNNER"
	maxPlaywrightSteps             = 30
	maxPlaywrightArtifactBytes     = 256 * 1024
	maxPlaywrightSelectorLength    = 2048
	maxPlaywrightValueLength       = 4096
)

type playwrightRunFunc func(context.Context, playwrightTransactionRequest) (playwrightTransactionRunResult, error)

type playwrightTransactionConfig struct {
	URL                 string                 `json:"url"`
	StartURL            string                 `json:"start_url"`
	Browser             string                 `json:"browser"`
	Headless            *bool                  `json:"headless"`
	Viewport            playwrightViewport     `json:"viewport"`
	UserAgent           string                 `json:"user_agent"`
	ScreenshotOnFailure bool                   `json:"screenshot_on_failure"`
	Screenshot          string                 `json:"screenshot"`
	ArtifactLimitBytes  int                    `json:"artifact_limit_bytes"`
	AllowedHosts        []string               `json:"allowed_hosts"`
	Variables           map[string]string      `json:"variables"`
	Steps               []playwrightStepConfig `json:"steps"`
}

type playwrightViewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type playwrightStepConfig struct {
	Name      string `json:"name"`
	Action    string `json:"action"`
	URL       string `json:"url"`
	Selector  string `json:"selector"`
	Value     string `json:"value"`
	ValueRef  string `json:"value_ref"`
	Text      string `json:"text"`
	Contains  string `json:"contains"`
	TimeoutMS int    `json:"timeout_ms"`
}

type playwrightSecrets struct {
	Variables map[string]string `json:"variables"`
	Values    map[string]string `json:"values"`
	Headers   map[string]string `json:"headers"`
}

type playwrightTransactionRequest struct {
	Config  playwrightTransactionConfig `json:"config"`
	Secrets playwrightSecrets           `json:"secrets"`
}

type playwrightTransactionRunResult struct {
	Ok            bool                   `json:"ok"`
	Browser       string                 `json:"browser"`
	Headless      bool                   `json:"headless"`
	Viewport      playwrightViewport     `json:"viewport"`
	StepResults   []playwrightStepResult `json:"steps"`
	Artifacts     []playwrightArtifact   `json:"artifacts"`
	ArtifactBytes int                    `json:"artifact_bytes"`
	FailureStage  string                 `json:"failure_stage"`
	FailureStep   string                 `json:"failure_step"`
	FailureIndex  int                    `json:"failure_index"`
	Error         string                 `json:"error"`
}

type playwrightStepResult struct {
	Index        int    `json:"index"`
	Name         string `json:"name"`
	Action       string `json:"action"`
	Ok           bool   `json:"ok"`
	DurationMS   int64  `json:"duration_ms"`
	FailureStage string `json:"failure_stage,omitempty"`
	Error        string `json:"error,omitempty"`
}

type playwrightArtifact struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	MimeType   string `json:"mime_type"`
	DataBase64 string `json:"data_base64,omitempty"`
	Truncated  bool   `json:"truncated"`
	Bytes      int    `json:"bytes"`
}

type playwrightResult struct {
	Health              string
	FinishedAt          time.Time
	Duration            time.Duration
	TargetURL           string
	Browser             string
	Headless            bool
	Viewport            playwrightViewport
	StepCount           int
	CompletedSteps      int
	Steps               []playwrightStepResult
	Artifacts           []playwrightArtifact
	ArtifactBytes       int
	ArtifactLimitBytes  int
	ScreenshotOnFailure bool
	RedactedVariables   []string
	RedactedHeaders     []string
	FailureStep         string
	FailureIndex        int
	Error               error
	FailureStage        string
}

func (a *App) runPlaywrightCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) playwrightResult {
	startedAt := time.Now()
	result := playwrightResult{
		Health:       "down",
		FinishedAt:   startedAt.UTC(),
		FailureIndex: -1,
	}

	runnerConfig, secrets, err := parsePlaywrightConfig(monitorConfig.ConfigJSON, monitorConfig.SecretRefJSON)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	result.Browser = runnerConfig.Browser
	result.TargetURL = runnerConfig.URL
	result.Headless = playwrightHeadless(runnerConfig)
	result.Viewport = runnerConfig.Viewport
	result.StepCount = len(runnerConfig.Steps)
	result.ArtifactLimitBytes = runnerConfig.ArtifactLimitBytes
	result.ScreenshotOnFailure = runnerConfig.ScreenshotOnFailure
	result.RedactedVariables = sortedStringKeys(playwrightSecretValues(secrets))
	result.RedactedHeaders = sortedHeaderKeys(secrets.Headers)

	timeout := time.Duration(monitorConfig.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	runResult, err := a.playwrightRun(checkCtx, playwrightTransactionRequest{Config: runnerConfig, Secrets: secrets})
	redactPlaywrightRunResult(&runResult, secrets)
	result.FinishedAt = time.Now().UTC()
	result.Duration = time.Since(startedAt)
	result.Steps = runResult.StepResults
	result.Artifacts = limitPlaywrightArtifacts(runResult.Artifacts, runnerConfig.ArtifactLimitBytes)
	result.ArtifactBytes = playwrightArtifactBytes(result.Artifacts)
	result.FailureStep = runResult.FailureStep
	result.FailureIndex = runResult.FailureIndex
	result.FailureStage = runResult.FailureStage
	result.CompletedSteps = countPlaywrightCompletedSteps(result.Steps)
	if err != nil {
		result.Error = errors.New(redactPlaywrightText(err.Error(), secrets))
		if result.FailureStage == "" {
			result.FailureStage = playwrightRuntimeFailureStage(err)
		}
		return result
	}
	if runResult.Error != "" {
		result.Error = errors.New(runResult.Error)
		if result.FailureStage == "" {
			result.FailureStage = "transaction"
		}
		return result
	}
	if !runResult.Ok {
		result.Error = fmt.Errorf("Playwright transaction failed")
		if result.FailureStage == "" {
			result.FailureStage = "transaction"
		}
		return result
	}
	result.Health = "up"
	return result
}

func parsePlaywrightConfig(rawConfig string, rawSecrets string) (playwrightTransactionConfig, playwrightSecrets, error) {
	var cfg playwrightTransactionConfig
	if err := json.Unmarshal([]byte(rawConfig), &cfg); err != nil {
		return cfg, playwrightSecrets{}, fmt.Errorf("parse config json: %w", err)
	}
	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.StartURL = strings.TrimSpace(cfg.StartURL)
	if cfg.URL == "" {
		cfg.URL = cfg.StartURL
	}
	if cfg.URL != "" {
		if err := validatePlaywrightURL(cfg.URL); err != nil {
			return cfg, playwrightSecrets{}, err
		}
	}
	cfg.Browser = strings.ToLower(strings.TrimSpace(cfg.Browser))
	if cfg.Browser == "" {
		cfg.Browser = defaultPlaywrightBrowser
	}
	switch cfg.Browser {
	case "chromium", "firefox", "webkit":
	default:
		return cfg, playwrightSecrets{}, fmt.Errorf("browser must be chromium, firefox, or webkit")
	}
	if cfg.Viewport.Width == 0 {
		cfg.Viewport.Width = 1280
	}
	if cfg.Viewport.Height == 0 {
		cfg.Viewport.Height = 720
	}
	if cfg.Viewport.Width < 320 || cfg.Viewport.Width > 3840 || cfg.Viewport.Height < 240 || cfg.Viewport.Height > 2160 {
		return cfg, playwrightSecrets{}, fmt.Errorf("viewport must be between 320x240 and 3840x2160")
	}
	if cfg.ArtifactLimitBytes == 0 {
		cfg.ArtifactLimitBytes = defaultPlaywrightArtifactLimit
	}
	if cfg.ArtifactLimitBytes < 0 || cfg.ArtifactLimitBytes > maxPlaywrightArtifactBytes {
		return cfg, playwrightSecrets{}, fmt.Errorf("artifact_limit_bytes must be between 0 and %d", maxPlaywrightArtifactBytes)
	}
	cfg.Screenshot = strings.ToLower(strings.TrimSpace(cfg.Screenshot))
	if cfg.Screenshot == "on_failure" {
		cfg.ScreenshotOnFailure = true
	}
	if len(cfg.Steps) == 0 && cfg.URL == "" {
		return cfg, playwrightSecrets{}, fmt.Errorf("url or steps are required")
	}
	effectiveMaxSteps := maxPlaywrightSteps
	if len(cfg.Steps) > effectiveMaxSteps {
		return cfg, playwrightSecrets{}, fmt.Errorf("steps must contain at most %d items", effectiveMaxSteps)
	}
	if cfg.URL != "" && len(cfg.Steps) == 0 {
		cfg.Steps = []playwrightStepConfig{{Name: "open", Action: "goto", URL: cfg.URL}}
	}
	for index := range cfg.Steps {
		step := &cfg.Steps[index]
		step.Name = strings.TrimSpace(step.Name)
		if step.Name == "" {
			step.Name = fmt.Sprintf("step-%d", index+1)
		}
		step.Action = strings.ToLower(strings.TrimSpace(step.Action))
		if step.Action == "" {
			step.Action = "goto"
		}
		switch step.Action {
		case "goto", "click", "fill", "select", "check", "wait_for_selector", "text_contains", "assert_text", "assert_url", "screenshot":
		default:
			return cfg, playwrightSecrets{}, fmt.Errorf("step %d %q action is unsupported", index, step.Name)
		}
		if step.TimeoutMS < 0 || step.TimeoutMS > 60000 {
			return cfg, playwrightSecrets{}, fmt.Errorf("step %d %q timeout_ms must be between 0 and 60000", index, step.Name)
		}
		if step.Action == "goto" && strings.TrimSpace(step.URL) == "" {
			return cfg, playwrightSecrets{}, fmt.Errorf("step %d %q url is required", index, step.Name)
		}
		if step.Action == "goto" && !strings.Contains(step.URL, "{{") {
			if err := validatePlaywrightURL(step.URL); err != nil {
				return cfg, playwrightSecrets{}, fmt.Errorf("step %d %q: %w", index, step.Name, err)
			}
		}
		if (step.Action == "click" || step.Action == "fill" || step.Action == "select" || step.Action == "check" || step.Action == "wait_for_selector" || step.Action == "text_contains" || step.Action == "assert_text") && strings.TrimSpace(step.Selector) == "" {
			return cfg, playwrightSecrets{}, fmt.Errorf("step %d %q selector is required", index, step.Name)
		}
		if len(step.Selector) > maxPlaywrightSelectorLength {
			return cfg, playwrightSecrets{}, fmt.Errorf("step %d %q selector exceeds %d bytes", index, step.Name, maxPlaywrightSelectorLength)
		}
		if len(step.Value) > maxPlaywrightValueLength || len(step.Text) > maxPlaywrightValueLength || len(step.Contains) > maxPlaywrightValueLength {
			return cfg, playwrightSecrets{}, fmt.Errorf("step %d %q value exceeds %d bytes", index, step.Name, maxPlaywrightValueLength)
		}
	}
	if cfg.Variables == nil {
		cfg.Variables = map[string]string{}
	}
	secrets := playwrightSecrets{}
	if strings.TrimSpace(rawSecrets) != "" && strings.TrimSpace(rawSecrets) != "{}" {
		if err := json.Unmarshal([]byte(rawSecrets), &secrets); err != nil {
			return cfg, secrets, fmt.Errorf("parse secret ref json: %w", err)
		}
	}
	if secrets.Variables == nil {
		secrets.Variables = map[string]string{}
	}
	if secrets.Values == nil {
		secrets.Values = map[string]string{}
	}
	if secrets.Headers == nil {
		secrets.Headers = map[string]string{}
	}
	return cfg, secrets, nil
}

func validatePlaywrightURL(rawURL string) error {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("url is invalid: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("url scheme must be http or https")
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("url host is required")
	}
	return nil
}

func playwrightHeadless(cfg playwrightTransactionConfig) bool {
	if cfg.Headless == nil {
		return true
	}
	return *cfg.Headless
}

func defaultPlaywrightRun(ctx context.Context, request playwrightTransactionRequest) (playwrightTransactionRunResult, error) {
	runnerPath := strings.TrimSpace(os.Getenv(playwrightRunnerEnv))
	if runnerPath == "" {
		return playwrightTransactionRunResult{FailureStage: "runtime_unavailable"}, fmt.Errorf("%s is not configured; set it to an executable Playwright runner on the Core worker host", playwrightRunnerEnv)
	}
	input, err := json.Marshal(request)
	if err != nil {
		return playwrightTransactionRunResult{FailureStage: "config"}, err
	}
	cmd := exec.CommandContext(ctx, runnerPath)
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return playwrightTransactionRunResult{FailureStage: "runtime", Error: string(bytes.TrimSpace(output))}, fmt.Errorf("run Playwright transaction: %w", err)
	}
	var result playwrightTransactionRunResult
	if err := json.Unmarshal(output, &result); err != nil {
		return playwrightTransactionRunResult{FailureStage: "runtime", Error: string(bytes.TrimSpace(output))}, fmt.Errorf("parse Playwright result: %w", err)
	}
	return result, nil
}

func limitPlaywrightArtifacts(artifacts []playwrightArtifact, limit int) []playwrightArtifact {
	if limit <= 0 {
		return nil
	}
	used := 0
	limited := []playwrightArtifact{}
	for _, artifact := range artifacts {
		if artifact.Bytes <= 0 {
			artifact.Bytes = len(artifact.DataBase64)
		}
		if used+artifact.Bytes > limit {
			remaining := limit - used
			if remaining <= 0 {
				break
			}
			if len(artifact.DataBase64) > remaining {
				artifact.DataBase64 = artifact.DataBase64[:remaining]
			}
			artifact.Bytes = remaining
			artifact.Truncated = true
		}
		used += artifact.Bytes
		limited = append(limited, artifact)
		if used >= limit {
			break
		}
	}
	return limited
}

func playwrightArtifactBytes(artifacts []playwrightArtifact) int {
	total := 0
	for _, artifact := range artifacts {
		total += artifact.Bytes
	}
	return total
}

func countPlaywrightCompletedSteps(steps []playwrightStepResult) int {
	count := 0
	for _, step := range steps {
		if step.Ok {
			count++
		}
	}
	return count
}

func playwrightRuntimeFailureStage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "runtime_unavailable"
}

func redactPlaywrightRunResult(result *playwrightTransactionRunResult, secrets playwrightSecrets) {
	result.Error = redactPlaywrightText(result.Error, secrets)
	for index := range result.StepResults {
		result.StepResults[index].Error = redactPlaywrightText(result.StepResults[index].Error, secrets)
	}
}

func redactPlaywrightText(value string, secrets playwrightSecrets) string {
	for _, secretValue := range playwrightSecretValues(secrets) {
		if secretValue == "" {
			continue
		}
		value = strings.ReplaceAll(value, secretValue, "[redacted]")
	}
	for _, secretValue := range secrets.Headers {
		if secretValue == "" {
			continue
		}
		value = strings.ReplaceAll(value, secretValue, "[redacted]")
	}
	return value
}

func playwrightSecretValues(secrets playwrightSecrets) map[string]string {
	values := map[string]string{}
	for key, value := range secrets.Values {
		values[key] = value
	}
	for key, value := range secrets.Variables {
		values[key] = value
	}
	return values
}

func (a *App) storePlaywrightReport(monitorID string, result playwrightResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   playwrightPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = playwrightPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func playwrightPayload(result playwrightResult, resultErr error) map[string]interface{} {
	payload := map[string]interface{}{
		"runner":                "core",
		"type":                  "playwright_transaction",
		"target_url":            result.TargetURL,
		"browser":               result.Browser,
		"headless":              result.Headless,
		"viewport":              result.Viewport,
		"step_count":            result.StepCount,
		"completed_steps":       result.CompletedSteps,
		"steps":                 result.Steps,
		"artifacts":             result.Artifacts,
		"artifact_bytes":        result.ArtifactBytes,
		"artifact_limit_bytes":  result.ArtifactLimitBytes,
		"screenshot_on_failure": result.ScreenshotOnFailure,
		"redacted_variables":    result.RedactedVariables,
		"redacted_values":       result.RedactedVariables,
		"redacted_headers":      result.RedactedHeaders,
		"duration_ms":           result.Duration.Milliseconds(),
		"ok":                    result.Health == "up",
		"collected_at":          result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage":         result.FailureStage,
		"failure_step":          result.FailureStep,
		"failure_index":         result.FailureIndex,
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
