package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	"orion/agent/internal/discover"
	"orion/agent/internal/ui"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func press(f *setupFlow, keys ...string) {
	for _, k := range keys {
		f.Update(key(k))
	}
}

// newTestFlow starts a flow on a fixture host with a temporary config.
func newTestFlow(t *testing.T) (*setupFlow, string) {
	t.Helper()
	fixtureDiscovery(t)
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	opts := &Options{ConfigPath: cfg, StatePath: filepath.Join(dir, "s.db"), SocketPath: filepath.Join(dir, "x.sock")}
	f := newSetupFlow(context.Background(), opts, ui.WithMode(false))
	f.plan.CoreURL = "https://core.example.com"
	f.Update(f.scanCmd()())
	if f.step != flowChoose {
		t.Fatalf("step = %v, want choose", f.step)
	}
	return f, cfg
}

func TestFlowConnectValidatesBeforeChecking(t *testing.T) {
	fixtureDiscovery(t)
	f := newSetupFlow(context.Background(), &Options{ConfigPath: filepath.Join(t.TempDir(), "c.yaml"), StatePath: filepath.Join(t.TempDir(), "s.db")}, ui.WithMode(false))
	press(f, "enter") // empty URL
	if f.conn.Check != "fail" || !strings.Contains(stripANSI(f.View()), "full URL") {
		t.Fatalf("an empty URL must be refused:\n%s", stripANSI(f.View()))
	}
	press(f, "https://core.example.com", "enter")
	if f.conn.Focus != 1 {
		t.Fatal("a valid URL moves to the token")
	}
	press(f, "enter") // empty token
	if f.conn.Check != "fail" {
		t.Fatal("an empty token must be refused before any network call")
	}
	press(f, "orn_enr_x")
	if got := stripANSI(f.View()); strings.Contains(got, "orn_enr_x") || !strings.Contains(got, "•") {
		t.Fatalf("token must be masked:\n%s", got)
	}
	f.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if !strings.Contains(stripANSI(f.View()), "orn_enr_x") {
		t.Fatal("ctrl+r shows the token")
	}
}

func TestFlowChooseReviewApplyWritesOnlyTickedMonitors(t *testing.T) {
	f, cfg := newTestFlow(t)
	view := stripANSI(f.View())
	if !strings.Contains(view, "nginx") || !strings.Contains(view, "selected") {
		t.Fatalf("choose screen:\n%s", view)
	}
	// untick whatever the cursor is on (nginx), keep the rest
	press(f, " ", "down", " ") // untick nginx, tick myapp
	press(f, "enter")
	if f.step != flowReview {
		t.Fatalf("step = %v, want review (note %q)", f.step, f.note)
	}
	if strings.Contains(strings.Join(f.review.Rows, "\n"), "svc-nginx") || !strings.Contains(strings.Join(f.review.Rows, "\n"), "svc-myapp") {
		t.Fatalf("review rows: %v", f.review.Rows)
	}
	_, cmd := f.Update(key("enter"))
	if f.step != flowApplying || cmd == nil {
		t.Fatalf("apply must start committing, step=%v", f.step)
	}
	f.Update(cmd())
	loaded, _, err := config.Load(cfg)
	if err != nil || len(loaded.Monitors) != 1 || loaded.Monitors[0].Name != "svc-myapp" {
		t.Fatalf("config = %+v err=%v", loaded, err)
	}
	if f.step != flowApplied && !f.done {
		t.Fatalf("flow did not finish: step=%v", f.step)
	}
}

func TestFlowCancelOnReviewChangesNothing(t *testing.T) {
	f, cfg := newTestFlow(t)
	press(f, "enter", "right")
	_, cmd := f.Update(key("enter"))
	if cmd == nil || f.err == nil {
		t.Fatal("cancel must end the flow with an error")
	}
	if _, err := os.Stat(cfg); err == nil {
		t.Fatal("cancel wrote the config")
	}
}

func TestFlowNeverSuggestSavesToConfigAndRestoreBringsItBack(t *testing.T) {
	f, cfg := newTestFlow(t)
	first := f.selectableItems()[0].ID
	press(f, "x")
	data, _ := os.ReadFile(cfg)
	if !strings.Contains(string(data), first) {
		t.Fatalf("ignore not saved to discovery.ignore:\n%s", data)
	}
	for _, it := range f.selectableItems() {
		if it.ID == first {
			t.Fatal("an ignored suggestion must leave the list")
		}
	}
	press(f, "s")
	if f.step != flowSettings || len(f.settings.Never) != 1 {
		t.Fatalf("settings must list the ignored id: %+v", f.settings)
	}
	press(f, "down", "down", "down", "down", "down", "down", "down", "r")
	data, _ = os.ReadFile(cfg)
	if strings.Contains(string(data), first) {
		t.Fatalf("restore must remove it from discovery.ignore:\n%s", data)
	}
}

