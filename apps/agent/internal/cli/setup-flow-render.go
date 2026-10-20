package cli

import (
	"fmt"
	"strings"
	"time"

	"orion/agent/internal/control"
	"orion/agent/internal/discover"
	"orion/agent/internal/doctor"
	"orion/agent/internal/ui"
)

// Step states of the setup header: done, done because the host was already
// enrolled, the step in progress, and the steps still to come.
const (
	stepDone = "done"
	stepSkip = "skip"
	stepNow  = "now"
	stepTodo = "todo"
)

var setupStepNames = [3]string{"Connect", "Choose", "Review"}

// setupHeader is the title line with the three-step progress, right aligned.
func setupHeader(th *ui.Theme, steps [3]string) string {
	if th.Plain() {
		var parts []string
		for i, s := range steps {
			parts = append(parts, fmt.Sprintf("%d %s %s", i+1, strings.ToLower(setupStepNames[i]), map[string]string{stepDone: "done", stepSkip: "done", stepNow: "now", stepTodo: "todo"}[s]))
		}
		return "Set up this host: " + strings.Join(parts, ", ")
	}
	var right []string
	for i, s := range steps {
		name := setupStepNames[i]
		switch s {
		case stepDone:
			right = append(right, th.Tone("up", "✓")+" "+name)
		case stepSkip:
			right = append(right, th.Muted("– "+name+" (done)"))
		case stepNow:
			right = append(right, th.Accent("●")+" "+th.Bold(name))
		default:
			right = append(right, th.Muted("○ "+name))
		}
	}
	r := strings.Join(right, "  ")
	left := " " + th.Accent("◆") + " " + th.Bold("Set up this host")
	return left + strings.Repeat(" ", max(2, panelWidth-visibleWidth(left)-visibleWidth(r))) + r
}

// setupKeys renders the key hints at the foot of a screen.
func setupKeys(th *ui.Theme, pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, th.Bold(pairs[i])+"  "+th.Muted(pairs[i+1]))
	}
	return "  " + strings.Join(parts, "   ")
}

// connectView is the state of step one.
type connectView struct {
	URL       string
	Token     string
	ShowToken bool
	Focus     int    // 0 core url, 1 token
	Check     string // "", "checking", "ok", "fail"
	Latency   time.Duration
	Version   string
	Err       string
	Frame     int
}

func connectScreen(th *ui.Theme, v connectView) string {
	steps := [3]string{stepNow, stepTodo, stepTodo}
	if th.Plain() {
		return strings.Join([]string{setupHeader(th, steps), "Core URL: " + v.URL, "Add server token: " + strings.Repeat("*", len([]rune(v.Token)))}, "\n") + "\n"
	}
	bar := func(focused bool, rows ...string) []string {
		out := make([]string, len(rows))
		for i, r := range rows {
			if focused {
				out[i] = " " + th.Accent("┃") + " " + r
			} else {
				out[i] = " " + r
			}
		}
		return out
	}
	prompt := th.Accent("›")
	urlRows := []string{th.Muted("The address you open the Console at."), prompt + " " + v.URL}
	switch v.Check {
	case "checking":
		urlRows = append(urlRows, th.Tone("degraded", th.Spinner(v.Frame))+" "+th.Muted("checking Core and the token…"))
	case "ok":
		line := "reachable"
		if v.Version != "" {
			line += " · Core " + v.Version
		}
		line += fmt.Sprintf(" · %d ms", v.Latency.Milliseconds())
		urlRows = append(urlRows, th.Pass()+" "+th.Muted(line))
	case "fail":
		urlRows = append(urlRows, th.FailMark()+" "+v.Err)
	}
	token := strings.Repeat("•", len([]rune(v.Token)))
	if v.ShowToken {
		token = v.Token
	}
	tokenRows := []string{prompt + " " + token, th.Muted("Servers › Add server in the Console creates one.")}
	out := []string{
		setupHeader(th, steps), "",
		th.Panel("Where is your Core?", "step 1 of 3", bar(v.Focus == 0, urlRows...), panelWidth),
		th.Panel("One-time token", "", bar(v.Focus == 1, tokenRows...), panelWidth),
		"", setupKeys(th, "enter", "next", "ctrl+r", "show token", "esc", "quit"),
	}
	return margin(th, out) + "\n"
}

