package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orion/node/internal/config"
	"orion/node/internal/control"
	"orion/node/internal/discover"
	nodestate "orion/node/internal/state"
)

func scanFixture(extra ...string) func(*config.UserConfig) discover.Env {
	return func(cfg *config.UserConfig) discover.Env {
		out := "nginx.service loaded active running web\n"
		for _, u := range extra {
			out += u + ".service loaded active running x\n"
		}
		e := discover.Env{GOOS: "linux", Interval: "60s", LookupBin: func(n string) bool { return n == "systemctl" },
			Run: func(context.Context, string, ...string) ([]byte, error) { return []byte(out), nil }}
		e.Kinds = map[string]bool{}
		for _, k := range discover.AllKinds {
			e.Kinds[k] = cfg.Discovery.KindEnabled(k)
		}
		e.Ignore, e.Existing = cfg.Discovery.Ignore, cfg.Monitors
		return e
	}
}

func newTestDaemon(t *testing.T, cfg *config.UserConfig) (*daemonControl, *nodestate.Store) {
	t.Helper()
	store, err := nodestate.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	d := newDaemonControl(&Options{ConfigPath: "/etc/orion/config.yaml"}, cfg, store)
	return d, store
}

func TestFirstScanIsBaselineThenNewThingsAreFlaggedAndNeverAddedSilently(t *testing.T) {
	cfg := &config.UserConfig{}
	d, _ := newTestDaemon(t, cfg)
	d.scan.env = scanFixture()
	v, err := d.scan.Rescan(context.Background())
	if err != nil || len(v.Items) != 1 {
		t.Fatalf("first scan: %+v %v", v, err)
	}
	if v.Items[0].New {
		t.Fatal("what exists at install time is the starting list, not 'new on this server'")
	}
	st, _ := d.Status()
	if st.Suggestions != 1 || len(st.NewItems) != 0 {
		t.Fatalf("status = suggestions %d new %d", st.Suggestions, len(st.NewItems))
	}

	d.scan.env = scanFixture("myapp")
	v, _ = d.scan.Rescan(context.Background())
	st, _ = d.Status()
	if st.Suggestions != 2 || len(st.NewItems) != 1 || st.NewItems[0].ID != "service:myapp" {
		t.Fatalf("a service that appears later must be flagged new: %+v", st)
	}
	if len(cfg.Monitors) != 0 {
		t.Fatal("nothing is added to the config silently")
	}
	if err := d.DiscoveryAction(control.DiscoveryAction{Action: "review"}); err != nil {
		t.Fatal(err)
	}
	if st, _ = d.Status(); len(st.NewItems) != 0 || st.Suggestions != 2 {
		t.Fatalf("reviewing clears 'new' but not the suggestions: %+v", st)
	}
}

func TestDismissedStaysDeclinedAcrossRescansAndRestores(t *testing.T) {
	d, _ := newTestDaemon(t, &config.UserConfig{})
	d.scan.env = scanFixture("myapp")
	d.scan.Rescan(context.Background())
	if err := d.DiscoveryAction(control.DiscoveryAction{Action: "dismiss", ID: "service:myapp"}); err != nil {
		t.Fatal(err)
	}
	d.scan.Rescan(context.Background())
	d.scan.Rescan(context.Background())
	st, _ := d.Status()
	if st.Suggestions != 1 {
		t.Fatalf("a dismissed suggestion must stay out of the count: %d", st.Suggestions)
	}
	v, _ := d.Discovery()
	found := false
	for _, it := range v.Items {
		if it.ID == "service:myapp" && it.Dismissed {
			found = true
		}
	}
	if !found {
		t.Fatalf("dismissed items stay listed as dismissed: %+v", v.Items)
	}
	if err := d.DiscoveryAction(control.DiscoveryAction{Action: "restore", ID: "service:myapp"}); err != nil {
		t.Fatal(err)
	}
	if st, _ = d.Status(); st.Suggestions != 2 {
		t.Fatalf("restore makes it a suggestion again: %d", st.Suggestions)
	}
	if err := d.DiscoveryAction(control.DiscoveryAction{Action: "dismiss", ID: "service:ghost"}); err == nil || !strings.Contains(err.Error(), "orion-node discover") {
		t.Fatalf("an unknown id explains the fix: %v", err)
	}
}

