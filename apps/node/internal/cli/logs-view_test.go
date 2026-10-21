package cli

import (
	"errors"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
	"time"

	"orion/node/internal/control"
	"orion/node/internal/logship"
	"orion/node/internal/ui"
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

func boardLogsState(t *testing.T) logsState {
	st := logFixture()
	st.Host = "edge-05"
	st.Runtime = runtimeState{Label: "Connected", Tone: "up"}
	st.Interactive = true
	for i := range st.Entries {
		if st.Entries[i].ID == "4" { // the error line is selected, as on the board
			st.SelectedID = "4"
			st.Entries[i].Attributes = map[string]any{"upstream": "127.0.0.1:8080", "request": "GET /health"}
			st.Entries[i].Resource["log.file.path"] = "/var/log/nginx/error.log"
			shipped := true
			st.Entries[i].Shipped = &shipped
		}
	}
	st.Drawer = true
	return st
}

func TestScreenLogsMatchesBoard(t *testing.T) {
	board := boardBlock(t, "cli-logs.txt", "logs --follow")
	_, board, _ = strings.Cut(board, "\n")
	// The spool does not record when an entry was sent, so the drawer says
	// "sent to Core" without a time, and attributes are listed alphabetically.
	board = strings.Replace(board, "✓ sent to Core 14:32:06", "✓ sent to Core", 1)
	board = strings.Replace(board, "upstream 127.0.0.1:8080      request GET /health", "request  GET /health         upstream 127.0.0.1:8080", 1)
	for _, dark := range []bool{false, true} {
		th := ui.WithMode(dark)
		got := stripANSI(logsScreen(th, boardLogsState(t), 100, 6) + "\n" + logsFooter(th) + "\n")
		if norm(got) != norm(board) {
			t.Fatalf("logs differ from the board (dark=%v).\n--- board\n%s\n--- got\n%s", dark, norm(board), norm(got))
		}
		for _, l := range strings.Split(got, "\n") {
			if strings.HasPrefix(l, " ╭") || strings.HasPrefix(l, " │") || strings.HasPrefix(l, " ╰") {
				if len([]rune(l)) != 85 {
					t.Errorf("drawer line is %d wide, the board's is 85: %q", len([]rune(l)), l)
				}
			}
		}
	}
	screenGolden(t, "logs-light-100", logsScreen(ui.WithMode(false), boardLogsState(t), 100, 6))
	screenGolden(t, "logs-dark-100", logsScreen(ui.WithMode(true), boardLogsState(t), 100, 6))
}

func TestScreenLogsStatesMatchBoard(t *testing.T) {
	th := ui.WithMode(false)
	st := logFixture()
	st.Following = true
	if h := stripANSI(followChip(th, st)); !strings.Contains(h, "● Following") {
		t.Errorf("following chip: %q", h)
	}
	st.Paused, st.Arrived = true, 14
	if h := stripANSI(followChip(th, st)); !strings.Contains(h, "◐ Paused +14") {
		t.Errorf("paused chip: %q", h)
	}
	st.Paused, st.Following = false, false
	if h := stripANSI(followChip(th, st)); !strings.Contains(h, "○ Snapshot") {
		t.Errorf("snapshot chip: %q", h)
	}
	if h := stripANSI(runtimeChip(th, logRuntimeState(controlStatusZero(), errNoDaemon, time.Now()))); !strings.Contains(h, "◐ No reply") {
		t.Errorf("no reply chip: %q", h)
	}
	st.Level, st.Search = "warn", "timeout"
	empty := stripANSI(logsScreen(th, st, 100, 5))
	if !strings.Contains(norm(empty), `no log entries match level ≥ warn · search "timeout" press esc to clear`) {
		t.Errorf("empty state:\n%s", empty)
	}
}

func TestScreenLogsPlainNarrowPausedAndFooter(t *testing.T) {
	st := logFixture()
	st.Following = true
	plain := logsScreen(ui.Plain(), st, 80, 7)
	if strings.Contains(plain, "\x1b") || !strings.Contains(plain, "[● following]") && !strings.Contains(plain, "[following]") {
		t.Fatalf("plain logs must have no escapes and a bracketed state:\n%s", plain)
	}
	for _, l := range strings.Split(plain, "\n") {
		if len([]rune(l)) > 80 {
			t.Errorf("wider than 80 columns: %q", l)
		}
	}
	screenGolden(t, "logs-plain-80", plain)

	st.Paused, st.Arrived = true, 12
	if p := stripANSI(logsScreen(ui.WithMode(false), st, 100, 3)); !strings.Contains(p, "Paused +12") {
		t.Fatalf("paused state must show how many lines arrived:\n%s", p)
	}
	st.Search, st.Level = "zzz", "error"
	if p := logsScreen(ui.Plain(), st, 80, 5); !strings.Contains(p, "no log entries match") {
		t.Fatalf("empty result must say so:\n%s", p)
	}
	if f := logsFooter(ui.Plain()); !strings.Contains(f, "↑↓ move") || !strings.Contains(f, "enter details") || !strings.Contains(f, "q quit") {
		t.Fatalf("footer = %q", f)
	}
}

var errNoDaemon = errors.New("no daemon")

func controlStatusZero() control.Status { return control.Status{} }

func logsModelWithEntries(t *testing.T) logsModel {
	t.Helper()
	st := logFixture()
	m := newLogsModel(ui.WithMode(false), func() ([]logship.Entry, error) { return st.Entries, nil },
		func() runtimeState { return runtimeState{Label: "Connected", Tone: "up"} }, "info", "nginx")
	next, _ := m.Update(entriesMsg(st.Entries))
	return next.(logsModel)
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestLogsViewerCursorDrawerAndFollow(t *testing.T) {
	m := logsModelWithEntries(t)
	step := func(k string) { next, _ := m.Update(keyMsg(k)); m = next.(logsModel) }

	step("down")
	step("down")
	rows := m.state.visible(10)
	if m.state.SelectedID != rows[2].ID {
		t.Fatalf("cursor on %q, want the third newest %q", m.state.SelectedID, rows[2].ID)
	}
	step("enter")
	if !m.state.Drawer || !strings.Contains(stripANSI(m.View()), "details") {
		t.Fatal("enter opens the details drawer")
	}
	step("esc")
	if m.state.Drawer {
		t.Fatal("esc closes the drawer before it quits")
	}
	step("up")
	step("up")
	if m.state.SelectedID != "" {
		t.Fatalf("the newest line means staying with the stream, got %q", m.state.SelectedID)
	}
	step("down")
	step("f")
	if m.state.SelectedID != "" || m.state.Drawer {
		t.Fatal("f jumps back to the newest line")
	}
	if _, cmd := m.Update(keyMsg("q")); cmd == nil {
		t.Fatal("q quits")
	}
}

func TestLogsViewerSearchAndEscapeClear(t *testing.T) {
	m := logsModelWithEntries(t)
	step := func(k string) { next, _ := m.Update(keyMsg(k)); m = next.(logsModel) }
	step("/")
	if !m.typing {
		t.Fatal("/ starts a search")
	}
	for _, r := range "health" {
		step(string(r))
	}
	step("enter")
	if m.state.Search != "health" || len(m.state.visible(10)) != 1 {
		t.Fatalf("search = %q, visible %d", m.state.Search, len(m.state.visible(10)))
	}
	step("esc")
	if m.state.Search != "" {
		t.Fatal("esc clears the search")
	}
}
