package cli

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"orion/agent/internal/ui"
)

// maintFormModel asks why and for how long when `maintenance down` is given
// no arguments on a terminal.
type maintFormModel struct {
	th     *ui.Theme
	state  maintFormState
	reason textinput.Model
	custom textinput.Model
	done   bool
	err    string
	cancel bool
}

func newMaintFormModel(th *ui.Theme) *maintFormModel {
	r := textinput.New()
	r.Prompt, r.Placeholder, r.CharLimit = "", "kernel upgrade", 120
	r.Focus()
	c := textinput.New()
	c.Prompt, c.Placeholder, c.CharLimit = "", "6h", 12
	return &maintFormModel{th: th, reason: r, custom: c, state: maintFormState{Start: true}}
}

func (m *maintFormModel) Init() tea.Cmd { return textinput.Blink }

// duration is the chosen length, or an error text for a bad custom value.
func (m *maintFormModel) duration() (time.Duration, string) {
	if m.state.Duration < len(maintenanceDurations) {
		return maintenanceDurations[m.state.Duration].D, ""
	}
	d, err := parseMaintenanceDuration(m.custom.Value())
	if err != nil {
		return 0, "enter a length such as 6h, 90m or 3d"
	}
	if d > maxMaintenanceDuration {
		return 0, "Maintenance can last at most 720 h"
	}
	return d, ""
}

// parseMaintenanceDuration accepts Go durations and whole days (3d).
func parseMaintenanceDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if n, ok := strings.CutSuffix(s, "d"); ok {
		d, err := time.ParseDuration(n + "h")
		return d * 24, err
	}
	d, err := time.ParseDuration(s)
	if err == nil && d <= 0 {
		err = errors.New("must be greater than zero")
	}
	return d, err
}

func (m *maintFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	m.err = ""
	s := &m.state
	switch key.String() {
	case "ctrl+c", "esc":
		m.cancel = true
		return m, tea.Quit
	}
	switch s.Focus {
	case 0:
		switch key.String() {
		case "enter", "tab", "down":
			s.Focus = 1
			m.reason.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.reason, cmd = m.reason.Update(msg)
		s.Reason = m.reason.Value()
		return m, cmd
	case 1:
		custom := len(maintenanceDurations)
		switch key.String() {
		case "left", "h", "shift+tab":
			s.Duration = max(0, s.Duration-1)
		case "right", "l", "tab":
			s.Duration = min(custom, s.Duration+1)
		case "up":
			s.Focus = 0
			m.reason.Focus()
		case "enter", "down":
			if s.Duration == custom && key.String() == "enter" {
				if _, bad := m.duration(); bad != "" {
					m.err = bad
					m.custom.Focus()
					return m, nil
				}
			}
			s.Focus = 2
			m.custom.Blur()
		default:
			if s.Duration == custom {
				m.custom.Focus()
				var cmd tea.Cmd
				m.custom, cmd = m.custom.Update(msg)
				s.Custom = m.custom.Value()
				return m, cmd
			}
		}
	case 2:
		switch key.String() {
		case "left", "right", "h", "l", "tab":
			s.Start = !s.Start
		case "up":
			s.Focus = 1
		case "enter":
			if !s.Start {
				m.cancel = true
				return m, tea.Quit
			}
			if _, bad := m.duration(); bad != "" {
				m.err = bad
				s.Focus = 1
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *maintFormModel) View() string {
	out := maintenanceForm(m.th, m.state)
	if m.err != "" {
		out += " " + m.th.Tone("down", m.err) + "\n"
	}
	return out
}

// askMaintenance runs the form and returns the reason and length chosen.
func askMaintenance(ctx context.Context, th *ui.Theme) (reason string, d time.Duration, ok bool, err error) {
	m := newMaintFormModel(th)
	final, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithOutput(outputWriter)).Run()
	if err != nil {
		return "", 0, false, err
	}
	fm := final.(*maintFormModel)
	if fm.cancel || !fm.done {
		return "", 0, false, nil
	}
	d, _ = fm.duration()
	return strings.TrimSpace(fm.state.Reason), d, true, nil
}
