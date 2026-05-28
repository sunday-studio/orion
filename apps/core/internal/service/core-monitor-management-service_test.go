package service

import "testing"

func TestValidateCoreDomainExpirationMonitorConfigAcceptsWHOISServer(t *testing.T) {
	if err := validateCoreDomainExpirationMonitorConfig(`{"domain":"example.com","whois_server":"whois.example.test:43"}`); err != nil {
		t.Fatalf("validate domain expiration config: %v", err)
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
