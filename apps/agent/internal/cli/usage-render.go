package cli

import (
	"fmt"
	"sort"
	"strings"

	"orion/agent/internal/appres"
	"orion/agent/internal/ui"
)

// compactBytes writes sizes the short way: 31 MB, 1.2 MB, 320 KB, 1.4 GB.
func compactBytes(n float64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", n/(1<<30))
	case n >= 10<<20:
		return fmt.Sprintf("%.0f MB", n/(1<<20))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", n/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", n/(1<<10))
	}
	return fmt.Sprintf("%.0f B", n)
}

// usageSorts are the orders the `s` key cycles through.
var usageSorts = []string{"CPU", "memory", "network"}

func netOf(s appres.Series) float64 {
	if s.NetRxBps == nil || s.NetTxBps == nil {
		return -1
	}
	return *s.NetRxBps + *s.NetTxBps
}

func memOf(s appres.Series) float64 {
	if s.MemoryBytes == nil {
		return -1
	}
	return float64(*s.MemoryBytes)
}

// usageRows picks the series worth showing, ordered by the chosen key.
func usageRows(rep appres.Report, sortBy string, top int) []appres.Series {
	var rows []appres.Series
	for _, s := range rep.Series {
		if s.Kind != appres.KindPath {
			rows = append(rows, s)
		}
	}
	key := cpuOf
	switch sortBy {
	case "memory":
		key = memOf
	case "network":
		key = netOf
	}
	sort.SliceStable(rows, func(i, j int) bool { return key(rows[i]) > key(rows[j]) })
	if top > 0 && len(rows) > top {
		rows = rows[:top]
	}
	return rows
}

// usageBar is a bar sized against the largest value in view; an unknown
// value is a dash, never an empty bar.
func usageBar(th *ui.Theme, v, max float64, width int) string {
	if v < 0 || max <= 0 {
		return th.Muted(padRight("-", width))
	}
	filled := int(v / max * float64(width))
	if filled <= 0 {
		return th.Muted(strings.Repeat("─", width))
	}
	if filled >= width {
		return th.Tone("up", strings.Repeat("━", width))
	}
	return th.Tone("up", strings.Repeat("━", filled-1)+"╸") + th.Faint(strings.Repeat("─", width-filled))
}

// usagePanel is the board's table of what apps cost right now.
func usagePanel(th *ui.Theme, rep appres.Report, sortBy string, live bool, top int) string {
	rows := usageRows(rep, sortBy, top)
	if th.Plain() {
		return plainUsage(rows)
	}
	var maxCPU, maxMem float64
	for _, s := range rows {
		maxCPU, maxMem = maxf(maxCPU, cpuOf(s)), maxf(maxMem, memOf(s))
	}
	window := fmt.Sprintf("%.0f s window", rep.IntervalSeconds)
	if live {
		window = th.Tone("up", "● live") + " " + window
	}
	left := " " + th.Accent("◆") + " " + th.Bold("Usage") + th.Muted("  what apps cost right now")
	header := left + strings.Repeat(" ", max(2, panelWidth-visibleWidth(left)-visibleWidth(window))) + th.Muted(window)
	lines := []string{" " + th.Muted(padRight("NAME", 11)+padRight("CPU", 25)+padRight("MEMORY", 23)+"NET rx / tx")}
	for _, s := range rows {
		net := "-"
		if s.NetRxBps != nil && s.NetTxBps != nil {
			net = compactBytes(*s.NetRxBps) + " / " + compactBytes(*s.NetTxBps)
		}
		cpu, mem := "-", "-"
		if s.CPUPercent != nil {
			cpu = fmt.Sprintf("%.1f%%", *s.CPUPercent)
		}
		if s.MemoryBytes != nil {
			mem = compactBytes(float64(*s.MemoryBytes))
		}
		lines = append(lines, " "+padRight(clip(s.Name, 10), 11)+
			usageBar(th, cpuOf(s), maxCPU, 14)+" "+padRight(cpu, 9)+
			usageBar(th, memOf(s), maxMem, 8)+" "+padRight(mem, 13)+net)
	}
	body := th.Panel("Apps", "by "+sortBy, lines, panelWidth)
	foot := " " + th.Muted("measured from outside: processes, systemd units and containers. - means unknown.")
	return "\n" + header + "\n" + margin(th, []string{body}) + "\n" + foot + "\n"
}

func maxf(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
}

func plainUsage(rows []appres.Series) string {
	var b strings.Builder
	b.WriteString("name cpu memory net_rx net_tx\n")
	for _, s := range rows {
		cpu, mem, rx, tx := "-", "-", "-", "-"
		if s.CPUPercent != nil {
			cpu = fmt.Sprintf("%.1f%%", *s.CPUPercent)
		}
		if s.MemoryBytes != nil {
			mem = compactBytes(float64(*s.MemoryBytes))
		}
		if s.NetRxBps != nil && s.NetTxBps != nil {
			rx, tx = compactBytes(*s.NetRxBps), compactBytes(*s.NetTxBps)
		}
		fmt.Fprintf(&b, "%s %s %s %s %s\n", s.Name, cpu, mem, rx, tx)
	}
	return b.String()
}
