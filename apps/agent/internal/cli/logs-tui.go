package cli

import (
	"context"
	"strings"
	"time"

	"orion/agent/internal/logship"
	"orion/agent/internal/ui"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time
type entriesMsg []logship.Entry
type runtimeMsg runtimeState

// logsModel is the follow viewer (cli-logs artboard): status line, newest-first
// rows, key footer. It reads the spool through fetch, so it works offline.
type logsModel struct {
	th      *ui.Theme
	fetch   func() ([]logship.Entry, error)
	status  func() runtimeState
	state   logsState
	seen    map[string]bool
	sources []string
	input   textinput.Model
	typing  bool
	width   int
	height  int
	err     error
}

func newLogsModel(th *ui.Theme, fetch func() ([]logship.Entry, error), status func() runtimeState, level, source string) logsModel {
	in := textinput.New()
	in.Prompt = "/ "
	in.CharLimit = 80
	return logsModel{th: th, fetch: fetch, status: status, seen: map[string]bool{}, input: in, width: 100, height: 24,
		state: logsState{Level: level, Source: source, Following: true}}
}

func (m logsModel) Init() tea.Cmd { return tea.Batch(m.poll(), m.pollStatus(), tick()) }

func tick() tea.Cmd { return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }) }

func (m logsModel) poll() tea.Cmd {
	return func() tea.Msg {
		es, err := m.fetch()
		if err != nil {
			return err
		}
		return entriesMsg(es)
	}
}

func (m logsModel) pollStatus() tea.Cmd {
	return func() tea.Msg { return runtimeMsg(m.status()) }
}

func (m logsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
	case tickMsg:
		return m, tea.Batch(m.poll(), m.pollStatus(), tick())
	case runtimeMsg:
		m.state.Runtime = runtimeState(v)
	case error:
		m.err = v
	case entriesMsg:
		m.err = nil
		for _, e := range v {
			if m.seen[e.ID] {
				continue
			}
			m.seen[e.ID] = true
			if m.state.Paused {
				if m.state.match(e) {
					m.state.Arrived++
				}
				continue
			}
			m.state.Entries = append(m.state.Entries, e)
			if src := entrySource(e); !contains(m.sources, src) {
				m.sources = append(m.sources, src)
			}
		}
		if len(m.state.Entries) > 2000 {
			m.state.Entries = m.state.Entries[len(m.state.Entries)-2000:]
		}
	case tea.KeyMsg:
		if m.typing {
			switch v.String() {
			case "enter":
				m.state.Search, m.typing = strings.TrimSpace(m.input.Value()), false
				m.input.Blur()
			case "esc":
				m.typing = false
				m.input.Blur()
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
			return m, nil
		}
		switch v.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "p":
			m.state.Paused = !m.state.Paused
			if !m.state.Paused {
				m.state.Arrived = 0
				m.seen = map[string]bool{} // re-read the spool so nothing missed while paused is lost
				m.state.Entries = nil
			}
		case "c":
			m.state.Entries = nil
		case "l":
			i := (levelRank(m.state.Level) + 1) % len(levelOrder)
			m.state.Level = levelOrder[i]
		case "f":
			all := append([]string{""}, m.sources...)
			next := 0
			for i, s := range all {
				if s == m.state.Source {
					next = (i + 1) % len(all)
				}
			}
			m.state.Source = all[next]
		case "/":
			m.typing = true
			m.input.SetValue(m.state.Search)
			m.input.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m logsModel) View() string {
	rows := max(m.height-6, 3)
	out := logsScreen(m.th, m.state, m.width, rows)
	if m.err != nil {
		out += m.th.Tone("down", "cannot read the log spool: "+m.err.Error()) + "\n"
	}
	if m.typing {
		return out + "\n" + m.input.View()
	}
	return out + "\n" + logsFooter(m.th)
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// runLogsTUI runs the interactive follower until the user quits.
func runLogsTUI(ctx context.Context, th *ui.Theme, fetch func() ([]logship.Entry, error), status func() runtimeState, level, source string) error {
	p := tea.NewProgram(newLogsModel(th, fetch, status, level, source), tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}
