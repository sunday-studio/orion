package cli

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"orion/agent/internal/config"
	agentstate "orion/agent/internal/state"
)

func TestRemoteMonitorSyncDetectsChangesAndSurvivesFailure(t *testing.T) {
	var body atomic.Value
	var fail atomic.Bool
	body.Store(`{"success":true,"data":{"definitions":[{"id":"1","name":"web-port","check_type":"port","params":{"port":8080},"interval_seconds":30,"fail_after":3}]}}`)
	var gotAuth, gotPath string
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		if fail.Load() {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte(body.Load().(string)))
	}))
	defer core.Close()

	store, err := agentstate.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.UpdateRegistration("ag-1", "tok-1", core.URL); err != nil {
		t.Fatal(err)
	}
	r := &remoteMonitors{store: store}
	changed, err := r.sync()
	if err != nil || !changed || len(r.current()) != 1 {
		t.Fatalf("first sync changed=%v err=%v monitors=%v", changed, err, r.current())
	}
	if gotAuth != "Bearer tok-1" || gotPath != "/v1/agents/ag-1/monitor-definitions/sync" {
		t.Fatalf("auth=%q path=%q", gotAuth, gotPath)
	}
	if changed, _ := r.sync(); changed {
		t.Fatal("identical definitions must not count as a change")
	}
	fail.Store(true)
	if _, err := r.sync(); err == nil || len(r.current()) != 1 {
		t.Fatalf("a failed sync must keep the last good set; err=%v monitors=%v", err, r.current())
	}
	fail.Store(false)
	body.Store(`{"success":true,"data":{"definitions":[]}}`)
	if changed, _ := r.sync(); !changed || len(r.current()) != 0 {
		t.Fatal("removing a definition in the Console must remove the monitor")
	}
}

func TestEffectiveLayersConsoleChecksOverFileConfig(t *testing.T) {
	file := &config.UserConfig{Monitors: []config.UserMonitor{{Name: "a"}}}
	out := effective(file, []config.UserMonitor{{Name: "a"}, {Name: "b"}})
	if len(out.Monitors) != 2 || len(file.Monitors) != 1 {
		t.Fatalf("effective=%d file=%d (file config must not be mutated)", len(out.Monitors), len(file.Monitors))
	}
}

func TestRemoteMonitorsSurviveRestartWhenCoreIsDown(t *testing.T) {
	dir := t.TempDir()
	store, err := agentstate.Open(filepath.Join(dir, "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":true,"data":{"definitions":[{"id":"1","name":"web-port","check_type":"port","params":{"port":8080}}]}}`))
	}))
	store.UpdateRegistration("ag-1", "tok", core.URL)
	first := &remoteMonitors{store: store, path: filepath.Join(dir, "remote-monitors.json")}
	if changed, err := first.sync(); err != nil || !changed {
		t.Fatalf("sync: %v %v", changed, err)
	}
	core.Close() // Core is down when the agent restarts

	second := &remoteMonitors{store: store, path: first.path}
	second.load()
	if _, err := second.sync(); err == nil {
		t.Fatal("expected a sync error with Core down")
	}
	if got := second.current(); len(got) != 1 || got[0].Name != "web-port" {
		t.Fatalf("after restart with Core down, monitors = %+v; they must stay so registration does not unregister them", got)
	}
}
