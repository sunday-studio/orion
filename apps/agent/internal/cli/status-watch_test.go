package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"orion/agent/internal/control"
	"orion/agent/internal/ui"
)

func TestStatusWatchModel(t *testing.T) {
	calls := 0
	fetch := func() (control.Status, error) {
		calls++
		return control.Status{Name: "edge-05", Registered: true, LastReportAt: time.Now(), ReportIntervalSeconds: 30,
			MonitorStates: []control.MonitorState{{Name: "redis", Kind: "tcp", Health: "down", Since: time.Now().Add(-4 * time.Minute)}}}, nil
	}
	m := newWatchModel(ui.WithMode(false), fetch)
	view := stripANSI(m.View())
	for _, want := range []string{"● live", "updated", "redis", "down", "q  quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = next.(watchModel)
	before := calls
	next, cmd := m.Update(watchTickMsg(time.Now()))
	m = next.(watchModel)
	if cmd == nil {
		t.Fatal("paused tick must keep ticking")
	}
	if calls != before || !strings.Contains(stripANSI(m.View()), "paused") {
		t.Errorf("paused view must not refetch (calls %d→%d)", before, calls)
	}
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}) // resume
	m = next.(watchModel)
	next, cmd = m.Update(watchTickMsg(time.Now()))
	if msg := m.fetchCmd()(); msg.(watchStatusMsg).st.Name != "edge-05" || cmd == nil {
		t.Errorf("a running tick must fetch: %v", msg)
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); cmd == nil {
		t.Error("q must quit")
	}
	down := newWatchModel(ui.WithMode(false), func() (control.Status, error) { return control.Status{}, errors.New("no socket") })
	if !strings.Contains(stripANSI(down.View()), "not running") {
		t.Errorf("daemon down view: %s", down.View())
	}
}
