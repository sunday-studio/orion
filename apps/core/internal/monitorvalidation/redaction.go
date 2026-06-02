package monitorvalidation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func RedactConfigJSON(configJSON string) map[string]interface{} {
	var value map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &value); err != nil {
		return map[string]interface{}{}
	}
	return redactValue(value).(map[string]interface{})
}

func RedactSecretRefJSON(secretRefJSON string) map[string]interface{} {
	var value map[string]interface{}
	if err := json.Unmarshal([]byte(secretRefJSON), &value); err != nil {
		return map[string]interface{}{}
	}
	return redactSecretRefValue(value).(map[string]interface{})
}

func SanitizeURL(rawURL string) string {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return strings.TrimSpace(rawURL)
	}
	parsedURL.User = nil
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	return parsedURL.String()
}

func apiRequestMethodAllowed(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func hasNonEmptyString(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func playwrightActionRequiresSelector(action string) bool {
	switch action {
	case "click", "fill", "select", "check", "wait_for_selector", "text_contains", "assert_text":
		return true
	default:
		return false
	}
}

func variableNameValid(name string) bool {
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

func validateExpectedStatuses(expectedStatus int, expectedStatuses []int) error {
	if expectedStatus != 0 && (expectedStatus < 100 || expectedStatus > 599) {
		return fmt.Errorf("%w: expected_status must be between 100 and 599", ErrValidation)
	}
	for _, status := range expectedStatuses {
		if status < 100 || status > 599 {
			return fmt.Errorf("%w: expected_statuses must contain values between 100 and 599", ErrValidation)
		}
	}
	return nil
}

func normalizeMailProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "smtp", "smtps":
		return "smtp"
	case "imap", "imaps":
		return "imap"
	case "pop", "pop3", "pop3s":
		return "pop"
	default:
		return strings.ToLower(strings.TrimSpace(protocol))
	}
}

func redactValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			if isSensitiveConfigKey(key) {
				redacted[key] = "[redacted]"
				continue
			}
			if isURLConfigKey(key) {
				if rawURL, ok := nested.(string); ok {
					redacted[key] = SanitizeURL(rawURL)
					continue
				}
			}
			redacted[key] = redactValue(nested)
		}
		return redacted
	case []interface{}:
		redacted := make([]interface{}, 0, len(typed))
		for _, nested := range typed {
			redacted = append(redacted, redactValue(nested))
		}
		return redacted
	default:
		return typed
	}
}

func isSensitiveConfigKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, token := range []string{"secret", "token", "password", "api_key", "apikey", "authorization", "auth_header", "private_key"} {
		if strings.Contains(key, token) {
			return true
		}
	}
	return false
}

func isURLConfigKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "url", "start_url", "rdap_url", "target_url":
		return true
	default:
		return false
	}
}

func redactSecretRefValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			redacted[key] = redactSecretRefValue(nested)
		}
		return redacted
	case []interface{}:
		redacted := make([]interface{}, 0, len(typed))
		for _, nested := range typed {
			redacted = append(redacted, redactSecretRefValue(nested))
		}
		return redacted
	default:
		return "[redacted]"
	}
}
