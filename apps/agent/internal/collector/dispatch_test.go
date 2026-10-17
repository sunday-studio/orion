package collector

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"orion/agent/internal/config"
)

func TestCollectMonitorReportDispatchesByMonitorType(t *testing.T) {
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer web.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port

	for name, tc := range map[string]struct {
		monitor    config.UserMonitor
		wantStatus string
		wantErr    bool
	}{
		"http up":                       {config.UserMonitor{Type: config.UserMonitorTypeHTTPHealthcheck, HTTP: &config.HTTPHealthcheckConfig{URL: web.URL, ExpectedStatus: 200, Timeout: "2s"}}, "up", false},
		"http expected status mismatch": {config.UserMonitor{Type: config.UserMonitorTypeHTTPHealthcheck, HTTP: &config.HTTPHealthcheckConfig{URL: web.URL, ExpectedStatus: 503, Timeout: "2s"}}, "down", false},
		"tcp up":                        {config.UserMonitor{Type: config.UserMonitorTypeTCP, TCP: &config.TCPMonitorConfig{Host: "127.0.0.1", Port: port, Timeout: "2s"}}, "up", false},
		"tcp down":                      {config.UserMonitor{Type: config.UserMonitorTypeTCP, TCP: &config.TCPMonitorConfig{Host: "127.0.0.1", Port: 1, Timeout: "500ms"}}, "down", true},
		"command up":                    {config.UserMonitor{Type: config.UserMonitorTypeCommand, Command: &config.CommandMonitorConfig{Command: "true", Timeout: "2s"}}, "up", false},
		"command down":                  {config.UserMonitor{Type: config.UserMonitorTypeCommand, Command: &config.CommandMonitorConfig{Command: "false", Timeout: "2s"}}, "down", false},
		"command with args":             {config.UserMonitor{Type: config.UserMonitorTypeCommand, Command: &config.CommandMonitorConfig{Command: "echo", Args: []string{"hello"}, Timeout: "2s"}}, "up", false},
		"website up":                    {config.UserMonitor{Type: config.UserMonitorTypeWebsite, Website: &config.WebsiteMonitorConfig{URL: web.URL, Timeout: "2s"}}, "up", false},
	} {
		t.Run(name, func(t *testing.T) {
			result, err := CollectMonitorReport(config.InternalStateMonitor{Name: name, ID: "m"}, tc.monitor)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, tc.wantErr)
			}
			if result == nil {
				t.Fatal("result = nil")
			}
			if result.Status != tc.wantStatus {
				t.Fatalf("status = %q, want %q (error %v)", result.Status, tc.wantStatus, result.Error)
			}
			if result.Timestamp.IsZero() {
				t.Fatal("result has no timestamp")
			}
		})
	}
}

func TestCollectMonitorReportRejectsUnknownMonitorType(t *testing.T) {
	result, err := CollectMonitorReport(config.InternalStateMonitor{Name: "x"}, config.UserMonitor{Type: "carrier-pigeon"})
	if err == nil && (result == nil || result.Status != "down") {
		t.Fatalf("unknown type result = %+v err = %v, want an error or a down result", result, err)
	}
}

func TestCollectSystemMetricsReturnsHostFacts(t *testing.T) {
	metrics, err := Collect()
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if metrics.Timestamp == "" || metrics.UptimeSeconds == 0 || metrics.Memory.UsedPercent <= 0 {
		t.Fatalf("metrics = %+v, want populated host facts", metrics)
	}
	with, err := CollectWithOptions(CollectOptions{IncludeLocation: false})
	if err != nil || with.Location != nil {
		t.Fatalf("CollectWithOptions(no location) = location %v err %v, want none", with.Location, err)
	}
}