// chooseView is the checklist of step two.
type chooseView struct {
	Items    []control.DiscoveryItem
	Ticked   map[string]bool
	Cursor   int // index among the selectable items, -1 for none
	Enrolled bool
	Note     string // a short message under the summary, e.g. why a choice failed
}

// selectable: monitored items are shown but cannot be picked.
func selectable(it control.DiscoveryItem) bool { return it.Status != discover.StatusMonitored }

func chooseScreen(th *ui.Theme, v chooseView) string {
	connect := stepDone
	if v.Enrolled {
		connect = stepSkip
	}
	out := []string{setupHeader(th, [3]string{connect, stepNow, stepTodo}), ""}
	sel, picked, monitored := 0, 0, 0
	for _, kind := range discover.AllKinds {
		var rows []string
		for _, it := range v.Items {
			if discover.ConfigKind(discover.Kind(it.Kind)) != kind || it.Dismissed {
				continue
			}
			if !selectable(it) {
				monitored++
				rows = append(rows, suggestionRow(th, it, false, false))
				continue
			}
			rows = append(rows, suggestionRow(th, it, v.Ticked[it.ID], sel == v.Cursor))
			if v.Ticked[it.ID] {
				picked++
			}
			sel++
		}
		if len(rows) > 0 {
			out = append(out, th.Panel(kindTitles[kind], fmt.Sprint(len(rows)), rows, panelWidth))
		}
	}
	if th.Plain() {
		return plainChoose(v, picked, sel, monitored)
	}
	if sel+monitored == 0 {
		out = append(out, th.Panel("Nothing to suggest", "", []string{" Host metrics are on. Nothing found that needs a monitor."}, panelWidth))
	}
	summary := fmt.Sprintf("%d of %d selected", picked, sel)
	if monitored > 0 {
		summary += fmt.Sprintf(" · %d already monitored", monitored)
	}
	summary += " · high confidence is pre-ticked"
	out = append(out, "", " "+th.Muted(summary))
	if v.Note != "" {
		out = append(out, " "+th.Tone("degraded", v.Note))
	}
	out = append(out, "", setupKeys(th, "space", "toggle", "a", "all high", "x", "never suggest", "s", "scan settings", "enter", "next"))
	return margin(th, out) + "\n"
}

func plainChoose(v chooseView, picked, total, monitored int) string {
	lines := []string{"Set up this host: choose what to monitor"}
	for _, it := range v.Items {
		if it.Dismissed {
			continue
		}
		switch {
		case !selectable(it):
			lines = append(lines, "[=] "+displayName(it)+" already monitored")
		case v.Ticked[it.ID]:
			lines = append(lines, "[x] "+displayName(it)+" "+it.Confidence)
		default:
			lines = append(lines, "[ ] "+displayName(it)+" "+it.Confidence)
		}
	}
	lines = append(lines, fmt.Sprintf("%d of %d selected, %d already monitored", picked, total, monitored))
	return strings.Join(lines, "\n") + "\n"
}

// scanSettingsView edits discovery.enabled, discovery.kinds and the ignore list.
type scanSettingsView struct {
	Enabled  bool
	Kinds    map[string]bool // config kind -> scanned
	Never    []neverItem
	Enrolled bool
	Cursor   int // 0 scanning, 1..6 kinds, then the never-suggest rows; -1 none
	Note     string
}

type neverItem struct{ ID, When string }

var scanKindLabels = [][2]string{
	{discover.ConfigServices, "services"}, {discover.ConfigContainers, "containers"}, {discover.ConfigPM2, "pm2 apps"},
	{discover.ConfigPorts, "ports"}, {discover.ConfigDisks, "disks"}, {discover.ConfigLogs, "log files"},
}

