package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orion/node/internal/config"
	"orion/node/internal/selfupdate"
	"orion/node/internal/transport"
)

const updatedBinary = "#!/bin/sh\necho orion-node 0.9.3\n"

func updaterFixture(t *testing.T, enabled bool) (*selfUpdater, *int, string) {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "orion-node")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(updatedBinary))
	sums := hex.EncodeToString(sum[:]) + "  orion-node-linux-amd64\n"
	env := &selfupdate.Env{
		Executable: exe, StateDir: dir, Current: "0.9.2", GOOS: "linux", GOARCH: "amd64",
		BaseURL: func(tag string) string { return "https://releases.test/" + tag },
		Fetch: func(url, dest string) error {
			if strings.HasSuffix(url, "SHA256SUMS") {
				return os.WriteFile(dest, []byte(sums), 0o644)
			}
			return os.WriteFile(dest, []byte(updatedBinary), 0o755)
		},
		Version: func(string) (string, error) { return "0.9.3", nil },
	}
	restarts := 0
	return &selfUpdater{env: env, enabled: func() bool { return enabled }, restart: func() { restarts++ }}, &restarts, hex.EncodeToString(sum[:])
}

func updateRequest(sha string) transport.PulledRequest {
	return transport.PulledRequest{ID: "req-1", Type: "update_node", Payload: map[string]any{"version": "0.9.3", "tag": "v0.9.3", "sha256": sha}}
}

func TestUpdateNodeSwapsTheBinaryAndAsksForARestart(t *testing.T) {
	u, restarts, sha := updaterFixture(t, true)
	if _, err := u.handle(updateRequest(sha)); !errors.Is(err, errUpdateRestart) {
		t.Fatalf("handle = %v, want errUpdateRestart", err)
	}
	if *restarts != 1 {
		t.Errorf("restarts = %d, want 1", *restarts)
	}
	if got, _ := os.ReadFile(u.env.Executable); string(got) != updatedBinary {
		t.Error("the new binary should be in place")
	}
}

func TestUpdateNodeRefusalsAreAnsweredInPlainWords(t *testing.T) {
	off, _, sha := updaterFixture(t, false)
	if _, err := off.handle(updateRequest(sha)); err == nil || !strings.Contains(err.Error(), "turned off") {
		t.Errorf("disabled = %v", err)
	}
	on, restarts, _ := updaterFixture(t, true)
	if _, err := on.handle(updateRequest("0000")); err == nil || errors.Is(err, errUpdateRestart) {
		t.Errorf("a bad checksum should fail, got %v", err)
	}
	if *restarts != 0 {
		t.Error("a refused update must not restart the Node")
	}
	same, _, sha := updaterFixture(t, true)
	same.env.Current = "0.9.3"
	if text, err := same.handle(updateRequest(sha)); err != nil || !strings.Contains(text, "Already on 0.9.3") {
		t.Errorf("already current = %q %v", text, err)
	}
}

func TestAnUpdateInFlightIsNotReportedByTheOldProcess(t *testing.T) {
	u, restarts, sha := updaterFixture(t, true)
	a, _, _, _, _ := newApplierFixture(t, "discovery:\n  report: true\n")
	a.updater = u
	status, text := a.apply(context.Background(), updateRequest(sha))
	if status != "" || text != "" {
		t.Fatalf("apply = %q %q: the old process must leave the result to the new version", status, text)
	}
	if *restarts != 1 {
		t.Fatalf("restarts = %d, want 1", *restarts)
	}
}

func TestNodeWithoutAnUpdaterSaysItDoesNotKnowTheRequest(t *testing.T) {
	a, _, _, _, _ := newApplierFixture(t, "discovery:\n  report: true\n")
	status, text := a.apply(context.Background(), updateRequest("x"))
	if status != "unsupported" || !strings.Contains(text, "update the node") {
		t.Errorf("got %s %q", status, text)
	}
}

func TestFinishedUpdateIsReportedOnceAndThenForgotten(t *testing.T) {
	u, _, sha := updaterFixture(t, true)
	if _, err := u.handle(updateRequest(sha)); !errors.Is(err, errUpdateRestart) {
		t.Fatal(err)
	}
	// The next run is the new version.
	next := *u.env
	next.Current = "0.9.3"
	u.env = &next
	if restart, err := u.start(context.Background()); err != nil || restart {
		t.Fatalf("start = %v %v", restart, err)
	}
	out, err := u.env.Confirm()
	if err != nil || out == nil {
		t.Fatalf("Confirm = %+v %v", out, err)
	}
	u.setPending(out)
	var got []appliedRequest
	fail := true
	report := func(id, status, text string) error {
		if fail {
			return errors.New("core is down")
		}
		got = append(got, appliedRequest{id, status, text})
		return nil
	}
	u.flush(report) // Core unreachable: keep the answer
	if m, _ := u.env.ReadMarker(); m == nil || len(got) != 0 {
		t.Fatal("the marker must stay until Core has the result")
	}
	fail = false
	u.flush(report)
	if len(got) != 1 || got[0].status != "applied" || got[0].id != "req-1" {
		t.Fatalf("reported %+v", got)
	}
	if m, _ := u.env.ReadMarker(); m != nil {
		t.Error("the marker should be gone once reported")
	}
	u.flush(report)
	if len(got) != 1 {
		t.Error("a reported update must not be reported again")
	}
}

func TestSelfUpdateIsAdvertisedOnlyWhenOnAndPossible(t *testing.T) {
	off := false
	if caps := nodeCapabilities(&config.UserConfig{RemoteUpdate: &off}); hasCap(caps, "self_update") {
		t.Errorf("remote_update: false must not advertise self_update: %v", caps)
	}
	t.Setenv("ORION_CONTAINER", "1")
	if caps := nodeCapabilities(&config.UserConfig{}); hasCap(caps, "self_update") {
		t.Errorf("a container must not advertise self_update: %v", caps)
	}
}

func hasCap(list []string, name string) bool {
	for _, x := range list {
		if x == name {
			return true
		}
	}
	return false
}
