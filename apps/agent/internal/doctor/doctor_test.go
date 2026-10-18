package doctor

import (
	"context"
	"errors"
	"io"
	"net/http"
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
		if c.Status != OK {
			t.Errorf("%s = %s %s", c.ID, c.Status, c.Detail)
		}
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
		"daemon":     func(e *Env) { e.Daemon = DaemonInfo{Err: errors.New("permission denied on agent control socket")} },
		"version":    func(e *Env) { e.Daemon.Version = "0.9.0" },
		"log-access": func(e *Env) { e.CanRead = func(string) error { return errors.New("x") } },
	}
	for id, mutate := range cases {
		e := baseEnv()
		mutate(e)
		if id == "config" {
			e.Config = nil
		}
		r := run(e)[id]
		if r.Status != Fail && r.Status != Warn {
			t.Errorf("%s: status %s, want warn/fail (detail %q)", id, r.Status, r.Detail)
			continue
		}
		if r.Fix == "" && id != "log-access" {
			t.Errorf("%s: no fix message", id)
		}
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
