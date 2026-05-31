package service

import (
	"encoding/json"
	"regexp"
	"strings"
)

const monitorReportRedactedValue = "[redacted]"

type monitorReportSensitivePattern struct {
	pattern     *regexp.Regexp
	replacement string
}

var monitorReportSensitivePatterns = []monitorReportSensitivePattern{
	{
		pattern:     regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._~+/-]+`),
		replacement: "${1}" + monitorReportRedactedValue,
	},
	{
		pattern:     regexp.MustCompile(`(?i)(token|password|secret|api[_-]?key|authorization|credential|session|cookie)(["']?\s*[:=]\s*["']?)[^"',\s}&]+`),
		replacement: "${1}${2}" + monitorReportRedactedValue,
	},
	{
		pattern:     regexp.MustCompile(`(?i)(https?://[^/\s:@]+:)[^@\s/]+@`),
		replacement: "${1}" + monitorReportRedactedValue + "@",
	},
}

// SafeMonitorReportPayload returns a frontend-safe JSON payload string.
func SafeMonitorReportPayload(payload string) string {
	var fields map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &fields); err != nil {
		return redactMonitorReportText(payload)
	}
	body, err := json.Marshal(RedactMonitorReportPayloadValue(fields))
	if err != nil {
		return redactMonitorReportText(payload)
	}
	return string(body)
}

func RedactMonitorReportPayloadValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, field := range typed {
			if monitorReportSensitiveKey(key) {
				result[key] = monitorReportRedactedValue
				continue
			}
			result[key] = RedactMonitorReportPayloadValue(field)
		}
		return result
	case map[string]string:
		result := make(map[string]string, len(typed))
		for key, field := range typed {
			if monitorReportSensitiveKey(key) {
				result[key] = monitorReportRedactedValue
				continue
			}
			result[key] = redactMonitorReportText(field)
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			result = append(result, RedactMonitorReportPayloadValue(item))
		}
		return result
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, redactMonitorReportText(item))
		}
		return result
	case string:
		return redactMonitorReportText(typed)
	default:
		return typed
	}
}

func monitorReportSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.NewReplacer("-", "_", " ", "_").Replace(normalized)
	for _, marker := range []string{
		"token",
		"password",
		"secret",
		"api_key",
		"apikey",
		"authorization",
		"credential",
		"session",
		"cookie",
		"private_key",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func redactMonitorReportText(value string) string {
	redacted := value
	for _, pattern := range monitorReportSensitivePatterns {
		redacted = pattern.pattern.ReplaceAllString(redacted, pattern.replacement)
	}
	return redacted
}