func TestScanningOffTouchesNothingAndSaysSo(t *testing.T) {
	off := false
	d, _ := newTestDaemon(t, &config.UserConfig{Discovery: config.DiscoveryConfig{Enabled: &off}})
	called := false
	d.scan.env = func(*config.UserConfig) discover.Env { called = true; return discover.Env{} }
	v, err := d.scan.Rescan(context.Background())
	if err != nil || v.Enabled || called {
		t.Fatalf("enabled=%v called=%v err=%v", v.Enabled, called, err)
	}
}

func TestHandWrittenMonitorIsAlreadyMonitoredNotSuggested(t *testing.T) {
	cfg := &config.UserConfig{Monitors: []config.UserMonitor{{Name: "mine", Type: config.UserMonitorTypeSystemd, Systemd: &config.SystemdServiceConfig{Name: "nginx"}}}}
	d, _ := newTestDaemon(t, cfg)
	d.scan.env = scanFixture()
	d.scan.Rescan(context.Background())
	st, _ := d.Status()
	if st.Suggestions != 0 {
		t.Fatalf("a hand-written entry is not suggested again: %d", st.Suggestions)
	}
	v, _ := d.Discovery()
	if v.Items[0].Status != "monitored" || v.Items[0].ManagedIn != "file" {
		t.Fatalf("%+v", v.Items[0])
	}
}

func TestDiscoverIgnoreAndRestoreEditTheConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("core_url: https://core.example.com\ninterval: 60s\n"), 0o640)
	base := []string{"--no-color", "--config", cfgPath, "--state", filepath.Join(dir, "s.db"), "--socket", filepath.Join(dir, "none.sock")}
	out, errOut, code := run(t, append(base, "discover", "ignore", "port:5432")...)
	if code != 0 || !strings.Contains(out, "Will not suggest port:5432 again") || !strings.Contains(out, "saved to discovery.ignore") {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
	cfg, _, err := config.LoadRaw(cfgPath)
	if err != nil || len(cfg.Discovery.Ignore) != 1 || cfg.Discovery.Ignore[0] != "port:5432" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
	data, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(data), "logging:") {
		t.Fatalf("editing one key must not pin defaults into the file:\n%s", data)
	}
	run(t, append(base, "discover", "ignore", "port:5432")...) // idempotent
	if cfg, _, _ = config.LoadRaw(cfgPath); len(cfg.Discovery.Ignore) != 1 {
		t.Fatalf("ignoring twice must not duplicate: %v", cfg.Discovery.Ignore)
	}
	out, _, code = run(t, append(base, "discover", "restore", "port:5432")...)
	if code != 0 || !strings.Contains(out, "port:5432 is a suggestion again") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if cfg, _, _ = config.LoadRaw(cfgPath); len(cfg.Discovery.Ignore) != 0 {
		t.Fatalf("restore removes it from discovery.ignore: %v", cfg.Discovery.Ignore)
	}
	if _, _, code = run(t, append(base, "discover", "restore", "port:1")...); code == 0 {
		t.Fatal("restoring something that was never dismissed must say so")
	}
	if _, _, code = run(t, append(base, "discover", "ignore", "nocolon")...); code == 0 {
		t.Fatal("a malformed id must be rejected")
	}
}

func TestDiscoverWithoutDaemonScansLocallyAndHonoursConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("core_url: https://core.example.com\ninterval: 60s\ndiscovery:\n  enabled: false\n"), 0o640)
	out, _, code := run(t, "--no-color", "--config", cfgPath, "--socket", filepath.Join(dir, "none.sock"), "discover")
	if code != 0 || !strings.Contains(out, "Scanning is off") || !strings.Contains(out, "discovery.enabled is false") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	js, _, _ := run(t, "--json", "--config", cfgPath, "--socket", filepath.Join(dir, "none.sock"), "discover")
	if !strings.Contains(js, `"schema_version"`) || !strings.Contains(js, `"enabled": false`) {
		t.Fatalf("json = %s", js)
	}
}
