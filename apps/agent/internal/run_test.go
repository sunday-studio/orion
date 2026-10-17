package agent

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/transport"
)

// fakeCore records every request path and answers 200 (or a fixed status).
type fakeCore struct {
	mu     sync.Mutex
	paths  []string
	status int
	server *httptest.Server
}

func newFakeCore(t *testing.T, status int) *fakeCore {
	t.Helper()
	core := &fakeCore{status: status}
	core.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		core.mu.Lock()
		core.paths = append(core.paths, r.Method+" "+r.URL.Path)
		core.mu.Unlock()
		w.WriteHeader(core.status)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(core.server.Close)
	return core
}

func (c *fakeCore) saw(substr string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, p := range c.paths {
		if strings.Contains(p, substr) {
			n++
		}
	}
	return n
}

func runnableAgent(t *testing.T, core *fakeCore, monitors []config.UserMonitor, internalMonitors []config.InternalStateMonitor) *Agent {
	t.Helper()
	store := openAgentTestStore(t)
	if err := store.UpdateRegistration("agent-1", "token", core.server.URL); err != nil {
		t.Fatal(err)
	}
	if len(internalMonitors) > 0 {
		if err := store.ReplaceMonitors(internalMonitors); err != nil {
			t.Fatal(err)
		}
	}
	a, err := NewWithStateStore(&config.UserConfig{CoreURL: core.server.URL, Interval: "1h", Monitors: monitors}, store)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func tcpMonitor(t *testing.T) (config.UserMonitor, config.InternalStateMonitor) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	addr := listener.Addr().(*net.TCPAddr)
	user := config.UserMonitor{Name: "db", Type: config.UserMonitorTypeTCP, Interval: "1h", TCP: &config.TCPMonitorConfig{Host: "127.0.0.1", Port: addr.Port, Timeout: "2s"}}
	return user, config.InternalStateMonitor{Name: "db", ID: "monitor-db"}
}

func TestRunReportsSystemAndMonitorsThenStopsOnCancel(t *testing.T) {
	core := newFakeCore(t, http.StatusOK)
	user, internal := tcpMonitor(t)
	a := runnableAgent(t, core, []config.UserMonitor{user}, []config.InternalStateMonitor{internal})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	deadline := time.Now().Add(5 * time.Second)
	for (core.saw("/report") < 2) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v, want clean stop on cancel", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not stop after cancel")
	}
	if core.saw("/v1/agents/agent-1/report") < 1 {
		t.Fatalf("no system report sent; paths: %v", core.paths)
	}
	if core.saw("/v1/agents/agent-1/monitor-db/report") < 1 {
		t.Fatalf("no monitor report sent; paths: %v", core.paths)
	}
}

func TestRunStopsWithAuthErrorWhenCoreRejectsCredentials(t *testing.T) {
	core := newFakeCore(t, http.StatusUnauthorized)
	a := runnableAgent(t, core, nil, nil)

	done := make(chan error, 1)
	go func() { done <- a.Run(context.Background()) }()
	select {
	case err := <-done:
		if !transport.IsAuthError(err) {
			t.Fatalf("Run() error = %v, want an auth error that stops reporting", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() kept going after Core rejected the credentials")
	}
}

func TestRunOnceSendsOneSystemAndMonitorReport(t *testing.T) {
	core := newFakeCore(t, http.StatusOK)
	user, internal := tcpMonitor(t)
	a := runnableAgent(t, core, []config.UserMonitor{user}, []config.InternalStateMonitor{internal})

	if err := a.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if core.saw("/v1/agents/agent-1/report") != 1 || core.saw("/v1/agents/agent-1/monitor-db/report") != 1 {
		t.Fatalf("paths = %v, want exactly one system and one monitor report", core.paths)
	}
}

func TestRunOnceSkipsMonitorsMissingFromState(t *testing.T) {
	core := newFakeCore(t, http.StatusOK)
	user, _ := tcpMonitor(t)
	a := runnableAgent(t, core, []config.UserMonitor{user}, nil) // monitor never registered

	if err := a.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v, want unregistered monitors skipped, not fatal", err)
	}
	if core.saw("/monitor-db/report") != 0 {
		t.Fatal("reported a monitor that has no registered id")
	}
}

func TestRunDoesNotReportWhileInMaintenanceAndResumesWhenItEnds(t *testing.T) {
	core := newFakeCore(t, http.StatusOK)
	a := runnableAgent(t, core, nil, nil)
	until := time.Now().Add(300 * time.Millisecond)
	if err := a.stateStore.SetMaintenanceWindow(true, nil, &until); err != nil {
		t.Fatal(err)
	}

	if err := a.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if core.saw("/report") != 0 {
		t.Fatalf("reported during maintenance: %v", core.paths)
	}
	time.Sleep(400 * time.Millisecond)
	if err := a.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if core.saw("/v1/agents/agent-1/report") != 1 {
		t.Fatalf("paths = %v, want reporting to resume once the window ended", core.paths)
	}
}

func TestReportFatalErrorNeverBlocks(t *testing.T) {
	errs := make(chan error, 1)
	reportFatalError(errs, context.Canceled)
	reportFatalError(errs, context.DeadlineExceeded) // channel full: must drop, not block
	if len(errs) != 1 {
		t.Fatalf("buffered errors = %d, want 1", len(errs))
	}
}
