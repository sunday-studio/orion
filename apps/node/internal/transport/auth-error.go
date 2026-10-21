package transport

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type AuthError struct {
	StatusCode int
	Body       string
	Code       string
	Message    string
}

func (e *AuthError) Error() string {
	if e.Code != "" && e.Message != "" {
		return fmt.Sprintf("core authentication failed with status %d: code=%s message=%s", e.StatusCode, e.Code, e.Message)
	}
	if e.Code != "" {
		return fmt.Sprintf("core authentication failed with status %d: code=%s", e.StatusCode, e.Code)
	}
	if e.Message != "" {
		return fmt.Sprintf("core authentication failed with status %d: %s", e.StatusCode, e.Message)
	}
	if e.Body == "" {
		return fmt.Sprintf("core authentication failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("core authentication failed with status %d: %s", e.StatusCode, e.Body)
}

func IsAuthError(err error) bool {
	var authErr *AuthError
	return errors.As(err, &authErr)
}

func NewAuthError(statusCode int, body []byte) *AuthError {
	code, message, safeBody := parseAuthErrorBody(body)
	return &AuthError{
		StatusCode: statusCode,
		Body:       safeBody,
		Code:       code,
		Message:    message,
	}
}

func parseAuthErrorBody(body []byte) (string, string, string) {
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return "", "", ""
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		code := firstStringField(payload, "code", "error_code", "error")
		message := firstStringField(payload, "message", "detail")
		return redactAuthText(code), redactAuthText(message), redactAuthText(raw)
	}

	return "", redactAuthText(raw), redactAuthText(raw)
}

func firstStringField(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		if text, ok := value.(string); ok {
			return text
		}
	}
	return ""
}

func redactAuthText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, pattern := range authRedactionPatterns {
		value = pattern.ReplaceAllString(value, "${1}[redacted]")
	}
	return value
}

var authRedactionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)("?(token|secret|password|api[_-]?key|authorization)"?\s*[:=]\s*"?)[^"\s,;{}]+`),
}
