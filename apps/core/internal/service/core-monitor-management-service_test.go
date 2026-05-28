package service

import "testing"

func TestValidateCoreDomainExpirationMonitorConfigAcceptsWHOISServer(t *testing.T) {
	if err := validateCoreDomainExpirationMonitorConfig(`{"domain":"example.com","whois_server":"whois.example.test:43"}`); err != nil {
		t.Fatalf("validate domain expiration config: %v", err)
	}
}

func TestValidateCoreSyntheticMonitorConfigAcceptsBrowserStep(t *testing.T) {
	raw := `{"steps":[{"type":"browser","name":"later-browser-step"}]}`
	if err := validateCoreSyntheticMonitorConfig(raw); err != nil {
		t.Fatalf("validate synthetic browser step: %v", err)
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

func TestValidateCorePlaywrightMonitorConfigRejectsWorkerConstraints(t *testing.T) {
	tests := []string{
		`{"steps":[{"action":"hover","selector":"button"}]}`,
		`{"steps":[{"action":"click"}]}`,
		`{"url":"https://example.com","artifact_limit_bytes":262145}`,
		`{"steps":[{"action":"goto","url":"ftp://example.com"}]}`,
	}
	for _, raw := range tests {
		if err := validateCorePlaywrightMonitorConfig(raw); err == nil {
			t.Fatalf("validate playwright config %s: got nil error, want validation error", raw)
		}
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
