package service

import (
	"errors"
	"strings"
	"testing"

	"orion/core/internal/config"
)

func TestValidateCoreDomainExpirationMonitorConfigAcceptsWHOISServer(t *testing.T) {
	if err := validateCoreDomainExpirationMonitorConfig(`{"domain":"example.com","whois_server":"whois.example.test:43"}`); err != nil {
		t.Fatalf("validate domain expiration config: %v", err)
	}
}

func TestValidateCoreSyntheticMonitorConfigRejectsBrowserStep(t *testing.T) {
	raw := `{"steps":[{"type":"browser","name":"later-browser-step"}]}`
	if err := validateCoreSyntheticMonitorConfig(raw); err == nil {
		t.Fatal("validate synthetic browser step: got nil error, want validation error")
	}
}

func TestValidateCoreSyntheticMonitorConfigRejectsWorkerConstraints(t *testing.T) {
	tests := []string{
		`{"steps":[{"type":"api","url":"https://example.com","method":"TRACE"}]}`,
		`{"variables":{"1bad":"value"},"steps":[{"type":"api","url":"https://example.com"}]}`,
		`{"steps":[{"type":"api","url":"ftp://example.com"}]}`,
	}
	for _, raw := range tests {
		if err := validateCoreSyntheticMonitorConfig(raw); err == nil {
			t.Fatalf("validate synthetic config %s: got nil error, want validation error", raw)
		}
	}
}

func TestValidateCoreManagedMonitorConfigRejectsPlaywrightKind(t *testing.T) {
	err := validateCoreManagedMonitorConfig("playwright_transaction", `{"url":"https://example.com"}`, `{}`)
	if !errors.Is(err, ErrCoreManagedMonitorUnsupportedKind) {
		t.Fatalf("validate playwright_transaction error = %v, want unsupported kind", err)
	}
}

func TestValidateCoreDomainExpirationMonitorConfigRejectsUnsafeWHOISServer(t *testing.T) {
	tests := []string{
		`{"domain":"example.com","whois_server":"https://whois.example.test"}`,
		`{"domain":"example.com","whois_server":"whois.example.test:70000"}`,
		`{"domain":"example.com","whois_server":"user@whois.example.test"}`,
	}
	for _, raw := range tests {
		if err := validateCoreDomainExpirationMonitorConfig(raw); err == nil {
			t.Fatalf("validate domain expiration config %s: got nil error, want validation error", raw)
		}
	}
}

func TestCoreMonitorTargetPolicyRejectsBlockedLiteralTargets(t *testing.T) {
	policy := NewCoreMonitorTargetPolicy(nil)
	tests := []string{
		"http://169.254.169.254/latest/meta-data",
		"http://100.100.100.200/latest/meta-data",
		"http://127.0.0.1:8080/health",
		"http://localhost:8080/health",
		"http://10.0.0.5/health",
	}
	for _, rawURL := range tests {
		if err := policy.ValidateURL(rawURL, "url"); err == nil {
			t.Fatalf("ValidateURL(%q) got nil error, want blocked target", rawURL)
		}
	}
}

func TestCoreMonitorTargetPolicyAllowsPrivateTargetsWhenConfigured(t *testing.T) {
	policy := NewCoreMonitorTargetPolicy(&config.Config{CoreMonitorAllowPrivateTargets: true})
	if err := policy.ValidateURL("http://10.0.0.5/health", "url"); err != nil {
		t.Fatalf("ValidateURL private target with allowance: %v", err)
	}
	if err := policy.ValidateURL("http://127.0.0.1:8080/health", "url"); err != nil {
		t.Fatalf("ValidateURL loopback target with allowance: %v", err)
	}
	if err := policy.ValidateURL("http://localhost:8080/health", "url"); err != nil {
		t.Fatalf("ValidateURL localhost target with allowance: %v", err)
	}
	if err := policy.ValidateURL("http://169.254.169.254/latest/meta-data", "url"); err == nil {
		t.Fatal("ValidateURL metadata target with allowance got nil error, want blocked metadata")
	}
}

func TestCoreMonitorTargetPolicySanitizesURLs(t *testing.T) {
	got := SanitizeCoreMonitorURL("https://user:pass@example.com/health?token=secret#frag")
	if got != "https://example.com/health" {
		t.Fatalf("SanitizeCoreMonitorURL() = %q, want sanitized URL", got)
	}
	redacted := RedactCoreMonitorConfigJSON(`{"url":"https://example.com/health?token=secret","steps":[{"url":"https://api.example.com/check?api_key=secret"}]}`)
	if redacted["url"] != "https://example.com/health" {
		t.Fatalf("redacted url = %#v, want query stripped", redacted["url"])
	}
	steps, ok := redacted["steps"].([]interface{})
	if !ok || len(steps) != 1 {
		t.Fatalf("redacted steps = %#v, want one step", redacted["steps"])
	}
	step, ok := steps[0].(map[string]interface{})
	if !ok || !strings.HasPrefix(step["url"].(string), "https://api.example.com/check") || strings.Contains(step["url"].(string), "api_key") {
		t.Fatalf("redacted step url = %#v, want sanitized URL", step["url"])
	}
}
