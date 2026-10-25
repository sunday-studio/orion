package testkit

import (
	"net/http"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"
)

func TestNewDBIsFileBackedWAL(t *testing.T) {
	database := NewDB(t)
	var mode string
	database.Raw("PRAGMA journal_mode").Scan(&mode)
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}

func TestTwoNodesAreIndependent(t *testing.T) {
	database := NewDB(t)
	a, b := TwoNodes(t, database, time.Now().UTC())
	if a.Node.ID == b.Node.ID || a.Monitor.NodeID != a.Node.ID || b.Monitor.NodeID != b.Node.ID {
		t.Fatalf("fixtures not independent: %+v %+v", a, b)
	}
	var n int64
	database.Model(&db.Monitor{}).Count(&n)
	if n != 2 {
		t.Fatalf("monitors = %d, want 2", n)
	}
}

func TestFakeClockAdvances(t *testing.T) {
	c := NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	c.Advance(90 * time.Second)
	if got := c.Now(); !got.Equal(time.Date(2026, 1, 1, 0, 1, 30, 0, time.UTC)) {
		t.Fatalf("Now() = %v", got)
	}
}

func TestWebhookScripting(t *testing.T) {
	w := NewWebhook(t)
	w.FailFirst(1)
	post := func() int {
		resp, err := http.Post(w.URL(), "application/json", strings.NewReader(`{"x":1}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if post() != 500 || post() != 204 {
		t.Fatal("FailFirst(1) did not fail once then succeed")
	}
	if w.Calls() != 2 || len(w.Bodies()) != 2 {
		t.Fatalf("calls = %d bodies = %d", w.Calls(), len(w.Bodies()))
	}
}
