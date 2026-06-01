package service

import (
	"net/http"
	"testing"
)

func TestCoreMonitorTargetPolicyRejectsBypassForms(t *testing.T) {
	policy := NewCoreMonitorTargetPolicy(nil)
	tests := []string{
		"https://user:pass@example.com/health",
		"http://[::1]:8080/health",
		"http://[::ffff:169.254.169.254]/latest/meta-data",
		"http://[fe80::1%25lo0]/health",
	}
	for _, rawURL := range tests {
		if err := policy.ValidateURL(rawURL, "url"); err == nil {
			t.Fatalf("ValidateURL(%q) got nil error, want blocked bypass form", rawURL)
		}
	}
}

func TestCoreMonitorTargetPolicyChecksRedirectTargets(t *testing.T) {
	policy := NewCoreMonitorTargetPolicy(nil)
	req, err := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data", nil)
	if err != nil {
		t.Fatalf("new redirect request: %v", err)
	}
	if err := policy.CheckRedirect(req, nil); err == nil {
		t.Fatal("CheckRedirect() to metadata address got nil error, want blocked redirect")
	}

	allowedReq, err := http.NewRequest(http.MethodGet, "https://example.com/health", nil)
	if err != nil {
		t.Fatalf("new allowed redirect request: %v", err)
	}
	if err := policy.CheckRedirect(allowedReq, nil); err != nil {
		t.Fatalf("CheckRedirect() allowed target error = %v", err)
	}
}
