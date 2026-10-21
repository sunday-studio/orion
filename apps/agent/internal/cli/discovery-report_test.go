package cli

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	agentstate "orion/agent/internal/state"
	"orion/agent/internal/transport"
)

func reporterFixture(t *testing.T, enroll bool, cfg *config.UserConfig) (*discoveryReporter, *[]transport.DiscoveryReport, *time.Time) {
	t.Helper()
	store, err := agentstate.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if enroll {
		if err := store.UpdateRegistration("agent-1", "tok", "https://core.example.com"); err != nil {
			t.Fatal(err)
		}
	}
	var sent []transport.DiscoveryReport
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	r := newDiscoveryReporter(store, func() *config.UserConfig { return cfg })
	r.now = func() time.Time { return now }
	r.send = func(url, token, id string, rep transport.DiscoveryReport) error {
		if url != "https://core.example.com" || token != "tok" || id != "agent-1" {
			t.Errorf("sent to %s as %s/%s", url, id, token)
		}
		sent = append(sent, rep)
		return nil
	}
	return r, &sent, &now
}

func scanOf(name string) control.Discovery {
	return control.Discovery{Enabled: true, ScannedAt: time.Now(),
		Kinds: []control.KindState{{Kind: "containers", State: "blocked", Fix: "sudo usermod -aG docker orion"}},
		Items: []control.DiscoveryItem{{ID: "service:" + name, Kind: "service", Name: name, Confidence: "high", Status: "suggested", New: true}}}
}

func TestReporterSendsAChangedScanOnceAndAgainAfterADay(t *testing.T) {
	r, sent, now := reporterFixture(t, true, &config.UserConfig{})
	r.Report(scanOf("nginx"))
	r.Report(scanOf("nginx")) // same things, new scan time: nothing to say
	if len(*sent) != 1 || (*sent)[0].Items[0].Name != "nginx" || (*sent)[0].Kinds[0].Fix == "" {
		t.Fatalf("sent = %+v", *sent)
	}
	r.Report(scanOf("redis"))
	if len(*sent) != 2 {
		t.Fatalf("a changed scan is sent, got %d", len(*sent))
	}
	*now = now.Add(25 * time.Hour)
	r.Report(scanOf("redis"))
	if len(*sent) != 3 {
		t.Fatalf("an unchanged scan is re-sent daily, got %d", len(*sent))
	}
}

func TestReporterStaysQuietWhenOffNotEnrolledOrCoreIsOlder(t *testing.T) {
	off := false
	r, sent, _ := reporterFixture(t, true, &config.UserConfig{Discovery: config.DiscoveryConfig{Report: &off}})
	r.Report(scanOf("nginx"))
	if len(*sent) != 0 {
		t.Fatal("discovery.report: false keeps the scan on the host")
	}
	r, sent, _ = reporterFixture(t, false, &config.UserConfig{})
	r.Report(scanOf("nginx"))
	if len(*sent) != 0 {
		t.Fatal("a host that is not enrolled has nowhere to report")
	}
	r, sent, now := reporterFixture(t, true, &config.UserConfig{})
	calls := 0
	r.send = func(string, string, string, transport.DiscoveryReport) error {
		calls++
		return transport.ErrDiscoveryUnsupported
	}
	r.Report(scanOf("nginx"))
	r.Report(scanOf("redis"))
	if calls != 1 {
		t.Fatalf("an older Core is asked once, not on every scan: %d", calls)
	}
	*now = now.Add(25 * time.Hour)
	r.Report(scanOf("redis"))
	if calls != 2 || len(*sent) != 0 {
		t.Fatalf("asked again after a day: %d", calls)
	}
}

func TestReporterRetriesAfterAFailedSend(t *testing.T) {
	r, _, _ := reporterFixture(t, true, &config.UserConfig{})
	fail := true
	calls := 0
	r.send = func(string, string, string, transport.DiscoveryReport) error {
		calls++
		if fail {
			return errors.New("connection refused")
		}
		return nil
	}
	r.Report(scanOf("nginx"))
	fail = false
	r.Report(scanOf("nginx"))
	if calls != 2 {
		t.Fatalf("a scan that failed to send is sent again next time, calls=%d", calls)
	}
}

func TestReporterCoalescesScansThatFinishWhileOneIsSending(t *testing.T) {
	r, _, _ := reporterFixture(t, true, &config.UserConfig{})
	started, release := make(chan struct{}), make(chan struct{})
	var names []string
	var mu sync.Mutex
	r.send = func(_, _, _ string, rep transport.DiscoveryReport) error {
		mu.Lock()
		names = append(names, rep.Items[0].Name)
		first := len(names) == 1
		mu.Unlock()
		if first {
			close(started)
			<-release
		}
		return nil
	}
	done := make(chan struct{})
	go func() { r.Report(scanOf("one")); close(done) }()
	<-started
	r.Report(scanOf("two"))
	r.Report(scanOf("three")) // replaces "two": only the newest waits
	close(release)
	<-done
	mu.Lock()
	defer mu.Unlock()
	if len(names) != 2 || names[0] != "one" || names[1] != "three" {
		t.Fatalf("sent %v, want [one three]", names)
	}
}

func TestReportAdvertisesAppResourcesOnlyWhileTheyAreSent(t *testing.T) {
	has := func(caps []string, c string) bool {
		for _, x := range caps {
			if x == c {
				return true
			}
		}
		return false
	}
	r, sent, _ := reporterFixture(t, true, &config.UserConfig{})
	r.Report(scanOf("nginx"))
	if len(*sent) != 1 || !has((*sent)[0].Capabilities, "requests") || !has((*sent)[0].Capabilities, "app_resources") {
		t.Fatalf("capabilities = %+v", *sent)
	}
	off := false
	r2, sent2, _ := reporterFixture(t, true, &config.UserConfig{AppResources: config.AppResourcesConfig{Enabled: &off}})
	r2.Report(scanOf("nginx"))
	if len(*sent2) != 1 || has((*sent2)[0].Capabilities, "app_resources") || !has((*sent2)[0].Capabilities, "requests") {
		t.Fatalf("turned off, the agent must not claim it sends app usage: %+v", *sent2)
	}
}
