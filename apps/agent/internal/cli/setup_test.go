package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orion/agent/internal/config"
	"orion/agent/internal/discover"
)

func fixtureDiscovery(t *testing.T) {
	t.Helper()
	old := discoverFn
	t.Cleanup(func() { discoverFn = old })
	discoverFn = func(ctx context.Context) discover.Result {
		return discover.Run(ctx, discover.Env{
			GOOS: "linux", Interval: "60s", LookupBin: func(n string) bool { return n == "systemctl" },
			Run: func(_ context.Context, name string, _ ...string) ([]byte, error) {
				return []byte("nginx.service loaded active running web\nmyapp.service loaded active running app\n"), nil
			},
		})
	}
}

func runSetupCmd(t *testing.T, cfg string, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	base := []string{"--config", cfg, "--state", filepath.Join(filepath.Dir(cfg), "s.db"), "--socket", filepath.Join(filepath.Dir(cfg), "x.sock"), "--no-color", "setup", "--yes"}
	code := Execute(context.Background(), append(base, args...), &out, &errOut)
	return out.String(), errOut.String(), code
}

func TestSetupYesAddsDiscoveredMonitorsAndIsIdempotent(t *testing.T) {
	fixtureDiscovery(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	out, errOut, code := runSetupCmd(t, cfg, "--core-url", "https://core.example.com", "--monitor", "service:nginx")
	if code != 0 {
		t.Fatalf("code=%d err=%s out=%s", code, errOut, out)
	}
	loaded, _, err := config.Load(cfg)
	if err != nil || len(loaded.Monitors) != 1 || loaded.Monitors[0].Name != "svc-nginx" {
		t.Fatalf("config = %+v err=%v", loaded, err)
	}
	// Second run: no changes, and a hand edit survives.
	data, _ := os.ReadFile(cfg)
	edited := strings.Replace(string(data), "systemd unit nginx", "hand edited", 1)
	os.WriteFile(cfg, []byte(edited), 0o640)
	out, _, code = runSetupCmd(t, cfg, "--core-url", "https://core.example.com", "--monitor", "service:nginx")
	if code != 0 || !strings.Contains(out, "already up to date") {
		t.Fatalf("second run code=%d out=%s", code, out)
	}
	after, _ := os.ReadFile(cfg)
	if !strings.Contains(string(after), "hand edited") {
		t.Fatal("setup overwrote a hand edit")
	}
}

func TestSetupDryRunWritesNothing(t *testing.T) {
	fixtureDiscovery(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	out, _, code := runSetupCmd(t, cfg, "--core-url", "https://core.example.com", "--monitor", "high", "--dry-run")
	if code != 0 || !strings.Contains(out, "+ ") {
		t.Fatalf("code=%d out=%s", code, out)
	}
	if _, err := os.Stat(cfg); err == nil {
		t.Fatal("dry run wrote the config")
	}
}

func TestSetupUnknownMonitorIdExplainsFix(t *testing.T) {
	fixtureDiscovery(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	_, errOut, code := runSetupCmd(t, cfg, "--core-url", "https://core.example.com", "--monitor", "service:ghost")
	if code == 0 || !strings.Contains(strings.Join(strings.Fields(errOut), " "), "orion-agent discover") {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
}

func TestSetupRequiresCoreURLWhenNonInteractive(t *testing.T) {
	t.Setenv("ORION_CORE_URL", "")
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	_, errOut, code := runSetupCmd(t, cfg)
	if code == 0 || !strings.Contains(errOut, "--core-url") {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
}
