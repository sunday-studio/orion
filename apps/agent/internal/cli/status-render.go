package cli

import (
	"fmt"
	"strings"
	"time"

	"orion/agent/internal/control"
	"orion/agent/internal/doctor"
	"orion/agent/internal/ui"
)

// kindLabel is the short monitor kind shown in the status table.
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
	return doctor.Duration(d)
}

// statusScreen renders the cli-status artboard from the daemon's own view.
// In plain mode the box and rule use ASCII-safe words and [..] marks.
func statusScreen(th *ui.Theme, st control.Status, now time.Time, width int) string {
	boxW := min(width, 64)
	conn, word := "connected", "up"
	switch {
	case !st.Registered:
		conn, word = "not enrolled", "unknown"
	case st.MaintenanceMode:
		conn, word = "maintenance", "maintenance"
	case st.LastReportAt.IsZero():
		conn, word = "waiting for first report", "unknown"
	case now.Sub(st.LastReportAt) > 5*time.Minute:
		conn, word = "no recent report", "degraded"
	}
	head := th.StatusMark(word) + " " + th.Bold(conn)
	if st.CoreURL != "" {
		head += th.Muted("  to " + st.CoreURL)
	}
	kv := func(k, v string) string { return th.Muted(fmt.Sprintf("%-12s", k)) + v }
	last := "-"
	if !st.LastReportAt.IsZero() {
		last = st.LastReportAt.Local().Format("15:04:05") + th.Muted(" ("+ago(now, st.LastReportAt)+" ago)")
	}
	rows := []string{
		head, "",
		kv("Version", st.Version),
		kv("Uptime", doctor.Duration(time.Duration(st.UptimeSeconds)*time.Second)),
		kv("Last report", last),
		kv("Queue", fmt.Sprintf("%d pending", st.QueuePending)),
		kv("Mode", st.Mode),
	}
	name := st.Name
	if name == "" {
		name = "this host"
	}
	var b strings.Builder
	b.WriteString("\n" + th.Box(name, rows, boxW) + "\n\n")
	b.WriteString(th.Bold("Watching") + th.Muted(fmt.Sprintf("  %d %s, %d log %s", st.Monitors, plural(st.Monitors, "monitor", "monitors"), st.LogSources, plural(st.LogSources, "source", "sources"))) + "\n")
	if len(st.MonitorStates) > 0 {
		b.WriteString("\n" + th.Muted(fmt.Sprintf("  %-15s %-11s %-10s %s", "NAME", "KIND", "STATUS", "LAST")) + "\n")
		b.WriteString("  " + th.Rule(min(width-2, 52)) + "\n")
		for _, m := range st.MonitorStates {
			mark := th.StatusMark(healthStatus(m.Health))
			status := mark + " " + healthWord(m.Health)
			if th.Plain() {
				status = mark // "[slow]" already spells the state
			}
			status = padRight(status, 11)
			b.WriteString(fmt.Sprintf("  %-15s %-11s %s%s\n", clip(m.Name, 15), kindLabel(m.Kind), status, ago(now, m.At)))
		}
	}
	if len(st.TopApps) > 0 {
		b.WriteString("\n" + th.Bold("Using now") + th.Muted("  top apps by CPU") + "\n  ")
		parts := make([]string, 0, len(st.TopApps))
		for _, a := range st.TopApps {
			parts = append(parts, fmt.Sprintf("%s %.1f%%", a.Name, a.CPUPercent))
		}
		b.WriteString(strings.Join(parts, "   ") + "\n")
	}
	return b.String()
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

// statusStopped is shown when the daemon cannot be reached.
func statusStopped(th *ui.Theme, name, state, hint string, width int) string {
	rows := []string{th.StatusMark("unknown") + " " + th.Bold("not running") + th.Muted("  "+state), "", th.Muted("Next        ") + hint}
	if name == "" {
		name = "this host"
	}
	return "\n" + th.Box(name, rows, min(width, 64)) + "\n"
}

func padRight(s string, w int) string {
	if n := visibleWidth(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}
