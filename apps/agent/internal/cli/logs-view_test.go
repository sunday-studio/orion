package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"orion/agent/internal/logship"
	"orion/agent/internal/ui"
)

func logFixture() logsState {
	base := time.Date(2026, 10, 18, 14, 32, 8, 0, time.Local)
	mk := func(i int, sec int, sev, src, body string) logship.Entry {
		return logship.Entry{ID: fmt.Sprint(i), Timestamp: base.Add(-time.Duration(sec) * time.Second).UTC().Format(time.RFC3339Nano), Severity: sev,
			Body: body, Resource: map[string]any{"log.source": src}}
	}
	return logsState{Level: "info", Source: "nginx", Following: true, Entries: []logship.Entry{
		mk(1, 5, "info", "nginx", "POST /orders 201 91ms"),
		mk(2, 4, "info", "nginx", "GET /products 200 38ms"),
		mk(3, 3, "info", "nginx", "GET /health 502 3ms"),
		mk(4, 3, "error", "nginx", "connect() failed (111) while connecting to upstream"),
		mk(5, 2, "warn", "nginx", "upstream response is buffered to a temporary file"),
		mk(6, 1, "info", "nginx", "GET /products 200 41ms"),
		mk(7, 0, "info", "nginx", "GET /api/orders 200 84ms"),
		mk(8, 0, "debug", "nginx", "hidden below the level threshold"),
		mk(9, 0, "info", "postgres", "hidden by the source filter"),
	}}
}

// The cli-logs artboard rows, character for character (colours aside).
const artboardLogsText = `
 following   level >= info   source nginx   search: none
──────────────────────────────────────────────────────────────────────
14:32:08 info  nginx   GET /api/orders 200 84ms
14:32:07 info  nginx   GET /products 200 41ms
14:32:06 warn  nginx   upstream response is buffered to a temporary file
14:32:05 error nginx   connect() failed (111) while connecting to upstream
14:32:05 info  nginx   GET /health 502 3ms
14:32:04 info  nginx   GET /products 200 38ms
14:32:03 info  nginx   POST /orders 201 91ms
`

func TestScreenLogsMatchesArtboard(t *testing.T) {
	st := logFixture()
	for _, dark := range []bool{false, true} {
		got := stripANSI(logsScreen(ui.WithMode(dark), st, 100, 7))
		if got != artboardLogsText {
			t.Fatalf("logs differ from the artboard (dark=%v).\n--- artboard\n%s\n--- got\n%s", dark, artboardLogsText, got)
		}
	}
	screenGolden(t, "logs-light-100", logsScreen(ui.WithMode(false), st, 100, 7))
	screenGolden(t, "logs-dark-100", logsScreen(ui.WithMode(true), st, 100, 7))
}

func TestScreenLogsPlainNarrowPausedAndFooter(t *testing.T) {
	st := logFixture()
	plain := logsScreen(ui.Plain(), st, 80, 7)
	if strings.Contains(plain, "\x1b") || !strings.Contains(plain, "[following]") {
		t.Fatalf("plain logs must have no escapes and a bracketed state:\n%s", plain)
	}
	for _, l := range strings.Split(plain, "\n") {
		if len([]rune(l)) > 80 {
			t.Errorf("wider than 80 columns: %q", l)
		}
	}
	screenGolden(t, "logs-plain-80", plain)

	st.Paused, st.Arrived = true, 12
	if p := stripANSI(logsScreen(ui.WithMode(false), st, 100, 3)); !strings.Contains(p, "paused +12") {
		t.Fatalf("paused state must show how many lines arrived:\n%s", p)
	}
	st.Search, st.Level = "zzz", "error"
	if p := logsScreen(ui.Plain(), st, 80, 5); !strings.Contains(p, "no log entries match") {
		t.Fatalf("empty result must say so:\n%s", p)
	}
	if f := logsFooter(ui.Plain()); f != " q quit   f filter   l level   p pause   / search   c clear " {
		t.Fatalf("footer = %q", f)
	}
}
