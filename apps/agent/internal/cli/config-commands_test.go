package cli

import (
	"bytes"
	"context"
	"orion/agent/internal/config"
	"orion/agent/internal/ui"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestConfigDiffShowsDropInOverride(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfg, []byte("core_url: https://a.example.com\ninterval: 30s\nmonitors: []\n"), 0o600)
	os.MkdirAll(filepath.Join(dir, "config.d"), 0o755)
	os.WriteFile(filepath.Join(dir, "config.d", "10.yaml"), []byte("interval: 90s\n"), 0o600)

	var out, errOut bytes.Buffer
	code := Execute(context.Background(), []string{"--config", cfg, "--no-color", "config", "diff"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "- interval: 30s") || !strings.Contains(out.String(), "+ interval: 90s") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
}

func TestConfigShowListsSources(t *testing.T) {
	t.Setenv("ORION_LOG_LEVEL", "debug")
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfg, []byte("core_url: https://a.example.com\ninterval: 30s\n"), 0o600)
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), []string{"--config", cfg, "--no-color", "config", "show"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "ORION_LOG_LEVEL overrides logging") || !strings.Contains(out.String(), "debug") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
}

func TestLineDiffIdentical(t *testing.T) {
	if d := lineDiff([]string{"a", "b"}, []string{"a", "b"}); len(d) != 0 {
		t.Fatalf("diff = %v", d)
	}
}

func TestConfigRenderScreensMatchBoard(t *testing.T) {
	th := ui.WithMode(false)
	on := true
	_ = on
	cfg := &config.UserConfig{CoreURL: "https://core.example.com", Interval: "30s",
		Logging:   config.LoggingConfig{Level: "info", Path: "/var/log/orion/agent.log"},
		Discovery: config.DiscoveryConfig{Kinds: []string{"services", "containers", "ports", "disks", "logs"}, Ignore: []string{"docker:legacy"}},
		Logs:      config.LogsConfig{Sources: []config.LogSource{{Name: "nginx", Type: "file", Path: "/var/log/nginx/error.log"}}},
		Monitors: []config.UserMonitor{
			{Name: "nginx", Type: config.UserMonitorTypeSystemd},
			{Name: "api", Type: config.UserMonitorTypeDocker, Meta: map[string]any{"source": "scan"}},
			{Name: "disk /var", Type: config.UserMonitorTypeDiskUsage, DiskUsage: &config.DiskUsageConfig{Path: "/var", MaxPercent: 85}, Meta: map[string]any{"source": "scan"}},
			{Name: "redis", Type: config.UserMonitorTypeTCP, TCP: &config.TCPMonitorConfig{Port: 6379}},
		}}
	src := config.Source{File: "/etc/orion/config.yaml", FileFound: true, DropIns: []string{"/etc/orion/config.d/10-logs.yaml"}, EnvKeys: []string{config.EnvCoreURL},
		Origins: map[string]string{"core_url": "env", "logs": "drop-in"}, LogOrigins: map[string]string{"nginx": "drop-in"},
		MonitorOrigins: map[string]string{"nginx": "file", "redis": "file"}}
	board := boardBlock(t, "cli-config.txt", "config show")
	_, board, _ = strings.Cut(board, "\n")
	// The board leaves logging out of Settings; this screen lists it so a
	// log-level override has somewhere to show.
	got := stripANSI(configShowScreen(th, cfg, src))
	got = regexp.MustCompile(`(?m)^ │ logging .*\n`).ReplaceAllString(got, "")
	// The board counts five monitors and lists four; this screen counts what it lists.
	board = strings.Replace(board, "monitors   5", "monitors   4", 1)
	sameAsBoard(t, "config show", board, got)

	diffBoard := boardBlock(t, "cli-config.txt", "validate and diff")
	_, diffBoard, _ = strings.Cut(diffBoard, "$ orion-agent config diff\n")
	gotDiff := stripANSI(configDiffScreen(th, "base file only", []string{"- core_url: https://old.example.com", "+ core_url: https://core.example.com"}, true))
	if norm(gotDiff) != norm(diffBoard) {
		t.Errorf("diff differs.\n--- board\n%s\n--- got\n%s", norm(diffBoard), norm(gotDiff))
	}
}

func TestConfigEditReviewHunksMatchBoard(t *testing.T) {
	oldCfg := "core_url: https://core.example.com\nmonitors:\n  - name: redis\n    type: tcp\n    port: 6379\nlogs:\n  endpoint: https://core.example.com/ingest\n"
	newCfg := "core_url: https://core.example.com\nmonitors:\n  - name: redis\n    type: tcp\n    port: 6380\nlogs:\n  endpoint: https://core.example.com/ingest\n  key_file: /etc/orion/nginx.key\n"
	got := stripANSI(reviewHunks(ui.WithMode(false), []byte(oldCfg), []byte(newCfg)))
	want := "@@ monitors.redis @@\n- port: 6379\n+ port: 6380\n@@ logs @@\n+ key_file: /etc/orion/nginx.key\n"
	board := boardBlock(t, "cli-config.txt", "config edit")
	if !strings.Contains(norm(board), " "+strings.ReplaceAll(strings.TrimSpace(want), "\n", "\n ")) {
		t.Fatalf("test expectation is not the board's:\n%s", board)
	}
	if norm(got) != norm(" "+strings.ReplaceAll(strings.TrimSpace(want), "\n", "\n ")+"\n") {
		t.Errorf("hunks differ.\n--- want\n%s\n--- got\n%s", norm(want), norm(got))
	}
}

func TestConfigReviewAndDiffNeverPrintCredentialValues(t *testing.T) {
	th := ui.WithMode(false)
	oldCfg := "core_url: https://c\nenrollment_token: orn_enr_old\n"
	newCfg := "core_url: https://c\nenrollment_token: orn_enr_new\n"
	hunks := stripANSI(reviewHunks(th, []byte(oldCfg), []byte(newCfg)))
	diff := stripANSI(configDiffScreen(th, "base file only", []string{"- enrollment_token: orn_enr_old", "+ enrollment_token: orn_enr_new"}, true))
	for name, out := range map[string]string{"hunks": hunks, "diff": diff} {
		if strings.Contains(out, "orn_enr_") || !strings.Contains(out, "enrollment_token: <hidden>") {
			t.Errorf("%s leaks or loses the key:\n%s", name, out)
		}
	}
}
