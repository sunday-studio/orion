package cli

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	"orion/agent/internal/discover"
	"orion/agent/internal/ui"
)

type flowStep int

const (
	flowConnect flowStep = iota
	flowScanning
	flowChoose
	flowSettings
	flowReview
	flowApplying
	flowApplied
)

// setupFlow is the interactive setup: connect, choose, review, applied.
type setupFlow struct {
	ctx  context.Context
	opts *Options
	th   *ui.Theme

	height   int    // terminal rows, from the window size message
	title    string // "" for setup, "Discover" when discover reuses the flow
	step     flowStep
	enrolled bool
	frame    int

	conn   connectView
	plan   setupPlan
	items  []control.DiscoveryItem
	ticked map[string]bool
	cursor int
	note   string

	settings scanSettingsView
	sCursor  int
	dirty    bool // scan settings changed: rescan on the way back
	// savedSettings: discovery.* edits are written as they are made, so they
	// survive a cancel.
	savedSettings bool

	review reviewView
	result commitResult
	added  []string
	shown  []appliedMonitor
	polls  int

	err  error // set when the flow ends without applying
	done bool
}

type (
	flowTick     time.Time
	connectedMsg struct {
		conn setupConnection
		err  error
	}
	scannedMsg struct {
		res  discover.Result
		took time.Duration
	}
	committedMsg struct {
		res commitResult
		err error
	}
	polledMsg struct{ st control.Status }
)

func newSetupFlow(ctx context.Context, opts *Options, th *ui.Theme) *setupFlow {
	discoverConfigPath = opts.ConfigPath // the scan honours this config's discovery settings and existing monitors
	f := &setupFlow{ctx: ctx, opts: opts, th: th, ticked: map[string]bool{}, plan: setupPlan{Force: opts.SetupForce}}
	url := firstNonEmpty(opts.SetupCoreURL, envOrEmpty(config.EnvCoreURL))
	if url == "" {
		if existing, _, err := config.Load(opts.ConfigPath); err == nil {
			url = existing.CoreURL
		}
	}
	f.conn = connectView{URL: url}
	if cur := currentSetupState(opts.StatePath); cur != nil {
		f.enrolled = true
		f.plan.CoreURL = cur.CoreURL
		f.step = flowScanning
	}
	return f
}

func (f *setupFlow) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd()}
	if f.step == flowScanning {
		cmds = append(cmds, f.scanCmd())
	}
	return tea.Batch(cmds...)
}

func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return flowTick(t) })
}

func (f *setupFlow) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case flowTick:
		f.frame++
		return f, tickCmd()
	case tea.WindowSizeMsg:
		f.height = m.Height
	case connectedMsg:
		return f.onConnected(m)
	case scannedMsg:
		return f.onScanned(m)
	case committedMsg:
		return f.onCommitted(m)
	case polledMsg:
		return f.onPolled(m)
	case tea.KeyMsg:
		if m.String() == "ctrl+c" {
			switch f.step {
			case flowApplying: // the commit is running; let it finish
				return f, nil
			case flowApplied: // already applied: this is just leaving early
				f.done = true
				return f, tea.Quit
			}
			return f.cancel()
		}
		switch f.step {
		case flowConnect:
			return f.keyConnect(m)
		case flowChoose:
			return f.keyChoose(m)
		case flowSettings:
			return f.keySettings(m)
		case flowReview:
			return f.keyReview(m)
		case flowApplied:
			if s := m.String(); s == "q" || s == "enter" || s == "esc" {
				f.done = true
				return f, tea.Quit
			}
		}
	}
	return f, nil
}

func (f *setupFlow) cancel() (tea.Model, tea.Cmd) {
	cause := errors.New("nothing was changed")
	if f.savedSettings {
		cause = errors.New("monitors were not changed; the scan settings you saved stay in the config")
	}
	if f.title != "" { // discover: backing out is a normal way to leave
		f.done = true
		return f, tea.Quit
	}
	f.err = NewCommandError("setup cancelled", cause)
	return f, tea.Quit
}