func scanSettingsScreen(th *ui.Theme, v scanSettingsView) string {
	if th.Plain() {
		return "Scan settings: discovery.enabled=" + fmt.Sprint(v.Enabled) + "\n"
	}
	connect := stepDone
	if v.Enrolled {
		connect = stepSkip
	}
	at := func(i int, row string) string { // the cursor replaces the row's first space
		if i == v.Cursor && strings.HasPrefix(row, " ") {
			return th.Accent("›") + row[1:]
		}
		return row
	}
	on, off := th.Muted("on"), th.Muted("off")
	if v.Enabled {
		on = th.Accent(th.Bold("on"))
	} else {
		off = th.Accent(th.Bold("off"))
	}
	var kindRows []string
	for i := 0; i < len(scanKindLabels); i += 3 {
		cells := ""
		for j, kl := range scanKindLabels[i : i+3] {
			mark := th.Muted("◯")
			if v.Kinds[kl[0]] {
				mark = th.Tone("up", "◉")
			}
			cells += mark + " " + padRight(kl[1], []int{12, 14, 10}[j])
			if v.Cursor == i+j+1 {
				cells = strings.TrimSuffix(cells, padRight(kl[1], []int{12, 14, 10}[j])) + th.Accent(padRight(kl[1], []int{12, 14, 10}[j]))
			}
		}
		kindRows = append(kindRows, " "+strings.TrimRight(cells, " "))
	}
	never := []string{th.Muted(" Nothing is hidden.")}
	if len(v.Never) > 0 {
		never = nil
		for i, n := range v.Never {
			left := at(len(scanKindLabels)+1+i, " ") + padRight(n.ID, 21) + th.Muted(n.When)
			right := th.Bold("r") + "  " + th.Muted("restore") + " "
			never = append(never, left+strings.Repeat(" ", max(1, panelWidth-2-visibleWidth(left)-visibleWidth(right)))+right)
		}
	}
	out := []string{
		setupHeader(th, [3]string{connect, stepNow, stepTodo}), "",
		th.Panel("Scanning", "discovery.enabled", []string{at(0, " ") + " " + on + "    " + off}, panelWidth),
		th.Panel("Look at", "discovery.kinds", kindRows, panelWidth),
		th.Panel("Never suggest", fmt.Sprint(len(v.Never)), never, panelWidth),
		"", " " + th.Muted("Saved to the config file, so a hand-edited file and this screen agree."),
	}
	if v.Note != "" {
		out = append(out, " "+th.Tone("degraded", v.Note))
	}
	out = append(out, "", setupKeys(th, "space", "toggle", "r", "restore ignored", "esc", "back to the list"))
	return margin(th, out) + "\n"
}

// reviewView is step three: the exact change, what is left out, and the choice.
type reviewView struct {
	Path     string
	Rows     []string // diff rows, "+ ..." / "- ..." / "  ..."
	NotWrite []string
	Apply    bool // the Apply button is the highlighted one
	Enrolled bool
}

func reviewScreen(th *ui.Theme, v reviewView) string {
	if th.Plain() {
		return "Review: " + v.Path + "\n" + strings.Join(v.Rows, "\n") + "\n"
	}
	var rows []string
	added := 0
	for _, l := range v.Rows {
		switch {
		case strings.HasPrefix(l, "+"):
			rows = append(rows, "  "+th.Tone("up", l))
			added++
		case strings.HasPrefix(l, "-"):
			rows = append(rows, "  "+th.Tone("down", l))
		default:
			rows = append(rows, "  "+th.Muted(l))
		}
	}
	right := fmt.Sprintf("+%d lines", added)
	if added == 1 {
		right = "+1 line"
	}
	if len(rows) == 0 {
		rows = []string{th.Muted("  No changes to the config.")}
		right = ""
	}
	connect := stepDone
	if v.Enrolled {
		connect = stepSkip
	}
	out := []string{setupHeader(th, [3]string{connect, stepDone, stepNow}), "", th.Panel(v.Path, right, rows, panelWidth)}
	if len(v.NotWrite) > 0 {
		var nw []string
		for _, l := range v.NotWrite {
			nw = append(nw, " "+l)
		}
		out = append(out, th.Panel("Not written", "", nw, panelWidth))
	}
	apply, cancel := th.Muted(" Apply "), th.Badge("up", "Cancel")
	if v.Apply {
		apply, cancel = th.Badge("up", "Apply"), th.Muted(" Cancel ")
	}
	out = append(out, "", " "+th.Muted("Applies on the agent's next report. No restart."), "", "   "+apply+"   "+cancel, "",
		setupKeys(th, "← →", "choose", "enter", "confirm", "esc", "back"))
	return margin(th, out) + "\n"
}

