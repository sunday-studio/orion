package cli

import (
	"fmt"
	"path"
	"strings"

	"orion/node/internal/control"
	"orion/node/internal/discover"
	"orion/node/internal/ui"
)

var kindTitles = map[string]string{
	discover.ConfigServices: "Services", discover.ConfigContainers: "Containers", discover.ConfigPM2: "Apps (pm2)",
	discover.ConfigPorts: "Ports", discover.ConfigDisks: "Disks", discover.ConfigLogs: "Logs", discover.ConfigStorage: "Storage",
}

// kindNames are the lowercase words used inside sentences.
var kindNames = map[string]string{
	discover.ConfigServices: "Services", discover.ConfigContainers: "containers", discover.ConfigPM2: "pm2 apps",
	discover.ConfigPorts: "ports", discover.ConfigDisks: "disks", discover.ConfigLogs: "log files", discover.ConfigStorage: "storage",
}

func actionFor(kind string) string {
	switch kind {
	case "port":
		return "Check it accepts connections"
	case "disk":
		return "Alert above 90% full"
	case "log":
		return "Ship its lines"
	case "storage":
		return "Track the space it uses"
	}
	return "Check it is running"
}

// preTicked: high-confidence suggestions nobody dismissed are ticked by default.
func preTicked(it control.DiscoveryItem) bool {
	return it.Status == discover.StatusSuggested && !it.Dismissed && it.Confidence == "high" && it.Kind != "log"
}

func discoverCounts(d control.Discovery) (things, fresh, ticked int) {
	for _, it := range d.Items {
		if it.Dismissed {
			continue
		}
		things++
		if it.New {
			fresh++
		}
		if preTicked(it) {
			ticked++
		}
	}
	return
}

func confTone(c string) string {
	switch c {
	case "high":
		return "up"
	case "medium":
		return "degraded"
	}
	return "unknown"
}

func displayName(it control.DiscoveryItem) string {
	switch it.Kind {
	case "port":
		if rest, ok := strings.CutPrefix(it.Detail, "listening, "); ok && rest != "unknown process" {
			return it.Name + " " + rest
		}
	case "log":
		dir, file := path.Split(it.Name)
		return path.Base(dir) + " " + file
	}
	return it.Name
}

// Column widths inside a panel: lead 2, mark+space, name 18, action 32, note 20, confidence 10 = 84.
const (
	colName   = 18
	colAction = 32
	colNote   = 20
	colConf   = 10
)

// discoverRow is one suggestion line inside a panel, laid out like the board.
func discoverRow(th *ui.Theme, it control.DiscoveryItem) string {
	return suggestionRow(th, it, preTicked(it), false)
}

// suggestionRow is discoverRow with the tick and the cursor decided by the
// caller, which is what the setup checklist needs.
func suggestionRow(th *ui.Theme, it control.DiscoveryItem, tick, cursor bool) string {
	name := padRight(clip(displayName(it), colName-1), colName)
	if it.Status == discover.StatusMonitored {
		by := map[string]string{"file": "managed in file", "scan": "accepted from scan", "console": "managed in Console"}[it.ManagedIn]
		if th.Plain() {
			return "[=] " + name + "already monitored (" + by + ")"
		}
		return "  " + th.Faint("✓ "+name+padRight("already monitored", colAction)+padRight(by, colNote))
	}
	mark := "◯"
	if th.Plain() {
		mark = "[ ]"
		if tick {
			mark = "[x]"
		}
	} else if tick {
		mark = th.Tone("up", "◉")
	} else {
		mark = th.Muted("◯")
	}
	action := actionFor(it.Kind)
	note := ""
	if it.Kind == "log" {
		note = "needs a log key"
	}
	if th.Plain() {
		line := mark + " " + name + padRight(action, colAction)
		if note != "" {
			line += padRight(note, colNote)
		}
		return line + it.Confidence
	}
	lead := "  "
	if cursor {
		lead = " " + th.Accent("›")
	}
	return lead + mark + " " + name + th.Muted(padRight(action, colAction)) + th.Tone("degraded", padRight(note, colNote)) + th.Tone(confTone(it.Confidence), padRight(it.Confidence, colConf))
}

const panelWidth = 86

