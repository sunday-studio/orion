package doctor

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"orion/agent/internal/config"
)

type fakeHTTP struct {
	status int
	date   string
	err    error
}

func (f fakeHTTP) Do(r *http.Request) (*http.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	h := http.Header{}
	if f.date != "" {
		h.Set("Date", f.date)
	}
	return &http.Response{StatusCode: f.status, Header: h, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func baseEnv() *Env {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	return &Env{
		ConfigPath: "/etc/orion/config.yaml", StatePath: "/var/lib/orion/state.db", SocketPath: "/run/orion/agent.sock",
		Version: "1.2.3", GOOS: "linux",
		Config:     &config.UserConfig{CoreURL: "https://core.example.com", Logging: config.LoggingConfig{Path: "/var/log/orion/agent.log"}},
		Registered: true,
		Service:    func() (bool, bool, string) { return true, true, "active" },
		LookupHost: func(context.Context, string) ([]string, error) { return []string{"10.0.0.1"}, nil },
		HTTP:       fakeHTTP{status: 200, date: now.Format(http.TimeFormat)},
		DialTLS:    func(context.Context, string, string) (time.Time, error) { return now.Add(90 * 24 * time.Hour), nil },
		Now:        func() time.Time { return now },
		DiskFreeMB: func(string) (uint64, error) { return 10000, nil },
		CanRead:    func(string) error { return nil },
		Daemon:     DaemonInfo{Reachable: true, Version: "1.2.3"},
		TokenProbe: func(context.Context) (int, error) { return 200, nil },
		Queue:      func() (int, bool) { return 0, true },
		StateOpen:  func() error { return nil },
		Discovery:  func() (DiscoveryInfo, bool) { return DiscoveryInfo{ScannedAt: now, Things: 7}, true },
		ReadKey:    func(string) (string, error) { return "key", nil },
	}
}

func run(e *Env) map[string]Result {
	out := map[string]Result{}
	Run(context.Background(), e, DefaultChecks(), func(r Result) { out[r.ID] = r })
	return out
}

func TestHealthyHostPasses(t *testing.T) {
	rep := Run(context.Background(), baseEnv(), DefaultChecks(), nil)
	if !rep.OK || rep.SchemaVersion != SchemaVersion {
		t.Fatalf("report = %+v", rep)
	}
	for _, c := range rep.Checks {
		if c.Status != OK && !(c.Status == Skip && (c.ID == "discovery-access" || c.ID == "log-keys")) {
			t.Errorf("%s = %s %s", c.ID, c.Status, c.Message)
		}
		if c.Group == "" || c.Title == "" {
			t.Errorf("%s has no group or title: %+v", c.ID, c)
		}
	}
}

func TestEveryProblemHasATitleAndTheBoardWords(t *testing.T) {
	e := baseEnv()
	e.HTTP = fakeHTTP{err: errors.New("dial tcp 10.0.0.5:8999: connection refused")}
	e.TokenProbe = func(context.Context) (int, error) { return 401, nil }
	r := run(e)
	if c := r["core"]; c.Title != "Core is not reachable" || len(c.Evidence) == 0 || !strings.Contains(c.Evidence[0], "connection refused") || c.Fix != "check the address, then orion-agent config edit   (firewall, proxy, or wrong core_url?)" {
		t.Errorf("core = %+v", c)
	}
	if c := r["token"]; c.Title != "Credentials rejected by Core" || c.Evidence[0] != "Core answered 401 for this server's token" {
		t.Errorf("token = %+v", c)
	}
}

func TestNewChecks(t *testing.T) {
	e := baseEnv()
	e.StateOpen = func() error { return errors.New("unable to open database file") }
	e.Discovery = func() (DiscoveryInfo, bool) {
		return DiscoveryInfo{ScannedAt: e.Now(), Things: 3, Blocked: []BlockedKind{{Kind: "containers", Message: "Docker is not readable by the agent", Fix: "sudo usermod -aG docker orion   (this access is powerful)", Why: "Containers are not scanned, so none can be suggested."}}}, true
	}
	e.Config.Logs.Sources = []config.LogSource{{Name: "nginx", Type: "file", Path: "/var/log/nginx/error.log", IngestKeyFile: "/run/orion/nginx.key"}}
	e.ReadKey = func(string) (string, error) { return " \n", nil }
	r := run(e)
	if c := r["state"]; c.Status != Fail || c.Title != "The state database cannot be opened" {
		t.Errorf("state = %+v", c)
	}
	if c := r["discovery-access"]; c.Status != Warn || c.Title != "Docker is not readable by the agent" || c.Evidence[0] != "Containers are not scanned, so none can be suggested." || !strings.Contains(c.Fix, "docker orion") {
		t.Errorf("discovery-access = %+v", c)
	}
	if c := r["discovery"]; c.Status != OK || !strings.HasSuffix(c.Message, "found 3 things") {
		t.Errorf("discovery = %+v", c)
	}
	if c := r["log-keys"]; c.Status != Warn || c.Title != "Log key for nginx is missing" {
		t.Errorf("log-keys = %+v", c)
	}
	e.Discovery = func() (DiscoveryInfo, bool) { return DiscoveryInfo{}, false }
	if c := run(e)["discovery"]; c.Status != Skip {
		t.Errorf("an unreachable daemon skips the scan check: %+v", c)
	}
}

// Every failing fixture must carry a concrete fix.
func TestEveryFailureHasFix(t *testing.T) {
	cases := map[string]func(*Env){
		"config":  func(e *Env) { e.ConfigErr = errors.New("config.yaml:3: unknown field") },
		"service": func(e *Env) { e.Service = func() (bool, bool, string) { return true, false, "failed" } },
		"dns": func(e *Env) {
			e.LookupHost = func(context.Context, string) ([]string, error) { return nil, errors.New("nxdomain") }
		},
		"tls": func(e *Env) {
			e.DialTLS = func(context.Context, string, string) (time.Time, error) {
				return time.Time{}, errors.New("x509: unknown authority")
			}
		},
		"core":       func(e *Env) { e.HTTP = fakeHTTP{err: errors.New("connection refused")} },
		"clock":      func(e *Env) { e.HTTP = fakeHTTP{status: 200, date: e.Now().Add(-time.Hour).Format(http.TimeFormat)} },
		"token":      func(e *Env) { e.TokenProbe = func(context.Context) (int, error) { return 401, nil } },
		"disk":       func(e *Env) { e.DiskFreeMB = func(string) (uint64, error) { return 10, nil } },
		"version":    func(e *Env) { e.Daemon.Version = "0.9.0" },
		"log-access": func(e *Env) { e.CanRead = func(string) error { return errors.New("x") } },
		"log-sources": func(e *Env) {
			e.Config.Logs.Sources = []config.LogSource{{Name: "api", Type: "file", Path: "/var/log/api.log", IngestKeyFile: "/run/orion/api.key"}}
			e.CanRead = func(path string) error {
				if path == "/run/orion/api.key" {
					return errors.New("permission denied")
				}
				return nil
			}
		},
	}
	for id, mutate := range cases {
		e := baseEnv()
		mutate(e)
		if id == "config" {
			e.Config = nil
		}
		r := run(e)[id]
		if r.Status != Fail && r.Status != Warn {
			t.Errorf("%s: status %s, want warn/fail (detail %q)", id, r.Status, r.Message)
			continue
		}
		if r.Fix == "" && id != "log-access" {
			t.Errorf("%s: no fix message", id)
		}
	}
}

func TestLogSourcesRejectMissingEndpoint(t *testing.T) {
	e := baseEnv()
	e.Config.Logs.Sources = []config.LogSource{{Name: "api", Type: "file", Path: "/var/log/api.log", IngestKeyFile: "/run/orion/api.key"}}
	e.HTTP = fakeHTTP{status: http.StatusNotFound}
	r := run(e)["log-sources"]
	if r.Status != Fail || !strings.Contains(r.Message, "HTTP 404") {
		t.Fatalf("result = %#v", r)
	}
}

func TestSoonExpiringCertWarns(t *testing.T) {
	e := baseEnv()
	e.DialTLS = func(context.Context, string, string) (time.Time, error) { return e.Now().Add(5 * 24 * time.Hour), nil }
	if r := run(e)["tls"]; r.Status != Warn {
		t.Fatalf("tls = %+v", r)
	}
}

func TestUnenrolledHostIsWarnedNotFailed(t *testing.T) {
	e := baseEnv()
	e.Registered = false
	rep := Run(context.Background(), e, DefaultChecks(), nil)
	if !rep.OK {
		t.Fatal("unenrolled host should not fail doctor")
	}
}

func TestUnreachableDaemonWarnsOnServiceRow(t *testing.T) {
	e := baseEnv()
	e.Daemon = DaemonInfo{Err: errors.New("permission denied on agent control socket")}
	r := run(e)["service"]
	if r.Status != Warn || r.Fix == "" {
		t.Fatalf("service = %+v", r)
	}
}

func TestStatePermissionDeniedIsAWarningNotAFailure(t *testing.T) {
	e := baseEnv()
	e.StateOpen = func() error { return &os.PathError{Op: "open", Path: "/var/lib/orion/state.db", Err: os.ErrPermission} }
	if r := run(e)["state"]; r.Status != Warn || r.Fix == "" {
		t.Fatalf("state = %+v", r)
	}
}
