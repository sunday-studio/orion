package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func clearEnv(t *testing.T) {
	for _, k := range []string{EnvCoreURL, EnvInterval, EnvLogLevel, EnvLogPath} {
		t.Setenv(k, "")
		t.Setenv(k+"_FILE", "")
	}
}

func TestZeroConfigFromEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvCoreURL, "https://core.example.com")
	cfg, src, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !src.ZeroConf || cfg.CoreURL != "https://core.example.com" || cfg.Interval != DefaultInterval {
		t.Fatalf("cfg=%+v src=%+v", cfg, src)
	}
}

func TestMissingFileWithoutEnvExplainsFix(t *testing.T) {
	clearEnv(t)
	_, _, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil || !strings.Contains(err.Error(), "orion-agent setup") || !strings.Contains(err.Error(), EnvCoreURL) {
		t.Fatalf("err = %v", err)
	}
}

func TestDropInsAndEnvPrecedence(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	main := filepath.Join(dir, "config.yaml")
	write(t, main, "core_url: https://a.example.com\ninterval: 30s\nmonitors:\n  - name: web\n    type: tcp\n    interval: 30s\n    tcp: {host: 127.0.0.1, port: 80}\n")
	write(t, filepath.Join(dir, "config.d", "10-more.yaml"), "interval: 45s\nmonitors:\n  - name: web\n    type: tcp\n    interval: 10s\n    tcp: {host: 127.0.0.1, port: 81}\n  - name: db\n    type: tcp\n    interval: 30s\n    tcp: {host: 127.0.0.1, port: 5432}\n")
	tokenFile := filepath.Join(dir, "url")
	write(t, tokenFile, "https://b.example.com\n")
	t.Setenv(EnvCoreURL+"_FILE", tokenFile)

	cfg, src, err := Load(main)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CoreURL != "https://b.example.com" {
		t.Fatalf("env _FILE should win, got %s", cfg.CoreURL)
	}
	if cfg.Interval != "45s" || len(cfg.Monitors) != 2 || cfg.Monitors[0].TCP.Port != 81 {
		t.Fatalf("drop-in merge wrong: %+v", cfg)
	}
	if len(src.DropIns) != 1 || len(src.EnvKeys) != 1 {
		t.Fatalf("src = %+v", src)
	}
}

func TestUnknownFieldNamesLineAndSuggests(t *testing.T) {
	clearEnv(t)
	main := filepath.Join(t.TempDir(), "config.yaml")
	write(t, main, "core_url: https://a.example.com\nintervl: 30s\n")
	_, _, err := Load(main)
	if err == nil || !strings.Contains(err.Error(), "config.yaml:2:") || !strings.Contains(err.Error(), `did you mean "interval"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestValidationErrorNamesMonitorLineAndFix(t *testing.T) {
	clearEnv(t)
	main := filepath.Join(t.TempDir(), "config.yaml")
	write(t, main, "core_url: https://a.example.com\ninterval: 30s\nmonitors:\n  - name: site\n    type: tcp\n    interval: 30s\n    tcp: {host: x.example.com, port: 99999}\n")
	_, _, err := Load(main)
	if err == nil || !strings.Contains(err.Error(), "config.yaml:4:") {
		t.Fatalf("err = %v", err)
	}
}
