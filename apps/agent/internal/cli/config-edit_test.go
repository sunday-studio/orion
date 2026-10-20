package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	"orion/agent/internal/discover"
)

const handWritten = `# my agent
core_url: https://core.example.com   # production Core
interval: 30s

monitors:
  # the one I care about
  - name: web
    type: tcp
    interval: 30s
    tcp: {host: 127.0.0.1, port: 80}
`

func TestIgnoreEditsOnlyThatKeyAndKeepsComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(handWritten), 0o640)
	// a drop-in monitor and an env override must NOT be merged into the main file
	os.MkdirAll(filepath.Join(dir, "config.d"), 0o755)
	os.WriteFile(filepath.Join(dir, "config.d", "10.yaml"), []byte("monitors:\n  - name: dropin\n    type: tcp\n    interval: 30s\n    tcp: {host: 127.0.0.1, port: 81}\n"), 0o640)
	t.Setenv("ORION_INTERVAL", "99s")

	changed, err := editDiscoveryIgnore(path, "port:5432", true)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	got, _ := os.ReadFile(path)
	text := string(got)
	for _, keep := range []string{"# my agent", "# production Core", "# the one I care about", "name: web", "interval: 30s"} {
		if !strings.Contains(text, keep) {
			t.Errorf("hand-written content lost (%q):\n%s", keep, text)
		}
	}
	for _, leak := range []string{"dropin", "99s", "logging:"} {
		if strings.Contains(text, leak) {
			t.Errorf("%q leaked into the main file:\n%s", leak, text)
		}
	}
	if !strings.Contains(text, "ignore:") || !strings.Contains(text, "port:5432") {
		t.Fatalf("ignore not written:\n%s", text)
	}
	if again, _ := editDiscoveryIgnore(path, "port:5432", true); again {
		t.Fatal("adding twice must not change the file")
	}
	if changed, _ = editDiscoveryIgnore(path, "port:5432", false); !changed {
		t.Fatal("removal must change the file")
	}
	after, _ := os.ReadFile(path)
	if strings.Contains(string(after), "port:5432") || !strings.Contains(string(after), "# my agent") {
		t.Fatalf("after removal:\n%s", after)
	}
	if none, _ := editDiscoveryIgnore(path, "port:1", false); none {
		t.Fatal("removing an absent id changes nothing")
	}
}

func TestIgnoreKeepsModeAndSymlinkedConfig(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "managed", "config.yaml")
	os.MkdirAll(filepath.Dir(real), 0o755)
	os.WriteFile(real, []byte(handWritten), 0o600)
	link := filepath.Join(dir, "config.yaml")
	os.Symlink(real, link)
	if _, err := editDiscoveryIgnore(link, "port:1:x", true); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Lstat(link); st.Mode()&os.ModeSymlink == 0 {
		t.Fatal("a symlinked (managed) config must stay a symlink")
	}
	if st, _ := os.Stat(real); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode changed to %v", st.Mode().Perm())
	}
	if b, _ := os.ReadFile(real); !strings.Contains(string(b), "port:1:x") {
		t.Fatal("the real file was not edited")
	}
}

func TestRestoreDoesNotRewriteTheConfigWhenNothingToRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(handWritten), 0o640)
	before, _ := os.Stat(path)
	_, _, code := run(t, "--config", path, "--state", filepath.Join(dir, "s.db"), "--socket", filepath.Join(dir, "none.sock"), "discover", "restore", "port:9")
	after, _ := os.Stat(path)
	if code == 0 || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("code=%d; the file must not be rewritten", code)
	}
}

func TestIgnoreTakesEffectInTheViewImmediately(t *testing.T) {
	cfg := &config.UserConfig{}
	d, _ := newTestDaemon(t, cfg)
	d.scan.env = scanFixture("myapp")
	d.scan.Rescan(context.Background())
	d.update(&config.UserConfig{Discovery: config.DiscoveryConfig{Ignore: []string{"service:myapp"}}})
	v, _ := d.Discovery()
	for _, it := range v.Items {
		if it.ID == "service:myapp" {
			t.Fatal("an ignored id must disappear from the view without waiting for a rescan")
		}
	}
}

func TestRestoreOfSomethingNotDismissedIsRefused(t *testing.T) {
	d, _ := newTestDaemon(t, &config.UserConfig{})
	d.scan.env = scanFixture()
	d.scan.Rescan(context.Background())
	if err := d.DiscoveryAction(control.DiscoveryAction{Action: "restore", ID: "service:nginx"}); err == nil || !strings.Contains(err.Error(), "not dismissed") {
		t.Fatalf("err = %v", err)
	}
}

func TestBlockedThenRecoveredScanDoesNotAnnounceEverythingAsNew(t *testing.T) {
	d, _ := newTestDaemon(t, &config.UserConfig{})
	mk := func(blockDocker bool) func(*config.UserConfig) discover.Env {
		return func(cfg *config.UserConfig) discover.Env {
			e := scanFixture()(cfg)
			e.LookupBin = func(n string) bool { return n == "systemctl" || n == "docker" }
			base := e.Run
			e.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "docker" {
					if blockDocker {
						return nil, os.ErrPermission
					}
					return []byte(`{"Names":"web","Image":"nginx","Status":"Up"}` + "\n"), nil
				}
				return base(ctx, name, args...)
			}
			return e
		}
	}
	d.scan.env = mk(false)
	d.scan.Rescan(context.Background()) // baseline with the container
	d.scan.env = mk(true)
	d.scan.Rescan(context.Background()) // docker blocked: containers absent from the scan
	d.scan.env = mk(false)
	d.scan.Rescan(context.Background()) // recovered
	st, _ := d.Status()
	if len(st.NewItems) != 0 {
		t.Fatalf("a recovered scan must not call old containers new: %+v", st.NewItems)
	}
}
