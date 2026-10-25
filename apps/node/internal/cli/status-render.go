package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"orion/node/internal/control"
	"orion/node/internal/doctor"
	"orion/node/internal/ui"
)

// statusInput is everything the status screen needs. Stopped means the daemon
// could not be reached (Status then only has the name and why).
type statusInput struct {
	St      control.Status
	Now     time.Time
	Width   int
	Stopped bool
	Why     string // "the service is stopped" / "the service is not installed"
	Hint    string // the command that helps when stopped
}

type statusKind int

const (
	stConnected statusKind = iota
	stNotEnrolled
	stWaiting
	stNoRecent
	stMaintenance
	stNotRunning
)

// staleAfter is how long without a report counts as "no recent report".
func staleAfter(st control.Status) time.Duration {
	d := time.Duration(st.ReportIntervalSeconds) * time.Second * 3
	if d < 2*time.Minute {
		d = 2 * time.Minute
	}
	return d
}

func statusKindOf(in statusInput) statusKind {
	st := in.St
	switch {
	case in.Stopped:
		return stNotRunning
	case !st.Registered:
		return stNotEnrolled
	case st.MaintenanceMode:
		return stMaintenance
	case st.LastReportAt.IsZero():
		return stWaiting
	case in.Now.Sub(st.LastReportAt) > staleAfter(st):
		return stNoRecent
	}
	return stConnected
}

type badgeSpec struct{ tone, glyph, text, plain string }

var badges = map[statusKind]badgeSpec{
	stConnected:   {"up", "●", "Connected", "[up] connected"},
	stNotEnrolled: {"unknown", "○", "Not enrolled", "[?] not enrolled"},
	stWaiting:     {"unknown", "○", "Waiting", "[?] waiting for the first report"},
	stNoRecent:    {"degraded", "◐", "No recent report", "[slow] no recent report"},
	stMaintenance: {"maintenance", "‖", "Maintenance", "[maint] maintenance"},
	stNotRunning:  {"unknown", "○", "Not running", "[down] not running"},
}

func next(th *ui.Theme, cmd, why string) string {
	if th.Plain() {
		return "next: " + cmd
	}
	line := "   " + th.Tone("up", "▸") + " " + th.Bold(cmd)
	if why != "" {
		line += "  " + th.Muted(why)
	}
	return line
}

// statusScreen renders the cli-status board for every state, or the plain
// text layout when colour is off.
func statusScreen(th *ui.Theme, in statusInput) string {
	if th.Plain() {
		return statusPlain(in)
	}
	kind := statusKindOf(in)
	b := badges[kind]
	name := in.St.Name
	if name == "" {
		name = "this host"
	}
	var out []string
	out = append(out, "")
	out = append(out, headerLine(th, name, th.Badge(b.tone, b.glyph+" "+b.text)))
	switch kind {
	case stNotEnrolled:
		out = append(out, next(th, "orion-node setup", "connect this host to a Core"))
	case stWaiting:
		out = append(out, "   "+th.Muted(fmt.Sprintf("enrolled; first report is due within %d s", max(in.St.ReportIntervalSeconds, 30))))
	case stNoRecent:
		out = append(out, "   "+th.Muted(fmt.Sprintf("last report %s, %s ago · queue %d pending", in.St.LastReportAt.Local().Format("15:04:05"), doctor.Duration(in.Now.Sub(in.St.LastReportAt)), in.St.QueuePending)))
		if in.St.Suggestions > 0 {
			out = append(out, "   "+th.Muted(fmt.Sprintf("node offline since %s · %d %s kept", in.St.LastReportAt.Local().Format("15:04"), in.St.Suggestions, plural(in.St.Suggestions, "suggestion", "suggestions"))))
			out = append(out, "   "+th.Muted("they apply when the node reconnects"))
		}
		out = append(out, next(th, "orion-node doctor", "find out why reports are not arriving"))
	case stMaintenance:
		out = append(out, "   "+th.Muted(maintenanceLine(in)))
		out = append(out, next(th, "orion-node maintenance up", "end it early"))
	case stNotRunning:
		out = append(out, "   "+th.Muted(in.Why))
		out = append(out, next(th, in.Hint, ""))
	default:
		out = append(out, connectedBody(th, in)...)
	}
	return strings.Join(out, "\n") + "\n"
}

