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

func TestSetupAcceptDiscoveredSelectsHighConfidenceMonitors(t *testing.T) {
	fixtureDiscovery(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	out, errOut, code := runSetupCmd(t, cfg, "--core-url", "https://core.example.com", "--accept-discovered", "--dry-run")
	if code != 0 || !strings.Contains(out, "svc-nginx") {
		t.Fatalf("code=%d err=%s out=%s", code, errOut, out)
	}
}

func TestSetupTokenIsStagedForEnrollmentAndRemovedOnDryRun(t *testing.T) {
	fixtureDiscovery(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	opts := &Options{ConfigPath: cfg, SetupCoreURL: "https://core.example.com", SetupToken: "orn_enr_secret", SetupDryRun: true}
	plan, err := planFromFlags(context.Background(), opts)
	if err != nil || !plan.RemoveTokenFile || plan.TokenFile == "" {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	if data, err := os.ReadFile(plan.TokenFile); err != nil || string(data) != "orn_enr_secret" {
		t.Fatalf("staged token = %q, %v", data, err)
	}
	if err := applySetup(context.Background(), opts, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plan.TokenFile); !os.IsNotExist(err) {
		t.Fatalf("staged token still exists: %v", err)
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

func TestSetupAcceptDiscoveredPrintsTheBoardSummary(t *testing.T) {
	fixtureDiscovery(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	out, errOut, code := runSetupCmd(t, cfg, "--core-url", "https://core.example.com", "--accept-discovered")
	if code != 0 {
		t.Fatalf("code=%d err=%s out=%s", code, errOut, out)
	}
	for _, want := range []string{"[ok]   Scanned: found 2 things", "[ok]   Accepted: 1 high-confidence", "[ok]   Config written: " + cfg, "--dry-run prints the diff and stops."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
