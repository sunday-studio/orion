package worker

import (
	"encoding/json"
	"fmt"
	"orion/core/internal/service"
	"slices"
	"strings"
	"time"
)

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