func maintenanceLine(in statusInput) string {
	line := in.St.MaintenanceReason
	if line == "" {
		line = "maintenance"
	}
	if u := in.St.MaintenanceUntil; u != nil {
		left := u.Sub(in.Now)
		line += fmt.Sprintf(" · alerts muted until %s (%s)", u.Local().Format("15:04"), humanLeft(left))
	}
	return line
}

func humanLeft(d time.Duration) string {
	if d < time.Minute {
		return "under a minute"
	}
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h > 0 {
		return fmt.Sprintf("%d h %d m", h, m)
	}
	return fmt.Sprintf("%d m", m)
}

const statusWidth = 86

// headerLine is "◆ name" with the badge right-aligned so it ends two columns
// inside the panel edge, as on the board.
func headerLine(th *ui.Theme, name, badge string) string {
	left := " " + th.Tone("up", "◆") + " " + th.Bold(name)
	gap := statusWidth - visibleWidth(left) - visibleWidth(badge)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + badge
}

func connectedBody(th *ui.Theme, in statusInput) []string {
	st := in.St
	var out []string
	core := strings.TrimPrefix(strings.TrimPrefix(st.CoreURL, "https://"), "http://")
	sub := fmt.Sprintf("%s · v%s · up %s · %s", core, strings.TrimPrefix(st.Version, "v"), doctor.Duration(time.Duration(st.UptimeSeconds)*time.Second), modeWord(st.Mode))
	out = append(out, "   "+th.Muted(sub), "")
	if st.Host != nil {
		out = append(out, indent1(th.Panel("Host", "always on", hostRows(th, st.Host), panelWidth)))
	}
	if st.Scanning {
		out = append(out, "   "+th.Tone("degraded", "⠹")+" Looking at this server…   "+th.Muted("a few seconds, read only"))
	}
	if len(st.MonitorStates) > 0 {
		out = append(out, indent1(monitorsPanel(th, st, in.Now)))
	} else if st.Monitors == 0 {
		line := "   " + th.Tone("up", "●") + " Watching 0 monitors"
		if st.ThingsFound > 0 {
			line += fmt.Sprintf("   %d %s found", st.ThingsFound, plural(st.ThingsFound, "thing", "things"))
		}
		out = append(out, line)
	}
	out = append(out, indent1(th.Panel("Reporting", "", []string{reportingRow(th, in)}, panelWidth)))
	if len(st.NewItems) > 0 {
		out = append(out, indent1(newItemsPanel(th, st.NewItems)))
	}
	var steps []string
	switch {
	case len(st.NewItems) > 0:
		steps = append(steps, next(th, "orion-node setup", "review them. Nothing is added silently"))
	case st.Suggestions > 0:
		steps = append(steps, next(th, "orion-node setup", fmt.Sprintf("%d %s waiting", st.Suggestions, plural(st.Suggestions, "suggestion", "suggestions"))))
	}
	if st.MissingLogKeys > 0 {
		steps = append(steps, next(th, "orion-node doctor", fmt.Sprintf("%d log %s missing", st.MissingLogKeys, plural(st.MissingLogKeys, "key is", "keys are"))))
	}
	if len(steps) > 0 {
		out = append(out, "")
		out = append(out, steps...)
	}
	return out
}

// indent1 indents every non-empty line of a panel by one space.
func indent1(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = " " + l
		}
	}
	return strings.Join(lines, "\n")
}

func modeWord(mode string) string {
	if strings.HasPrefix(mode, "system service") {
		return "system service"
	}
	if mode == "" {
		return "service"
	}
	return strings.SplitN(mode, ",", 2)[0]
}