// discoverScreen renders the cli-discover board: grouped suggestions with a
// confidence label, "already monitored" rows and scan problems. Every line is
// indented one space, as on the board.
func discoverScreen(th *ui.Theme, d control.Discovery, host string, width int) string {
	var out []string
	if !d.Enabled {
		out = append(out, th.Panel("Scanning is off", "", []string{
			" discovery.enabled is false, so nothing is looked at.",
			" Host metrics, manual config and Create monitor still work.", "",
			" " + th.Accent(" orion-node config edit") + "   " + th.Muted("set discovery.enabled: true"),
		}, panelWidth))
		return "\n" + margin(th, out) + "\n"
	}
	things, fresh, ticked := discoverCounts(d)
	if things > 0 {
		summary := fmt.Sprintf("%d %s", things, plural(things, "thing", "things"))
		if fresh > 0 {
			summary += fmt.Sprintf(" · %d new", fresh)
		}
		if th.Plain() {
			out = append(out, "Discover: found on "+host+", "+strings.ReplaceAll(summary, " · ", ", "))
		} else {
			left := th.Accent("◆") + " " + th.Bold("Discover") + th.Muted("  found on "+host)
			gap := panelWidth - visibleWidth(left) - len(summary) // the summary ends one column before the panel edge
			if gap < 2 {
				gap = 2
			}
			out = append(out, left+strings.Repeat(" ", gap)+th.Muted(summary))
		}
	}
	if hasBlocked(d.Kinds) {
		out = append(out, scanPanel(th, d.Kinds, panelWidth))
	}
	shown := 0
	for _, kind := range discover.AllKinds {
		var rows []string
		for _, it := range d.Items {
			if discover.ConfigKind(discover.Kind(it.Kind)) == kind && !it.Dismissed {
				rows = append(rows, discoverRow(th, it))
			}
		}
		switch {
		case kindState(d.Kinds, kind) == discover.ScanOff:
			out = append(out, th.Panel(kindTitles[kind], "", []string{th.Muted(" Not scanned · turned off in discovery.kinds")}, panelWidth))
		case len(rows) > 0:
			out = append(out, th.Panel(kindTitles[kind], fmt.Sprint(len(rows)), rows, panelWidth))
			shown += len(rows)
		}
	}
	if shown == 0 && !hasBlocked(d.Kinds) {
		out = append(out, th.Panel("Nothing else detected", "", []string{
			" Host metrics are on. Nothing new to suggest.", "",
			" " + th.Accent(" orion-node discover --rescan"),
		}, panelWidth))
	}
	s := "\n" + margin(th, out) + "\n"
	if ticked > 0 {
		s += "\n " + th.Muted(fmt.Sprintf("%d pre-ticked · apply them with ", ticked)) + th.Bold("orion-node setup") + "\n"
	}
	return s
}

// margin indents every line of the blocks by one space (plain mode has no box
// and no margin).
func margin(th *ui.Theme, blocks []string) string {
	text := strings.Join(blocks, "\n")
	if th.Plain() {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = " " + l
		}
	}
	return strings.Join(lines, "\n")
}

func kindState(ks []control.KindState, kind string) string {
	for _, k := range ks {
		if k.Kind == kind {
			return k.State
		}
	}
	return ""
}

func hasBlocked(ks []control.KindState) bool {
	for _, k := range ks {
		if k.State == discover.ScanBlocked || k.State == discover.ScanFailed {
			return true
		}
	}
	return false
}

// scanPanel renders the "Scan" box: consecutive scanned kinds are one line,
// a blocked kind gets its message, the exact fix and the trade-off.
func scanPanel(th *ui.Theme, ks []control.KindState, width int) string {
	var rows []string
	var group []string
	blocked := 0
	flush := func() {
		if len(group) > 0 {
			label := strings.Join(group, ", ")
			rows = append(rows, " "+th.Pass()+" "+strings.ToUpper(label[:1])+label[1:])
			group = nil
		}
	}
	for _, kind := range discover.AllKinds {
		var k *control.KindState
		for i := range ks {
			if ks[i].Kind == kind {
				k = &ks[i]
			}
		}
		if k == nil || k.State == discover.ScanOff || k.State == discover.ScanNone {
			continue
		}
		if k.State == discover.ScanOK {
			group = append(group, kindNames[kind])
			continue
		}
		flush()
		blocked++
		status := "not fully scanned"
		if k.State == discover.ScanFailed {
			status = "scan failed"
		}
		rows = append(rows, " "+th.WarnMark()+" "+padRight(capitalize(kindNames[kind]), 28)+th.Muted(status))
		bar := " " + strings.Repeat(" ", 5) + th.Faint("│") + " "
		rows = append(rows, bar+k.Message)
		if k.Fix != "" {
			rows = append(rows, bar+th.Muted("Fix")+"  "+k.Fix)
		}
		if k.Why != "" {
			rows = append(rows, bar+k.Why)
		}
	}
	flush()
	right := "ok"
	if blocked > 0 {
		right = fmt.Sprintf("%d !", blocked)
	}
	return th.Panel("Scan", right, rows, width)
}

// dismissedScreen lists dismissed suggestions with when and how to restore.
func dismissedScreen(th *ui.Theme, d control.Discovery, width int) string {
	rows := []string{th.Muted("   " + padRight("ID", 25) + "WHEN")}
	n := 0
	for _, it := range d.Items {
		if !it.Dismissed {
			continue
		}
		n++
		when := "-"
		if !it.DismissedAt.IsZero() {
			when = it.DismissedAt.Local().Format("2 Jan")
		}
		rows = append(rows, "   "+padRight(clip(it.ID, 24), 25)+padRight(when, 31)+th.Accent("restore"))
	}
	if n == 0 {
		return "\n" + margin(th, []string{th.Panel("Dismissed", "0", []string{th.Muted(" Nothing dismissed.")}, panelWidth)}) + "\n"
	}
	return "\n" + margin(th, []string{th.Panel("Dismissed", fmt.Sprint(n), rows, panelWidth)}) + "\n"
}
