package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"orion/node/internal/control"
)

// --- plain layout: one fact per line, no colour, box or glyph ----------------

func statusPlain(in statusInput) string {
	st := in.St
	kind := statusKindOf(in)
	b := badges[kind]
	name := st.Name
	if name == "" {
		name = "this host"
	}
	var out []string
	head := name + " " + b.plain
	if kind == stConnected && st.CoreURL != "" {
		head += " to " + strings.TrimPrefix(strings.TrimPrefix(st.CoreURL, "https://"), "http://")
	}
	out = append(out, head)
	switch kind {
	case stNotEnrolled:
		out = append(out, "next: orion-node setup")
	case stWaiting:
		out = append(out, fmt.Sprintf("first report is due within %d s", max(st.ReportIntervalSeconds, 30)))
	case stNoRecent:
		out = append(out, fmt.Sprintf("last report %s (%s ago), queue %d pending", st.LastReportAt.Local().Format("15:04:05"), ago(in.Now, st.LastReportAt), st.QueuePending), "next: orion-node doctor")
	case stMaintenance:
		out = append(out, maintenanceLine(in), "next: orion-node maintenance up")
	case stNotRunning:
		out = append(out, in.Why, "next: "+in.Hint)
	default:
		out = append(out, fmt.Sprintf("last report %s (%s ago), queue %d pending", st.LastReportAt.Local().Format("15:04:05"), ago(in.Now, st.LastReportAt), st.QueuePending))
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
		total := up + slow + down
		if total > 0 {
			out = append(out, fmt.Sprintf("monitors %d: %d up, %d slow, %d down", total, up, slow, down))
		}
		ms := append([]control.MonitorState(nil), st.MonitorStates...)
		sort.SliceStable(ms, func(i, j int) bool { return healthRank(ms[i].Health) < healthRank(ms[j].Health) })
		for _, m := range ms {
			if healthStatus(m.Health) == "up" {
				continue
			}
			since := m.Since
			if since.IsZero() {
				since = m.At
			}
			detail := m.Detail
			if detail == "" {
				detail = healthWord(m.Health)
			}
			out = append(out, fmt.Sprintf("  %-7s %-10s %s %s, %s", "["+healthWord(m.Health)+"]", clip(monitorLabel(m), 10), kindLabel(m.Kind), detail, ago(in.Now, since)))
		}
		if len(st.NewItems) > 0 {
			out = append(out, fmt.Sprintf("%d new on this server: orion-node discover", len(st.NewItems)))
		}
		switch {
		case st.Suggestions > 0:
			out = append(out, "next: orion-node setup ("+fmt.Sprintf("%d %s waiting", st.Suggestions, plural(st.Suggestions, "suggestion", "suggestions"))+")")
		case st.MissingLogKeys > 0:
			out = append(out, "next: orion-node doctor")
		case down+slow > 0:
			out = append(out, "next: orion-node doctor")
		}
	}
	return strings.Join(out, "\n") + "\n"
}

// statusShort is the one-line form for prompts and cron.
func statusShort(in statusInput) string {
	st := in.St
	name := st.Name
	if name == "" {
		name = "this host"
	}
	k := statusKindOf(in)
	if k != stConnected {
		return name + " " + strings.ToLower(badges[k].text) + "\n"
	}
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
	return fmt.Sprintf("%s connected, %d up %d slow %d down, last report %s ago\n", name, up, slow, down, ago(in.Now, st.LastReportAt))
}

func padRight(s string, w int) string {
	if n := visibleWidth(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

func kindLabel(monitorType string) string {
	switch monitorType {
	case "systemd-service":
		return "systemd"
	case "docker-container":
		return "container"
	case "disk-usage":
		return "disk"
	case "tcp":
		return "port"
	case "process-running":
		return "process"
	case "resource-threshold":
		return "resource"
	case "http-healthcheck", "internal-service", "website":
		return "http"
	}
	return monitorType
}

func healthStatus(h string) string {
	switch h {
	case "up":
		return "up"
	case "down":
		return "down"
	case "degraded", "slow":
		return "degraded"
	}
	return "unknown"
}

func healthWord(h string) string {
	switch healthStatus(h) {
	case "up":
		return "up"
	case "down":
		return "down"
	case "degraded":
		return "slow"
	}
	return "?"
}

func ago(now, at time.Time) string {
	if at.IsZero() {
		return "-"
	}
	d := now.Sub(at)
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%d s", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d m", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d h", int(d.Hours()))
	}
	return fmt.Sprintf("%d d", int(d.Hours()/24))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
