package control

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeHandler struct {
	maint   *MaintenanceRequest
	token   string
	reloads int
	logPath string
}

func (f *fakeHandler) Status() (Status, error) {
	return Status{Version: "t", PID: 7, Registered: true, AgentID: "a1"}, nil
}
func (f *fakeHandler) SetMaintenance(r MaintenanceRequest) error { f.maint = &r; return nil }
func (f *fakeHandler) ApplyToken(t string) error {
	if t == "bad" {
		return errors.New("rejected")
	}
	f.token = t
	return nil
}
func (f *fakeHandler) LogPath() string { return f.logPath }
func (f *fakeHandler) Reload() error   { f.reloads++; return nil }

func start(t *testing.T, h Handler) (*Server, *Client) {
	t.Helper()
	// Unix socket paths are length-limited; use a short temp dir.
	dir, err := os.MkdirTemp("", "oc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "a.sock")
	s := NewServer(sock, h)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, NewClient(sock)
}

func TestRoundTrip(t *testing.T) {
	h := &fakeHandler{}
	s, c := start(t, h)

	info, err := os.Stat(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o660 {
		t.Fatalf("socket mode = %v, want 0660", info.Mode().Perm())
	}
	st, err := c.Status()
	if err != nil || st.AgentID != "a1" || st.PID != 7 {
		t.Fatalf("status = %+v, %v", st, err)
	}
	if err := c.SetMaintenance(true, "deploy"); err != nil || h.maint == nil || !h.maint.Enabled || h.maint.Reason != "deploy" {
		t.Fatalf("maintenance not applied: %+v %v", h.maint, err)
	}
	if err := c.ApplyToken("tok"); err != nil || h.token != "tok" {
		t.Fatalf("token: %q %v", h.token, err)
	}
	if err := c.ApplyToken("bad"); err == nil || err.Error() != "rejected" {
		t.Fatalf("expected handler error, got %v", err)
	}
	if err := c.Reload(); err != nil || h.reloads != 1 {
		t.Fatalf("reload: %d %v", h.reloads, err)
	}
}

func TestLogs(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "agent.jsonl")
	os.WriteFile(logFile, []byte("{\"msg\":\"a\"}\n{\"msg\":\"b\"}\n"), 0o600)
	_, c := start(t, &fakeHandler{logPath: logFile})
	data, err := c.Logs()
	if err != nil || string(data) != "{\"msg\":\"a\"}\n{\"msg\":\"b\"}\n" {
		t.Fatalf("logs = %q, %v", data, err)
	}
	_, c2 := start(t, &fakeHandler{})
	if _, err := c2.Logs(); err == nil {
		t.Fatal("expected error when file logging disabled")
	}
}

func TestUnavailable(t *testing.T) {
	c := NewClient(filepath.Join(t.TempDir(), "none.sock"))
	if _, err := c.Status(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestStaleSocketReplacedLiveRefused(t *testing.T) {
	dir, _ := os.MkdirTemp("", "oc")
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "a.sock")
	os.WriteFile(sock, nil, 0o600) // stale non-socket file
	s := NewServer(sock, &fakeHandler{})
	if err := s.Start(); err != nil {
		t.Fatalf("stale socket should be replaced: %v", err)
	}
	defer s.Close()
	s2 := NewServer(sock, &fakeHandler{})
	if err := s2.Start(); err == nil {
		t.Fatal("second server must not steal a live socket")
	}
}