// bar draws a 16-cell meter: filled "━", a head "╸", then "─".
func bar(th *ui.Theme, pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	full := int(pct / 100 * float64(width))
	if full >= width {
		return th.Tone(barTone(pct), strings.Repeat("━", width))
	}
	return th.Tone(barTone(pct), strings.Repeat("━", full)+"╸") + th.Faint(strings.Repeat("─", width-full-1))
}

func barTone(pct float64) string {
	switch {
	case pct >= 90:
		return "down"
	case pct >= 75:
		return "degraded"
	}
	return "up"
}

func hostRows(th *ui.Theme, h *control.HostStat) []string {
	m := func(label string) string { return th.Muted(padRight(label, 9)) }
	r1 := " " + m("cpu") + bar(th, h.CPUPercent, 16) + " " + th.Bold(fmt.Sprintf("%.0f%%", h.CPUPercent)) + "   " +
		m("memory") + bar(th, h.MemoryPercent, 16) + " " + th.Bold(fmt.Sprintf("%.0f%%", h.MemoryPercent)) + "   " + th.Muted(fmt.Sprintf("load %.2f", h.Load1))
	r2 := " " + m("disk "+h.DiskPath) + bar(th, h.DiskPercent, 16) + " " + th.Bold(fmt.Sprintf("%.0f%%", h.DiskPercent)) + "   " +
		m("network") + th.Muted("↓ "+byteRate(h.NetRxBps)+"  ↑ "+byteRate(h.NetTxBps))
	return []string{r1, r2}
}

// byteRate formats bytes per second like "1.2 MB/s".
func byteRate(bps float64) string {
	switch {
	case bps >= 1<<30:
		return fmt.Sprintf("%.1f GB/s", bps/(1<<30))
	case bps >= 1<<20:
		return fmt.Sprintf("%.1f MB/s", bps/(1<<20))
	case bps >= 1<<10:
		return fmt.Sprintf("%.0f KB/s", bps/(1<<10))
	}
	return fmt.Sprintf("%.0f B/s", bps)
}

// valueWarn marks a percentage reading in the warning band (85% and up).
func valueWarn(v string) bool {
	n, err := strconv.Atoi(strings.TrimSuffix(v, "%"))
	return err == nil && n >= 85
}

// monitorsPanel: a proportional health bar, then the table, unhealthy first.
func monitorsPanel(th *ui.Theme, st control.Status, now time.Time) string {
	var up, slow, down int
	for _, m := range st.MonitorStates {
		switch healthStatus(m.Health) {
		case "up":
			up++
		case "degraded":
			slow++
		case "down":
			down++
		}
	}
	var parts []string
	if up > 0 {
		parts = append(parts, th.Tone("up", fmt.Sprintf("%d up", up)))
	}
	if slow > 0 {
		parts = append(parts, th.Tone("degraded", fmt.Sprintf("%d slow", slow)))
	}
	if down > 0 {
		parts = append(parts, th.Tone("down", fmt.Sprintf("%d down", down)))
	}
	right := strings.Join(parts, th.Muted(" · "))
	rows := []string{" " + segmentBar(th, up, slow, down, panelWidth-4)}
	rows = append(rows, th.Muted("     "+padRight("NAME", 16)+padRight("KIND", 13)+padRight("STATUS", 10)+padRight("AGE", 7)+"DETAIL"))
	ms := append([]control.MonitorState(nil), st.MonitorStates...)
	sort.SliceStable(ms, func(i, j int) bool {
		ri, rj := healthRank(ms[i].Health), healthRank(ms[j].Health)
		if ri != rj {
			return ri < rj
		}
		return ms[i].Name < ms[j].Name
	})
	const maxRows = 12
	for i, m := range ms {
		if i == maxRows {
			rows = append(rows, th.Muted(fmt.Sprintf("     … and %d more", len(ms)-maxRows)))
			break
		}
		status, hs := healthWord(m.Health), healthStatus(m.Health)
		if m.Value != "" {
			status = m.Value // a reading is more useful than "up"
			if hs == "up" && valueWarn(m.Value) {
				hs = "degraded"
			}
		}
		detail := m.Detail
		if detail == "" {
			detail = "-"
		}
		since := m.Since
		if since.IsZero() {
			since = m.At
		}
		rows = append(rows, "   "+th.StatusMark(hs)+" "+
			padRight(clip(monitorLabel(m), 15), 16)+th.Muted(padRight(kindLabel(m.Kind), 13))+
			th.Tone(hs, padRight(status, 10))+th.Muted(padRight(ago(now, since), 7))+th.Muted(clip(detail, 34)))
	}
	return th.Panel("Monitors", right, rows, panelWidth)
}

