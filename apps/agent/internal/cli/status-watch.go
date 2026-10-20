package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"orion/agent/internal/control"
	"orion/agent/internal/ui"
)

// watchTick is how often the live view asks the daemon for a fresh status.
const watchTick = 2 * time.Second

type watchTickMsg time.Time

type watchModel struct {
	th     *ui.Theme
	fetch  func() (control.Status, error)
	st     control.Status
	err    error
	at     time.Time
	paused bool
}

func newWatchModel(th *ui.Theme, fetch func() (control.Status, error)) watchModel {
	m := watchModel{th: th, fetch: fetch}
	m.st, m.err = fetch()
	m.at = time.Now()
	return m
}

func watchTickCmd() tea.Cmd {
	return tea.Tick(watchTick, func(t time.Time) tea.Msg { return watchTickMsg(t) })
}

func (m watchModel) Init() tea.Cmd { return watchTickCmd() }

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "p":
			m.paused = !m.paused
		case "r":
			m.st, m.err = m.fetch()
			m.at = time.Now()
		}
	case watchTickMsg:
		if !m.paused {
			m.st, m.err = m.fetch()
			m.at = time.Time(msg)
		}
		return m, watchTickCmd()
	}
	return m, nil
}

func (m watchModel) View() string {
	th := m.th
	live := th.Tone("up", "● live")
	if m.paused {
		live = th.Tone("degraded", "◐ paused")
	}
	name := m.st.Name
	if name == "" {
		name = "this host"
	}
	if m.err != nil {
		return "\n " + th.Bold(name) + "   " + th.Tone("down", "■ not running") + "\n   " +
			th.Muted("the daemon did not answer; retrying") + "\n\n" + watchKeys(th)
	}
	kind := statusKindOf(statusInput{St: m.st, Now: m.at})
	b := badges[kind]
	left := " " + th.Tone("up", "◆") + " " + th.Bold(name) + "   " + live + "  " + th.Muted("updated "+m.at.Format("15:04:05"))
	var out []string
	badge := th.Badge(b.tone, b.glyph+" "+b.text)
	out = append(out, "", left+strings.Repeat(" ", max(2, statusWidth-visibleWidth(left)-visibleWidth(badge)))+badge)
	out = append(out, " "+th.Muted(strings.Repeat("─", statusWidth-2)))
	out = append(out, fmt.Sprintf("   %s  %s   %s", th.Muted("reports"), th.Tone("up", spark(m.st.ReportMillis)), th.Muted(fmt.Sprintf("queue %d", m.st.QueuePending))), "")
	for _, ms := range m.st.MonitorStates {
		hs := healthStatus(ms.Health)
		out = append(out, "   "+th.StatusMark(hs)+" "+padRight(clip(monitorLabel(ms), 14), 15)+th.Tone(hs, padRight(healthWord(ms.Health), 7))+th.Muted(ago(m.at, ms.Since)))
	}
	out = append(out, "", watchKeys(th))
	return strings.Join(out, "\n") + "\n"
}

func watchKeys(th *ui.Theme) string {
	return "  " + th.Bold("q") + "  quit   " + th.Bold("p") + "  pause   " + th.Bold("r") + "  refresh  " + th.Muted("redraws in place, no alt-screen")
}

// runStatusWatch redraws the status in place until the user quits. It never
// takes over the whole screen, so scrollback stays intact.
func runStatusWatch(ctx context.Context, opts *Options) error {
	c := controlClient(opts)
	if opts.JSON || theme.Plain() {
		return &CommandError{Summary: "--watch needs an interactive terminal", NextSteps: []string{"orion-agent status --short  one line for scripts"}, ExitCode: 2}
	}
	m := newWatchModel(theme, c.Status)
	_, err := tea.NewProgram(m, tea.WithContext(ctx)).Run()
	return err
}