// appliedMonitor is one new monitor and what the daemon says about it.
type appliedMonitor struct {
	Name  string
	Ready bool
	Since time.Duration
}

type appliedView struct {
	Path     string
	Reloaded bool
	Monitors []appliedMonitor
	Frame    int
}

func appliedScreen(th *ui.Theme, v appliedView) string {
	apply := "no restart"
	if !v.Reloaded {
		apply = "takes effect when the agent next starts"
	}
	if th.Plain() {
		return "Applied: config written to " + v.Path + ", " + apply + "\nnext: orion-agent status\n"
	}
	done := []string{
		" " + th.Pass() + " " + padRight("Config written", 31) + th.Muted(v.Path),
		" " + th.Pass() + " " + padRight("Applied", 31) + th.Muted(apply),
	}
	out := []string{setupHeader(th, [3]string{stepDone, stepDone, stepDone}), "", th.Panel("Applied", "2 of 2", done, panelWidth)}
	if len(v.Monitors) > 0 {
		rows := []string{th.Muted("   NAME            STATE")}
		for _, m := range v.Monitors {
			if m.Ready {
				rows = append(rows, "   "+th.Tone("up", "●")+" "+padRight(clip(m.Name, 15), 16)+"reporting  "+doctor.Duration(m.Since))
			} else {
				rows = append(rows, "   "+th.Tone("degraded", th.Spinner(v.Frame))+" "+padRight(clip(m.Name, 15), 16)+th.Muted("waiting for first check"))
			}
		}
		out = append(out, th.Panel("Monitors", fmt.Sprint(len(v.Monitors)), rows, panelWidth))
	}
	out = append(out, "", th.Panel("", "", []string{" " + th.Accent("▸") + " " + th.Bold("orion-agent status") + "   " + th.Muted("to watch them come alive")}, panelWidth))
	return margin(th, out) + "\n"
}

// setupStep is one line of the non-interactive summary.
type setupStep struct {
	Mark   string // "ok", "warn"
	Name   string
	Detail string
	Took   time.Duration
}

func setupSummary(th *ui.Theme, steps []setupStep, dry bool) string {
	var out []string
	for _, s := range steps {
		if th.Plain() {
			tag := "[ok]  "
			if s.Mark == "warn" {
				tag = "[warn]"
			}
			out = append(out, tag+" "+s.Name+": "+s.Detail)
			continue
		}
		mark := th.Pass()
		if s.Mark == "warn" {
			mark = th.WarnMark()
		}
		left := "   " + mark + " " + padRight(s.Name, 31) + th.Muted(s.Detail)
		if s.Took > 0 {
			t := th.Muted(fmt.Sprintf("%.1f s", s.Took.Seconds()))
			left += strings.Repeat(" ", max(2, panelWidth-visibleWidth(left)-visibleWidth(t))) + t
		}
		out = append(out, left)
	}
	foot := "--dry-run prints the diff and stops. Log sources always need a key."
	if dry {
		foot = "Dry run: nothing was written. Log sources always need a key."
	}
	return "\n" + strings.Join(out, "\n") + "\n\n  " + th.Muted(foot) + "\n"
}