var scanPrefix = regexp.MustCompile(`^(svc|ctr|pm2|port|disk)-`)

// monitorLabel prefers what the monitor watches for generated names.
func monitorLabel(m control.MonitorState) string {
	if m.Target != "" && scanPrefix.MatchString(m.Name) {
		return m.Target
	}
	return m.Name
}

func healthTone(h string) string {
	switch healthStatus(h) {
	case "up":
		return "up"
	case "down":
		return "down"
	case "degraded":
		return "degraded"
	}
	return "unknown"
}

func healthRank(h string) int {
	switch healthStatus(h) {
	case "down":
		return 0
	case "degraded":
		return 1
	case "unknown":
		return 2
	}
	return 3
}

// segmentBar is one proportional bar: green for up, amber for slow, red for down.
func segmentBar(th *ui.Theme, up, slow, down, width int) string {
	total := up + slow + down
	if total == 0 {
		return th.Faint(strings.Repeat("─", width))
	}
	counts := []int{up, slow, down}
	cells := []int{0, 0, 0}
	left := width
	for i, c := range counts {
		if c > 0 {
			cells[i] = max(1, width*c/total)
			left -= cells[i]
		}
	}
	for i := range cells { // give rounding leftovers to the largest group
		if left != 0 && cells[i] > 0 && counts[i] == max(up, max(slow, down)) {
			cells[i] += left
			break
		}
	}
	return th.Tone("up", strings.Repeat("━", cells[0])) + th.Tone("degraded", strings.Repeat("━", cells[1])) + th.Tone("down", strings.Repeat("━", cells[2]))
}

func reportingRow(th *ui.Theme, in statusInput) string {
	st := in.St
	left := th.Muted("last report") + "  " + st.LastReportAt.Local().Format("15:04:05") + th.Muted(" ("+ago(in.Now, st.LastReportAt)+" ago)")
	every := fmt.Sprintf("every %d s · queue %d", st.ReportIntervalSeconds, st.QueuePending)
	return " " + padRight(left, 43) + th.Tone("up", padRight(spark(st.ReportMillis), 15)) + th.Muted(padRight(every, 24))
}

// spark draws values as ▁..█, scaled to the largest.
func spark(vals []int) string {
	if len(vals) == 0 {
		return ""
	}
	maxV := 1
	for _, v := range vals {
		if v > maxV {
			maxV = v
		}
	}
	levels := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	for _, v := range vals {
		b.WriteRune(levels[min(len(levels)-1, v*(len(levels)-1)/maxV)])
	}
	return b.String()
}

func newItemsPanel(th *ui.Theme, items []control.DiscoveryItem) string {
	var rows []string
	for _, it := range items {
		rows = append(rows, " "+th.Tone("degraded", "!")+" "+padRight(clip(it.ID, 29), 30)+th.Muted(newItemDetail(it)))
	}
	return th.Panel("New on this server", fmt.Sprintf("%d new", len(items)), rows, panelWidth)
}

func newItemDetail(it control.DiscoveryItem) string {
	switch it.Kind {
	case "container":
		return "container · started " + it.FirstSeen.Local().Format("15:04")
	case "port":
		if rest, ok := strings.CutPrefix(it.Detail, "listening, "); ok {
			return rest + ", listening"
		}
	case "service":
		return "service · appeared " + it.FirstSeen.Local().Format("15:04")
	}
	return it.Detail
}
