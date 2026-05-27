package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCORSOriginsFromEnvironment(t *testing.T) {
	t.Setenv("ORION_CORS_ORIGINS", "https://console.example.com, http://localhost:5173 ")
	t.Setenv("ORION_WORKER_ID", "worker-a")
	t.Setenv("ORION_WORKER_HEARTBEAT_SECONDS", "20")
	t.Setenv("ORION_WORKER_STALE_SECONDS", "90")

	cfg := Load()

	if len(cfg.CORSOrigins) != 2 {
		t.Fatalf("CORSOrigins len = %d, want 2", len(cfg.CORSOrigins))
	}
	if cfg.CORSOrigins[0] != "https://console.example.com" || cfg.CORSOrigins[1] != "http://localhost:5173" {
		t.Fatalf("CORSOrigins = %#v", cfg.CORSOrigins)
	}
	if cfg.CoreWorkerID != "worker-a" || cfg.CoreWorkerHeartbeatSeconds != 20 || cfg.CoreWorkerStaleSeconds != 90 {
		t.Fatalf("worker config = id %q heartbeat %d stale %d", cfg.CoreWorkerID, cfg.CoreWorkerHeartbeatSeconds, cfg.CoreWorkerStaleSeconds)
	}
}

func TestValidateRejectsPartialFrontendAuthConfig(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		secret   string
	}{
		{name: "username only", username: "admin"},
		{name: "password only", password: "secret"},
		{name: "secret only", secret: "jwt-secret"},
		{name: "missing secret", username: "admin", password: "secret"},
		{name: "missing password", username: "admin", secret: "jwt-secret"},
		{name: "missing username", password: "secret", secret: "jwt-secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				AdminUsername:  tt.username,
				AdminPassword:  tt.password,
				JWTSecret:      tt.secret,
				FrontendAuthOn: tt.username != "" && tt.password != "",
			}

			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want partial frontend auth config error")
			}
		})
	}
}

func TestValidateAcceptsCompleteFrontendAuthConfig(t *testing.T) {
	cfg := &Config{
		AdminUsername:              "admin",
		AdminPassword:              "secret",
		JWTSecret:                  "jwt-secret",
		FrontendAuthOn:             true,
		CoreWorkerHeartbeatSeconds: 15,
		CoreWorkerStaleSeconds:     60,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidWorkerDiagnosticsConfig(t *testing.T) {
	tests := []struct {
		name             string
		heartbeatSeconds int
		staleSeconds     int
	}{
		{name: "zero heartbeat", heartbeatSeconds: 0, staleSeconds: 60},
		{name: "negative heartbeat", heartbeatSeconds: -1, staleSeconds: 60},
		{name: "zero stale threshold", heartbeatSeconds: 15, staleSeconds: 0},
		{name: "negative stale threshold", heartbeatSeconds: 15, staleSeconds: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				CoreWorkerHeartbeatSeconds: tt.heartbeatSeconds,
				CoreWorkerStaleSeconds:     tt.staleSeconds,
			}

			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid worker diagnostics config error")
			}
		})
	}
}

func TestLoadDotEnvLoadsValuesWithoutOverridingExistingEnvironment(t *testing.T) {
	t.Setenv("ORION_PORT", "9999")
	t.Setenv("ORION_DATA_DIR", "")
	restoreEnv(t, "ORION_ADMIN_USERNAME")
	restoreEnv(t, "ORION_ADMIN_PASSWORD")
	if err := os.Unsetenv("ORION_ADMIN_USERNAME"); err != nil {
		t.Fatalf("Unsetenv() error = %v", err)
	}
	if err := os.Unsetenv("ORION_ADMIN_PASSWORD"); err != nil {
		t.Fatalf("Unsetenv() error = %v", err)
	}

	path := filepath.Join(t.TempDir(), ".env")
	content := `
# local core config
ORION_PORT=8999
ORION_DATA_DIR="./tmp data"
ORION_ADMIN_USERNAME=admin # inline comment
export ORION_ADMIN_PASSWORD='secret value'
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv() error = %v", err)
	}

	if got := os.Getenv("ORION_PORT"); got != "9999" {
		t.Fatalf("ORION_PORT = %q, want existing value", got)
	}
	if got := os.Getenv("ORION_DATA_DIR"); got != "" {
		t.Fatalf("ORION_DATA_DIR = %q, want existing empty value", got)
	}
	if got := os.Getenv("ORION_ADMIN_USERNAME"); got != "admin" {
		t.Fatalf("ORION_ADMIN_USERNAME = %q, want admin", got)
	}
	if got := os.Getenv("ORION_ADMIN_PASSWORD"); got != "secret value" {
		t.Fatalf("ORION_ADMIN_PASSWORD = %q, want secret value", got)
	}
}

func restoreEnv(t *testing.T, key string) {
	t.Helper()
	value, existed := os.LookupEnv(key)
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, value)
			return
		}
		_ = os.Unsetenv(key)
	})
}