func TestFlowScanSettingsToggleKindWritesConfig(t *testing.T) {
	f, cfg := newTestFlow(t)
	press(f, "s", "down", "down", " ") // cursor 2 = containers
	disc := readDiscoveryConfig(cfg)
	if disc.KindEnabled("containers") || !disc.KindEnabled("services") {
		t.Fatalf("discovery = %+v", disc)
	}
	_, cmd := f.Update(key("esc"))
	if f.step != flowScanning || cmd == nil {
		t.Fatalf("leaving changed settings must rescan, step=%v", f.step)
	}
}

func TestFlowCtrlCDuringApplyIsIgnoredAndAfterApplyIsSuccess(t *testing.T) {
	f, _ := newTestFlow(t)
	f.step = flowApplying
	if _, cmd := f.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd != nil || f.err != nil {
		t.Fatal("ctrl+c while applying must be ignored")
	}
	f.step = flowApplied
	if _, cmd := f.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil || f.err != nil || !f.done {
		t.Fatalf("ctrl+c after applying is a normal exit, err=%v", f.err)
	}
}

func TestFlowCtrlCEarlierCancelsWithoutClaimingMoreThanItDid(t *testing.T) {
	f, _ := newTestFlow(t)
	f.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if f.err == nil || !strings.Contains(f.err.Error(), "setup cancelled") {
		t.Fatalf("err = %v", f.err)
	}
	g, _ := newTestFlow(t)
	press(g, "s", " ", "esc") // save a scan setting, then cancel
	g.step = flowChoose
	g.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if ce, ok := g.err.(*CommandError); !ok || !strings.Contains(ce.Cause.Error(), "scan settings you saved stay") {
		t.Fatalf("err = %v", g.err)
	}
}

func TestFlowPollingMarksMonitorsReadyThenFinishes(t *testing.T) {
	f, _ := newTestFlow(t)
	f.shown = []appliedMonitor{{Name: "svc-nginx"}, {Name: "disk"}}
	f.step = flowApplied
	f.onPolled(polledMsg{st: control.Status{MonitorStates: []control.MonitorState{{Name: "svc-nginx", Since: time.Now().Add(-4 * time.Second)}}}})
	if !f.shown[0].Ready || f.shown[1].Ready || f.done {
		t.Fatalf("shown = %+v done=%v", f.shown, f.done)
	}
	_, cmd := f.onPolled(polledMsg{st: control.Status{MonitorStates: []control.MonitorState{{Name: "svc-nginx"}, {Name: "disk"}}}})
	if !f.done || cmd == nil {
		t.Fatal("everything reporting ends the flow")
	}
	g, _ := newTestFlow(t)
	g.shown, g.polls = []appliedMonitor{{Name: "x"}}, 11
	g.onPolled(polledMsg{})
	if !g.done {
		t.Fatal("polling gives up after a bounded time")
	}
}

func TestCommitWorksOnADiscoveryOnlyConfigAndKeepsComments(t *testing.T) {
	f, cfg := newTestFlow(t)
	os.WriteFile(cfg, []byte("# mine\ndiscovery:\n  enabled: true # keep\n"), 0o640)
	if _, err := editDiscoveryKind(cfg, "ports", false, discover.AllKinds); err != nil {
		t.Fatal(err)
	}
	if _, err := editDiscoveryEnabled(cfg, false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(cfg)
	if !strings.Contains(string(data), "# mine") || !strings.Contains(string(data), "# keep") {
		t.Fatalf("comments lost:\n%s", data)
	}
	f.plan.Selected = nil
	if _, err := commitSetup(context.Background(), f.opts, f.plan); err != nil {
		t.Fatalf("commit on a discovery-only file: %v", err)
	}
	if got := readDiscoveryConfig(cfg); got.IsEnabled() || got.KindEnabled("ports") {
		t.Fatalf("settings lost after commit: %+v", got)
	}
}

func TestRunFlowRemovesStagedTokenWhenTheProgramIsCancelled(t *testing.T) {
	f, _ := newTestFlow(t)
	tok, err := stageEnrollmentToken("orn_enr_x")
	if err != nil {
		t.Fatal(err)
	}
	f.plan.TokenFile, f.plan.RemoveTokenFile = tok, true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	prev := outputWriter
	outputWriter = &strings.Builder{}
	defer func() { outputWriter = prev }()
	_ = runFlowWith(ctx, f)
	if _, err := os.Stat(tok); !os.IsNotExist(err) {
		t.Fatalf("staged token still on disk: %v", err)
	}
}
