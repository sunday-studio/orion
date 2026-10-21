package cli

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"orion/node/internal/appres"
	"orion/node/internal/ui"
)

type usageTick time.Time

// usageModel redraws what apps cost every window, in place.
type usageModel struct {
	th      *ui.Theme
	sample  func() appres.Report
	rep     appres.Report
	window  time.Duration
	top     int
	sortIdx int
}

func newUsageModel(th *ui.Theme, sample func() appres.Report, window time.Duration, top int) usageModel {
	return usageModel{th: th, sample: sample, window: window, top: top, rep: sample()}
}

func (m usageModel) Init() tea.Cmd { return m.tick() }

func (m usageModel) tick() tea.Cmd {
	return tea.Tick(m.window, func(t time.Time) tea.Msg { return usageTick(t) })
}

func (m usageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case usageTick:
		m.rep = m.sample()
		return m, m.tick()
	case tea.KeyMsg:
		switch v.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "s":
			m.sortIdx = (m.sortIdx + 1) % len(usageSorts)
		}
	}
	return m, nil
}

func (m usageModel) View() string {
	return usagePanel(m.th, m.rep, usageSorts[m.sortIdx], true, m.top) + "\n  " + m.th.Bold("s") + "  " + m.th.Muted("sort") + "   " + m.th.Bold("q") + "  " + m.th.Muted("quit") + "\n"
}

func runUsageWatch(ctx context.Context, th *ui.Theme, s *appres.Sampler, window time.Duration, top int) error {
	s.Sample() // the first sample has no rates; the second does
	time.Sleep(min(window, time.Second))
	_, err := tea.NewProgram(newUsageModel(th, s.Sample, window, top), tea.WithContext(ctx), tea.WithOutput(outputWriter)).Run()
	return err
}
