package cli

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
	"time"

	"orion/node/internal/ui"
)

func TestMaintenanceScreensMatchBoard(t *testing.T) {
	th := ui.WithMode(false)
	form := boardBlock(t, "cli-maintenance.txt", "maintenance down  ·  no arguments")
	_, form, _ = strings.Cut(form, "\n")
	formBoard, rest, _ := strings.Cut(form, "\n ╭ Result")
	resultBoard := " ╭ Result" + rest
	got := stripANSI(maintenanceForm(th, maintFormState{Reason: "kernel upgrade", Focus: 2, Start: true}))
	sameAsBoard(t, "maintenance form", formBoard, got)

	now := time.Date(2026, 10, 20, 14, 32, 0, 0, time.Local)
	until := now.Add(2 * time.Hour).Add(10 * time.Minute) // 16:42
	res := stripANSI(maintenanceResultPanel(th, until, now, "kernel upgrade", 2*time.Hour))
	resBoard, short, _ := strings.Cut(resultBoard, "\n  Short form:")
	short = "Short form:" + short
	if norm(res) != norm(resBoard+"\n  "+short) {
		t.Errorf("result differs.\n--- board\n%s\n--- got\n%s", norm(resBoard+"\n  "+short), norm(res))
	}

	edge := boardBlock(t, "cli-maintenance.txt", "maintenance down  ·  edge cases")
	local := strings.Split(edge, "\n$ orion-node maintenance down --for 800h")[0]
	_, local, _ = strings.Cut(local, "\n")
	tomorrow := time.Date(2026, 10, 21, 14, 32, 0, 0, time.Local)
	gotLocal := stripANSI(maintenanceOutcome(th, true, tomorrow, now, false))
	if norm(gotLocal) != norm(local) {
		t.Errorf("local only differs.\n--- board\n%s\n--- got\n%s", norm(local), norm(gotLocal))
	}

	up := boardBlock(t, "cli-maintenance.txt", "maintenance up, and how status shows it")
	upBoard, _, _ := strings.Cut(up, "\n$ orion-node status")
	_, upBoard, _ = strings.Cut(upBoard, "\n")
	if got := stripANSI(maintenanceOutcome(th, false, time.Time{}, now, true)); norm(got) != norm(upBoard) {
		t.Errorf("maintenance up differs.\n--- board\n%s\n--- got\n%s", norm(upBoard), norm(got))
	}

	long := NewCommandError("Maintenance can last at most 720 h", errString("Try --for 720h, or end it early with orion-node maintenance up"))
	longBoard := boardBlock(t, "cli-maintenance.txt", "maintenance down  ·  edge cases")
	_, longBoard, _ = strings.Cut(longBoard, "$ orion-node maintenance down --for 800h\n")
	if got := stripANSI(renderErrorScreen(th, long, 80)); norm(got) != norm(longBoard) {
		t.Errorf("too long differs.\n--- board\n%s\n--- got\n%s", norm(longBoard), norm(got))
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestUntilText(t *testing.T) {
	now := time.Date(2026, 10, 20, 14, 32, 0, 0, time.Local)
	for _, c := range []struct {
		d    time.Duration
		want string
	}{{2 * time.Hour, "16:32"}, {24 * time.Hour, "14:32 tomorrow"}, {72 * time.Hour, "23 Oct 14:32"}} {
		if got := untilText(now.Add(c.d), now); got != c.want {
			t.Errorf("%v: %q, want %q", c.d, got, c.want)
		}
	}
}

func formKey(m *maintFormModel, k string) {
	switch k {
	case "enter":
		m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	case "right":
		m.Update(tea.KeyMsg{Type: tea.KeyRight})
	case "esc":
		m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	default:
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	}
}

func TestMaintenanceFormCollectsReasonAndLength(t *testing.T) {
	m := newMaintFormModel(ui.WithMode(false))
	for _, r := range "kernel upgrade" {
		formKey(m, string(r))
	}
	formKey(m, "enter") // to the length
	formKey(m, "right") // 1 hour
	formKey(m, "enter") // to the buttons
	formKey(m, "enter") // Start
	d, bad := m.duration()
	if !m.done || bad != "" || d != time.Hour || m.state.Reason != "kernel upgrade" {
		t.Fatalf("done=%v d=%v bad=%q reason=%q", m.done, d, bad, m.state.Reason)
	}
}

func TestMaintenanceFormCustomLengthIsValidatedAndEscCancels(t *testing.T) {
	m := newMaintFormModel(ui.WithMode(false))
	formKey(m, "enter")
	for i := 0; i < 4; i++ {
		formKey(m, "right")
	}
	formKey(m, "x") // not a length
	formKey(m, "enter")
	if m.err == "" || m.state.Focus != 1 {
		t.Fatalf("a bad custom length must stay on the field: err=%q focus=%d", m.err, m.state.Focus)
	}
	m.custom.SetValue("3d")
	m.state.Custom = "3d"
	if d, bad := m.duration(); bad != "" || d != 72*time.Hour {
		t.Fatalf("3d = %v %q", d, bad)
	}
	m.custom.SetValue("800h")
	if _, bad := m.duration(); !strings.Contains(bad, "720") {
		t.Fatalf("800h must be refused: %q", bad)
	}
	formKey(m, "esc")
	if !m.cancel {
		t.Fatal("esc cancels")
	}
}
