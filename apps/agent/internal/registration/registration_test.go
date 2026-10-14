package registration

import (
	"orion/agent/internal/config"
	"testing"
)

func TestEnrollmentTokenPrefersEnvironment(t *testing.T) {
	cfg := &config.UserConfig{EnrollmentToken: "from-yaml"}
	if got := enrollmentToken(cfg); got != "from-yaml" {
		t.Fatalf("enrollmentToken() = %q, want yaml value", got)
	}
	t.Setenv("ORION_ENROLLMENT_TOKEN", " from-env ")
	if got := enrollmentToken(cfg); got != "from-env" {
		t.Fatalf("enrollmentToken() = %q, want env value to win", got)
	}
}
