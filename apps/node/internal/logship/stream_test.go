package logship

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestParseJournaldLine(t *testing.T) {
	body, sev, ts, cur, ok := parseJournald(`{"MESSAGE":"unit failed","PRIORITY":"3","__CURSOR":"s=abc;i=1","__REALTIME_TIMESTAMP":"1760000000000000"}`)
	if !ok || body != "unit failed" || sev != "error" || cur != "s=abc;i=1" || ts == "" {
		t.Fatalf("%q %q %q %q %v", body, sev, ts, cur, ok)
	}
	if _, _, _, _, ok := parseJournald(`{"MESSAGE":""}`); ok {
		t.Fatal("entries without a cursor or message must be skipped")
	}
}

// A journald-style source driven by a fake process: entries are shipped, the
// cursor is committed, and a restart resumes after it.
func TestStreamSourceShipsAndCommitsCursor(t *testing.T) {
	old := commandFactory
	defer func() { commandFactory = old }()
	commandFactory = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", `printf '%s\n' '{"MESSAGE":"one","PRIORITY":"6","__CURSOR":"c1"}' '{"MESSAGE":"two","PRIORITY":"4","__CURSOR":"c2"}'; sleep 30`)
	}
	dir := t.TempDir()
	sp, _ := OpenSpool(dir, 8)
	off := &memOffsets{m: map[string]Offset{}}
	col := &collector{ids: map[string]int{}}
	sh := &Shipper{Sources: []SourceSpec{{Name: "svc", Type: "journald", Unit: "x.service", FromStart: true}}, Spool: sp, Offsets: off, Sender: col, Host: "h",
		FlushEvery: 20 * time.Millisecond, SendEvery: 20 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { sh.Run(ctx); close(done) }()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		col.mu.Lock()
		n := len(col.ids)
		col.mu.Unlock()
		if n == 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	<-done
	col.mu.Lock()
	defer col.mu.Unlock()
	if col.ids["one"] != 1 || col.ids["two"] != 1 {
		t.Fatalf("shipped = %v", col.ids)
	}
	if o, ok, _ := off.Get("svc"); !ok || o.Cursor != "c2" {
		t.Fatalf("cursor = %+v", o)
	}
}
