package monitorvalidation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func ValidateSyntheticConfigWithPolicy(configJSON string, targetPolicy TargetPolicy) error {
	var cfg struct {
		Variables map[string]string `json:"variables"`
		Steps     []struct {
			Type             string `json:"type"`
			URL              string `json:"url"`
			Method           string `json:"method"`
			ExpectedStatus   int    `json:"expected_status"`
			ExpectedStatuses []int  `json:"expected_statuses"`
			Request          *struct {
				URL              string `json:"url"`
				Method           string `json:"method"`
				ExpectedStatus   int    `json:"expected_status"`
				ExpectedStatuses []int  `json:"expected_statuses"`
			} `json:"request"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrValidation, err)
	}
	if len(cfg.Steps) == 0 {
		return fmt.Errorf("%w: steps are required", ErrValidation)
	}
	if len(cfg.Steps) > maxSyntheticSteps {
		return fmt.Errorf("%w: steps must contain at most %d items", ErrValidation, maxSyntheticSteps)
	}
	if len(cfg.Variables) > maxSyntheticVariables {
		return fmt.Errorf("%w: variables must contain at most %d entries", ErrValidation, maxSyntheticVariables)
	}
	for key, value := range cfg.Variables {
		if !variableNameValid(key) {
			return fmt.Errorf("%w: variable %q has invalid name", ErrValidation, key)
		}
		if len(value) > maxSyntheticVariableLength {
			return fmt.Errorf("%w: variable %q exceeds %d bytes", ErrValidation, key, maxSyntheticVariableLength)
		}
	}
	for index, step := range cfg.Steps {
		stepType := strings.ToLower(strings.TrimSpace(step.Type))
		if stepType == "" || stepType == "http" {
			stepType = "api"
		}
		switch stepType {
		case "api":
			targetURL := strings.TrimSpace(step.URL)
			method := strings.ToUpper(strings.TrimSpace(step.Method))
			expectedStatus := step.ExpectedStatus
			expectedStatuses := step.ExpectedStatuses
			if step.Request != nil {
				if strings.TrimSpace(step.Request.URL) != "" {
					targetURL = strings.TrimSpace(step.Request.URL)
				}
				if strings.TrimSpace(step.Request.Method) != "" {
					method = strings.ToUpper(strings.TrimSpace(step.Request.Method))
				}
				if step.Request.ExpectedStatus != 0 {
					expectedStatus = step.Request.ExpectedStatus
				}
				if len(step.Request.ExpectedStatuses) > 0 {
					expectedStatuses = step.Request.ExpectedStatuses
				}
			}
			if strings.TrimSpace(targetURL) == "" {
				return fmt.Errorf("%w: step %d url is required", ErrValidation, index+1)
			}
			if !strings.Contains(targetURL, "{{") {
				if err := targetPolicy.ValidateURL(targetURL, fmt.Sprintf("step %d url", index+1)); err != nil {
					return err
				}
			}
			if method == "" {
				method = http.MethodGet
			}
			if !apiRequestMethodAllowed(method) {
				return fmt.Errorf("%w: step %d method must be GET, POST, PUT, PATCH, DELETE, HEAD, or OPTIONS", ErrValidation, index+1)
			}
			if err := validateExpectedStatuses(expectedStatus, expectedStatuses); err != nil {
				return err
			}
		case "browser":
			continue
		default:
			return fmt.Errorf("%w: step %d type must be api, http, or browser", ErrValidation, index+1)
		}
	}
	return nil
}

func ValidatePlaywrightConfigWithPolicy(configJSON string, targetPolicy TargetPolicy) error {
	var cfg struct {
		URL      string `json:"url"`
		StartURL string `json:"start_url"`
		Browser  string `json:"browser"`
		Steps    []struct {
			Name      string `json:"name"`
			Action    string `json:"action"`
			URL       string `json:"url"`
			Selector  string `json:"selector"`
			Value     string `json:"value"`
			Text      string `json:"text"`
			Contains  string `json:"contains"`
			TimeoutMS int    `json:"timeout_ms"`
		} `json:"steps"`
		ArtifactLimitBytes int `json:"artifact_limit_bytes"`
		Viewport           struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"viewport"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrValidation, err)
	}
	targetURL := strings.TrimSpace(cfg.URL)
	if targetURL == "" {
		targetURL = strings.TrimSpace(cfg.StartURL)
	}
	if targetURL == "" && len(cfg.Steps) == 0 {
		return fmt.Errorf("%w: url or steps are required", ErrValidation)
	}
	if targetURL != "" {
		if err := targetPolicy.ValidateURL(targetURL, "url"); err != nil {
			return err
		}
	}
	browser := strings.ToLower(strings.TrimSpace(cfg.Browser))
	if browser != "" {
		switch browser {
		case "chromium", "firefox", "webkit":
		default:
			return fmt.Errorf("%w: browser must be chromium, firefox, or webkit", ErrValidation)
		}
	}
	if cfg.Viewport.Width != 0 || cfg.Viewport.Height != 0 {
		if cfg.Viewport.Width < 320 || cfg.Viewport.Width > 3840 || cfg.Viewport.Height < 240 || cfg.Viewport.Height > 2160 {
			return fmt.Errorf("%w: viewport must be between 320x240 and 3840x2160", ErrValidation)
		}
	}
	if cfg.ArtifactLimitBytes < 0 || cfg.ArtifactLimitBytes > maxPlaywrightArtifactBytes {
		return fmt.Errorf("%w: artifact_limit_bytes must be between 0 and %d", ErrValidation, maxPlaywrightArtifactBytes)
	}
	if len(cfg.Steps) > maxPlaywrightSteps {
		return fmt.Errorf("%w: steps must contain at most %d items", ErrValidation, maxPlaywrightSteps)
	}
	for index, step := range cfg.Steps {
		action := strings.ToLower(strings.TrimSpace(step.Action))
		if action == "" {
			action = "goto"
		}
		switch action {
		case "goto", "click", "fill", "select", "check", "wait_for_selector", "text_contains", "assert_text", "assert_url", "screenshot":
		default:
			return fmt.Errorf("%w: step %d action is unsupported", ErrValidation, index+1)
		}
		if step.TimeoutMS < 0 || step.TimeoutMS > 60000 {
			return fmt.Errorf("%w: step %d timeout_ms must be between 0 and 60000", ErrValidation, index+1)
		}
		if action == "goto" {
			if strings.TrimSpace(step.URL) == "" {
				return fmt.Errorf("%w: step %d url is required", ErrValidation, index+1)
			}
			if !strings.Contains(step.URL, "{{") {
				if err := targetPolicy.ValidateURL(step.URL, fmt.Sprintf("step %d url", index+1)); err != nil {
					return err
				}
			}
		}
		if playwrightActionRequiresSelector(action) && strings.TrimSpace(step.Selector) == "" {
			return fmt.Errorf("%w: step %d selector is required", ErrValidation, index+1)
		}
		if len(step.Selector) > maxPlaywrightSelectorLen {
			return fmt.Errorf("%w: step %d selector exceeds %d bytes", ErrValidation, index+1, maxPlaywrightSelectorLen)
		}
		if len(step.Value) > maxPlaywrightValueLen || len(step.Text) > maxPlaywrightValueLen || len(step.Contains) > maxPlaywrightValueLen {
			return fmt.Errorf("%w: step %d value exceeds %d bytes", ErrValidation, index+1, maxPlaywrightValueLen)
		}
	}
	return nil
}
