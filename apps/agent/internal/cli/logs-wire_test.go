package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	agentstate "orion/agent/internal/state"
)

// End to end: a log file is tailed, redacted, spooled, shipped to a fake Core
// with the per-source key, and readable offline through the control socket.
func TestLogShipperEndToEndWithOfflineRead(t *testing.T) {
	core := newFakeCore(t)
	dir, _ := os.MkdirTemp("", "ls")
	t.Cleanup(func() { os.RemoveAll(dir) })
	logf := filepath.Join(dir, "app.log")
	os.WriteFile(logf, []byte("started ok\nlogin password=hunter2 done\n"), 0o644)
	keyf := filepath.Join(dir, "key")
	os.WriteFile(keyf, []byte("orion_ik_e2e"), 0o600)

	store, err := agentstate.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := &config.UserConfig{CoreURL: core.srv.URL, Interval: "60s", Logs: config.LogsConfig{Sources: []config.LogSource{
		{Name: "app", Type: "file", Path: logf, From: "beginning", IngestKeyFile: keyf},
	}}}
	opts := &Options{ConfigPath: filepath.Join(dir, "c.yaml"), StatePath: filepath.Join(dir, "state.db"), SocketPath: filepath.Join(dir, "s.sock")}
	ctrl := newDaemonControl(opts, cfg, store)
	srv := control.NewServer(opts.SocketPath, ctrl)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	wait := startLogShipper(ctx, opts, cfg, store, ctrl)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		core.mu.Lock()
		n := len(core.bodies)
		core.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()
	wait()

	core.mu.Lock()
	defer core.mu.Unlock()
	if len(core.bodies) == 0 {
		t.Fatal("nothing reached Core")
	}
	body := strings.Join(core.bodies, "")
	if !strings.Contains(body, "started ok") || strings.Contains(body, "hunter2") || !strings.Contains(body, "[redacted]") {
		t.Fatalf("body = %s", body)
	}
	if core.auth[0] != "Bearer orion_ik_e2e" {
		t.Fatalf("auth = %q", core.auth[0])
	}
	if st, _ := ctrl.Status(); st.LogSources != 1 {
		t.Fatalf("status must report the configured source, got %d", st.LogSources)
	}
	entries, err := fetchSpool(opts, 10)
	if err != nil || len(entries) != 2 {
		t.Fatalf("offline read through the socket: %v %v", entries, err)
	}
}

func TestLogsConfigValidationExplainsFixes(t *testing.T) {
	bad := []struct {
		src  config.LogSource
		want string
	}{
		{config.LogSource{Name: "a", Type: "file", Path: "relative.log", IngestKeyEnv: "K"}, "absolute"},
		{config.LogSource{Name: "a", Type: "file", Path: "/x.log"}, "ingest_key_file or ingest_key_env"},
		{config.LogSource{Name: "a", Type: "journald", IngestKeyEnv: "K"}, "unit is required"},
		{config.LogSource{Name: "a", Type: "tail", IngestKeyEnv: "K"}, "type must be"},
		{config.LogSource{Name: "a", Type: "file", Path: "/x", MultilineStart: "(", IngestKeyEnv: "K"}, "multiline_start"},
	}
	for _, c := range bad {
		err := config.LogsConfig{Sources: []config.LogSource{c.src}}.Validate()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: err = %v, want %q", c.src, err, c.want)
		}
	}
	dup := config.LogsConfig{Sources: []config.LogSource{{Name: "a", Type: "docker", Container: "c", IngestKeyEnv: "K"}, {Name: "a", Type: "docker", Container: "d", IngestKeyEnv: "K"}}}
	if err := dup.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("err = %v", err)
	}
}
