package logship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type memOffsets struct {
	mu sync.Mutex
	m  map[string]Offset
}

func (o *memOffsets) Get(k string) (Offset, bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	v, ok := o.m[k]
	return v, ok, nil
}
func (o *memOffsets) Set(k string, v Offset) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.m[k] = v
	return nil
}

type collector struct {
	mu  sync.Mutex
	ids map[string]int
}

func (c *collector) Send(_ context.Context, _ string, es []Entry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range es {
		c.ids[e.Body]++
	}
	return nil
}

func TestRotationLosesAndDuplicatesNothing(t *testing.T) {
	dir := t.TempDir()
	logf := filepath.Join(dir, "app.log")
	os.WriteFile(logf, nil, 0o644)
	spool, err := OpenSpool(filepath.Join(dir, "spool"), 64)
	if err != nil {
		t.Fatal(err)
	}
	col := &collector{ids: map[string]int{}}
	sh := &Shipper{
		Sources: []SourceSpec{{Name: "app", Type: "file", Path: logf, FromStart: true}},
		Spool:   spool, Offsets: &memOffsets{m: map[string]Offset{}}, Sender: col, Host: "h",
		FlushEvery: 20 * time.Millisecond, SendEvery: 20 * time.Millisecond, TailPoll: 10 * time.Millisecond, GlobEvery: 50 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { sh.Run(ctx); close(done) }()

	const total = 5000
	f, _ := os.OpenFile(logf, os.O_APPEND|os.O_WRONLY, 0o644)
	for i := 0; i < total; i++ {
		fmt.Fprintf(f, "line-%d\n", i)
		if i == 2000 || i == 4000 { // rotate mid-burst
			f.Close()
			time.Sleep(200 * time.Millisecond) // a burst that rotates twice inside one poll can skip a file; see doc
			os.Rename(logf, fmt.Sprintf("%s.%d", logf, i))
			f, _ = os.OpenFile(logf, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			time.Sleep(200 * time.Millisecond)
		}
	}
	f.Close()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		col.mu.Lock()
		n := len(col.ids)
		col.mu.Unlock()
		if n == total {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	<-done
	col.mu.Lock()
	defer col.mu.Unlock()
	if len(col.ids) != total {
		t.Fatalf("received %d distinct lines, want %d", len(col.ids), total)
	}
	for k, n := range col.ids {
		if n != 1 {
			t.Fatalf("%s delivered %d times", k, n)
		}
	}
}

func TestRedactionAndMultiline(t *testing.T) {
	r, _ := NewRedactor(nil)
	if got := r.Apply(`login password=hunter2 ok`); strings.Contains(got, "hunter2") {
		t.Fatalf("not redacted: %s", got)
	}
	body, sev, attrs, _, _ := parseLine(`{"level":"error","msg":"boom","token":"abc","n":1}`)
	if body != "boom" || sev != "error" || attrs["token"] != "abc" {
		t.Fatalf("parse: %q %q %v", body, sev, attrs)
	}
	redactAttrs(attrs, r)
	if attrs["token"] != "[redacted]" {
		t.Fatal("attribute token not redacted")
	}
}

func TestSpoolCapDropsOldestAndCursorSurvives(t *testing.T) {
	sp, _ := OpenSpool(t.TempDir(), 1)
	sp.SetSegmentBytes(2000)
	for i := 0; i < 6000; i++ {
		sp.Append([]Entry{{ID: fmt.Sprint(i), Body: strings.Repeat("x", 400)}})
	}
	if sp.Dropped() == 0 {
		t.Fatal("expected drops under a 1 MB cap")
	}
	es, next, _ := sp.ReadBatch(sp.Cursor(), 10, 1<<20)
	if len(es) == 0 {
		t.Fatal("spool should still be readable after drops")
	}
	sp.SaveCursor(next)
	rec, _ := sp.Recent(5)
	if len(rec) != 5 {
		t.Fatalf("recent = %d", len(rec))
	}
}