func (f *setupFlow) View() string {
	switch f.step {
	case flowConnect:
		v := f.conn
		v.Frame = f.frame
		return connectScreen(f.th, v)
	case flowScanning:
		return margin(f.th, []string{setupHeader(f.th, [3]string{f.connectState(), stepNow, stepTodo}, f.title), "",
			" " + f.th.Tone("degraded", f.th.Spinner(f.frame)) + " " + f.th.Muted("Looking at this server…  a few seconds, read only")}) + "\n"
	case flowChoose:
		return chooseScreen(f.th, f.chooseView())
	case flowSettings:
		return scanSettingsScreen(f.th, f.settingsView())
	case flowReview:
		return reviewScreen(f.th, f.review)
	case flowApplying:
		return margin(f.th, []string{setupHeader(f.th, [3]string{f.connectState(), stepDone, stepNow}, f.title), "",
			" " + f.th.Tone("degraded", f.th.Spinner(f.frame)) + " " + f.th.Muted("Applying…")}) + "\n"
	}
	return appliedScreen(f.th, appliedView{Title: f.title, Path: f.opts.ConfigPath, Reloaded: f.result.Reloaded, Monitors: f.shown, Frame: f.frame})
}

func (f *setupFlow) connectState() string {
	if f.enrolled {
		return stepSkip
	}
	return stepDone
}

func (f *setupFlow) chooseView() chooseView {
	return chooseView{Height: f.height, Title: f.title, Items: f.items, Ticked: f.ticked, Cursor: f.cursor, Enrolled: f.enrolled, Note: f.note}
}

// --- connect ---------------------------------------------------------------

func (f *setupFlow) keyConnect(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	if f.conn.Check == "checking" {
		return f, nil
	}
	field := &f.conn.URL
	if f.conn.Focus == 1 {
		field = &f.conn.Token
	}
	switch m.Type {
	case tea.KeyEsc:
		return f.cancel()
	case tea.KeyCtrlR:
		f.conn.ShowToken = !f.conn.ShowToken
	case tea.KeyTab, tea.KeyShiftTab, tea.KeyUp, tea.KeyDown:
		f.conn.Focus = 1 - f.conn.Focus
	case tea.KeyBackspace:
		if r := []rune(*field); len(r) > 0 {
			*field = string(r[:len(r)-1])
		}
	case tea.KeyEnter:
		if f.conn.Focus == 0 {
			if err := validateCoreURL(f.conn.URL); err != nil {
				f.conn.Check, f.conn.Err = "fail", err.Error()
				return f, nil
			}
			f.conn.Check, f.conn.Focus = "", 1
			return f, nil
		}
		if strings.TrimSpace(f.conn.Token) == "" {
			f.conn.Check, f.conn.Err = "fail", "paste the one-time token from Servers › Add server"
			return f, nil
		}
		f.conn.Check = "checking"
		url, token := strings.TrimSpace(f.conn.URL), strings.TrimSpace(f.conn.Token)
		return f, func() tea.Msg {
			c, err := checkSetupConnection(f.ctx, url, token)
			return connectedMsg{conn: c, err: err}
		}
	case tea.KeyRunes, tea.KeySpace:
		*field += string(m.Runes)
		f.conn.Check = ""
	case tea.KeyCtrlU:
		*field = ""
	}
	return f, nil
}

func (f *setupFlow) onConnected(m connectedMsg) (tea.Model, tea.Cmd) {
	if m.err != nil {
		f.conn.Check, f.conn.Err = "fail", errText(m.err)
		return f, nil
	}
	f.conn.Check, f.conn.Latency = "ok", m.conn.Latency
	path, err := stageEnrollmentToken(strings.TrimSpace(f.conn.Token))
	if err != nil {
		f.err = err
		return f, tea.Quit
	}
	f.plan.CoreURL = strings.TrimSpace(f.conn.URL)
	f.plan.TokenFile, f.plan.RemoveTokenFile = path, true
	f.step = flowScanning
	return f, f.scanCmd()
}

