package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	agentstate "orion/agent/internal/state"
)

// startDaemonControl runs a real control server over a real state store.
func startDaemonControl(t *testing.T) (sock string, store *agentstate.Store) {
	t.Helper()
	dir, err := os.MkdirTemp("", "oc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	store, err = agentstate.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.UpdateRegistration("agent-1", "secret-token", ""); err != nil {
		t.Fatal(err)
	}
	opts := &Options{ConfigPath: "/etc/orion/config.yaml"}
	srv := control.NewServer(filepath.Join(dir, "a.sock"), newDaemonControl(opts, &config.UserConfig{}, store))
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	return srv.Path(), store
}

func TestStatusViaSocketWhenStateUnreadable(t *testing.T) {
	sock, _ := startDaemonControl(t)
	var out, errOut bytes.Buffer
	missing := filepath.Join(t.TempDir(), "unreadable.db")
	Execute(context.Background(), []string{"--json", "--socket", sock, "--state", missing, "status"}, &out, &errOut)
	if strings.Contains(out.String(), "secret-token") {
		t.Fatalf("status leaked the token: %s", out.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "agent-1") {
		t.Fatalf("status did not use the socket: %s", out.String())
	}
}

func TestMaintenanceViaSocketNeedsNoStateAccess(t *testing.T) {
	sock, store := startDaemonControl(t)
	missing := filepath.Join(t.TempDir(), "unreadable.db")
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), []string{"--socket", sock, "--state", missing, "--no-color", "maintenance", "down", "deploying"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d err=%s out=%s", code, errOut.String(), out.String())
	}
	st, err := store.Get()
	if err != nil {
		t.Fatal(err)
	}
	if !st.MaintenanceMode || st.MaintenanceReason == nil || *st.MaintenanceReason != "deploying" {
		t.Fatalf("state = %+v", st)
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("CLI must not touch the state file when the daemon handled the request")
	}
}

func TestTokenApplyViaSocket(t *testing.T) {
	sock, store := startDaemonControl(t)
	_ = store
	tok := filepath.Join(t.TempDir(), "tok")
	os.WriteFile(tok, []byte("new-token\n"), 0o600)
	var out, errOut bytes.Buffer
	// Registration needs a core URL; the daemon must refuse cleanly rather than fall back.
	code := Execute(context.Background(), []string{"--socket", sock, "--no-color", "token", "apply", "--token-file", tok}, &out, &errOut)
	if code == 0 {
		t.Fatalf("expected daemon rejection for unregistered core URL, out=%s", out.String())
	}
	if strings.Contains(out.String()+errOut.String(), "new-token") {
		t.Fatal("output leaked token")
	}
}

func TestDoctorJSONIsVersionedAndEveryFailureHasFix(t *testing.T) {
	var out, errOut bytes.Buffer
	dir := t.TempDir()
	Execute(context.Background(), []string{"--json", "--config", filepath.Join(dir, "none.yaml"), "--state", filepath.Join(dir, "s.db"), "--socket", filepath.Join(dir, "x.sock"), "doctor"}, &out, &errOut)
	var rep struct {
		SchemaVersion int  `json:"schema_version"`
		OK            bool `json:"ok"`
		Checks        []struct {
			ID, Status, Fix string
		} `json:"checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if rep.SchemaVersion != 2 || rep.OK || len(rep.Checks) < 8 {
		t.Fatalf("report = %+v", rep)
	}
	for _, c := range rep.Checks {
		if c.Status == "fail" && c.Fix == "" {
			t.Errorf("check %s failed without a fix", c.ID)
		}
	}
}
