package agent

import (
	"os"
	"path/filepath"
	"testing"

	"orion/agent/internal/config"
)

func TestCollectServiceLogEntriesBuildsBatchAndRedactsFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.log")
	body := `{"time":"2026-05-27T20:00:00Z","level":"INFO","component":"agent","msg":"started token=secret-token","token":"secret-token"}` + "\n" +
		`{"time":"2026-05-27T20:01:00Z","level":"ERROR","component":"monitor","message":"check failed","monitor_name":"homepage","error":"timeout"}` + "\n" +
		`not-json` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	entries, err := collectServiceLogEntries(path, []config.InternalStateMonitor{{Name: "homepage", ID: "monitor-1"}}, 10)
	if err != nil {
		t.Fatalf("collectServiceLogEntries() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if entries[0].Message != "started token=[redacted]" || entries[0].Fields["token"] != "[redacted]" {
		t.Fatalf("first entry = %+v, want redacted start log", entries[0])
	}
	if entries[1].MonitorName != "homepage" || entries[1].MonitorID != "monitor-1" || entries[1].Level != "ERROR" {
		t.Fatalf("second entry = %+v, want linked monitor error log", entries[1])
	}
	if entries[0].Fingerprint == "" || entries[0].Fingerprint == entries[1].Fingerprint {
		t.Fatalf("fingerprints = %q %q, want stable distinct hashes", entries[0].Fingerprint, entries[1].Fingerprint)
	}
}

func TestCollectServiceLogEntriesKeepsNewestLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.log")
	body := `{"time":"2026-05-27T20:00:00Z","level":"INFO","msg":"first"}` + "\n" +
		`{"time":"2026-05-27T20:01:00Z","level":"INFO","msg":"second"}` + "\n" +
		`{"time":"2026-05-27T20:02:00Z","level":"INFO","msg":"third"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	entries, err := collectServiceLogEntries(path, nil, 2)
	if err != nil {
		t.Fatalf("collectServiceLogEntries() error = %v", err)
	}
	if len(entries) != 2 || entries[0].Message != "second" || entries[1].Message != "third" {
		t.Fatalf("entries = %+v, want newest two entries", entries)
	}
}
