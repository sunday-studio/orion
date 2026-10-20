package cli

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	"orion/agent/internal/discover"
)

// --- scan settings ---------------------------------------------------------

func (f *setupFlow) openSettings() {
	disc := readDiscoveryConfig(f.opts.ConfigPath)
	f.settings = scanSettingsView{Title: f.title, Enabled: disc.IsEnabled(), Kinds: map[string]bool{}, Enrolled: f.enrolled}
	for _, kl := range scanKindLabels {
		f.settings.Kinds[kl[0]] = disc.KindEnabled(kl[0])
	}
	for _, id := range disc.Ignore {
		f.settings.Never = append(f.settings.Never, neverItem{ID: id, When: "in discovery.ignore"})
	}
	f.sCursor, f.step = 0, flowSettings
}

func (f *setupFlow) settingsView() scanSettingsView {
	v := f.settings
	v.Cursor = f.sCursor
	return v
}

func (f *setupFlow) keySettings(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := 1 + len(scanKindLabels) + len(f.settings.Never)
	f.settings.Note = ""
	fail := func(err error) { f.settings.Note = "could not save: " + errText(err) }
	defer func() { f.savedSettings = f.savedSettings || f.dirty }()
	switch m.String() {
	case "esc", "q":
		f.step = flowChoose
		if f.dirty {
			f.dirty, f.step = false, flowScanning
			return f, f.scanCmd()
		}
	case "up", "k":
		f.sCursor = max(0, f.sCursor-1)
	case "down", "j":
		f.sCursor = min(rows-1, f.sCursor+1)
	case " ", "enter":
		switch {
		case f.sCursor == 0:
			on := !f.settings.Enabled
			if _, err := editDiscoveryEnabled(f.opts.ConfigPath, on); err != nil {
				fail(err)
				return f, nil
			}
			f.settings.Enabled, f.dirty = on, true
		case f.sCursor <= len(scanKindLabels):
			kind := scanKindLabels[f.sCursor-1][0]
			on := !f.settings.Kinds[kind]
			if _, err := editDiscoveryKind(f.opts.ConfigPath, kind, on, discover.AllKinds); err != nil {
				fail(err)
				return f, nil
			}
			f.settings.Kinds[kind], f.dirty = on, true
		}
	case "r":
		if i := f.sCursor - 1 - len(scanKindLabels); i >= 0 && i < len(f.settings.Never) {
			if _, err := editDiscoveryIgnore(f.opts.ConfigPath, f.settings.Never[i].ID, false); err != nil {
				fail(err)
				return f, nil
			}
			f.settings.Never = append(f.settings.Never[:i], f.settings.Never[i+1:]...)
			f.sCursor = min(f.sCursor, rows-2)
			f.dirty = true
		}
	}
	return f, nil
}

// --- review and apply ------------------------------------------------------

func (f *setupFlow) toReview() (tea.Model, tea.Cmd) {
	var ids []string
	for _, it := range f.selectableItems() {
		if f.ticked[it.ID] && it.Kind != "log" {
			ids = append(ids, it.ID)
		}
	}
	sel, err := discover.Select(f.plan.Found.Candidates, ids)
	if err != nil {
		f.note = errText(err)
		return f, nil
	}
	f.plan.Selected = sel
	rows, notWritten, added, err := reviewRows(f.opts, f.plan, f.items)
	if err != nil {
		f.note = errText(err)
		return f, nil
	}
	f.added = added
	f.review = reviewView{Title: f.title, Path: f.opts.ConfigPath, Rows: rows, NotWrite: notWritten, Apply: true, Enrolled: f.enrolled}
	f.step = flowReview
	return f, nil
}

func (f *setupFlow) keyReview(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.String() {
	case "esc":
		f.step = flowChoose
	case "left", "h", "right", "l", "tab":
		f.review.Apply = !f.review.Apply
	case "enter":
		if !f.review.Apply {
			return f.cancel()
		}
		f.step = flowApplying
		plan := f.plan
		return f, func() tea.Msg {
			res, err := commitSetup(f.ctx, f.opts, plan)
			return committedMsg{res: res, err: err}
		}
	}
	return f, nil
}

func (f *setupFlow) onCommitted(m committedMsg) (tea.Model, tea.Cmd) {
	if f.plan.RemoveTokenFile {
		_ = os.Remove(f.plan.TokenFile)
	}
	if m.err != nil {
		f.err = m.err
		return f, tea.Quit
	}
	f.result = m.res
	for _, n := range f.added {
		f.shown = append(f.shown, appliedMonitor{Name: n})
	}
	f.step = flowApplied
	if !m.res.Reloaded || len(f.shown) == 0 {
		f.done = true
		return f, tea.Quit
	}
	return f, f.pollCmd()
}

func (f *setupFlow) pollCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		st, _ := controlClient(f.opts).Status()
		return polledMsg{st: st}
	})
}

func (f *setupFlow) onPolled(m polledMsg) (tea.Model, tea.Cmd) {
	f.polls++
	seen := map[string]control.MonitorState{}
	for _, s := range m.st.MonitorStates {
		seen[s.Name] = s
	}
	ready := 0
	for i := range f.shown {
		if s, ok := seen[f.shown[i].Name]; ok {
			f.shown[i].Ready, f.shown[i].Since = true, time.Since(s.Since)
			ready++
		}
	}
	if ready == len(f.shown) || f.polls >= 12 {
		f.done = true
		return f, tea.Quit
	}
	return f, f.pollCmd()
}

// readDiscoveryConfig reads only the discovery block of the main config file.
// It works on a half-written file (setup may run before core_url exists).
func readDiscoveryConfig(path string) config.DiscoveryConfig {
	var doc struct {
		Discovery config.DiscoveryConfig `yaml:"discovery"`
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(data, &doc)
	}
	return doc.Discovery
}