func errText(err error) string {
	var ce *CommandError
	if errors.As(err, &ce) {
		return ce.Summary
	}
	return err.Error()
}

// --- scan and choose -------------------------------------------------------

func (f *setupFlow) scanCmd() tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		return scannedMsg{res: discoverHost(f.ctx), took: time.Since(start)}
	}
}

func (f *setupFlow) onScanned(m scannedMsg) (tea.Model, tea.Cmd) {
	f.plan.Found, f.plan.ScanTook = &m.res, m.took
	f.items = f.items[:0]
	f.ticked = map[string]bool{}
	for _, c := range m.res.Candidates {
		if c.Monitor == nil && c.StoragePath == "" && c.LogPath == "" && c.Status != discover.StatusMonitored {
			continue // seen for resource metrics only: there is nothing to tick
		}
		it := control.DiscoveryItem{ID: c.ID, Kind: string(c.Kind), Name: c.Name, Detail: c.Detail, Confidence: string(c.Confidence), Status: c.Status, ManagedIn: c.ManagedIn}
		f.items = append(f.items, it)
		f.ticked[it.ID] = preTicked(it)
	}
	f.cursor, f.step, f.note = 0, flowChoose, ""
	return f, nil
}

// selectableItems are the rows the cursor can stand on, in screen order.
func (f *setupFlow) selectableItems() []control.DiscoveryItem {
	var out []control.DiscoveryItem
	for _, kind := range discover.AllKinds {
		for _, it := range f.items {
			if discover.ConfigKind(discover.Kind(it.Kind)) == kind && !it.Dismissed && selectable(it) {
				out = append(out, it)
			}
		}
	}
	return out
}

func (f *setupFlow) keyChoose(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := f.selectableItems()
	f.note = ""
	switch m.String() {
	case "esc", "q":
		return f.cancel()
	case "up", "k":
		f.cursor = max(0, f.cursor-1)
	case "down", "j":
		f.cursor = min(max(0, len(rows)-1), f.cursor+1)
	case " ":
		if f.cursor < len(rows) {
			id := rows[f.cursor].ID
			f.ticked[id] = !f.ticked[id]
		}
	case "a":
		for _, it := range rows {
			if it.Confidence == "high" && it.Kind != "log" {
				f.ticked[it.ID] = true
			}
		}
	case "x":
		if f.cursor < len(rows) {
			id := rows[f.cursor].ID
			if _, err := editDiscoveryIgnore(f.opts.ConfigPath, id, true); err != nil {
				f.note = "could not save: " + errText(err)
				return f, nil
			}
			f.savedSettings = true
			for i := range f.items {
				if f.items[i].ID == id {
					f.items[i].Dismissed = true
				}
			}
			delete(f.ticked, id)
			f.cursor = min(f.cursor, max(0, len(rows)-2))
		}
	case "s":
		f.openSettings()
	case "enter":
		return f.toReview()
	}
	return f, nil
}

// runFlow runs the interactive setup and returns what it applied. The flow
// commits the plan itself, so the caller must not apply it again.
func runFlow(ctx context.Context, opts *Options) error {
	return runFlowWith(ctx, newSetupFlow(ctx, opts, theme))
}

func runFlowWith(ctx context.Context, f *setupFlow) error {
	final, err := tea.NewProgram(f, tea.WithContext(ctx), tea.WithOutput(outputWriter)).Run()
	if f.plan.RemoveTokenFile {
		_ = os.Remove(f.plan.TokenFile) // whatever ended the program, never leave the token behind
	}
	if err != nil {
		return NewCommandError("setup wizard failed", err)
	}
	if ff, ok := final.(*setupFlow); ok && ff.err != nil {
		return ff.err
	}
	return nil
}
