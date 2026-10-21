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
		if s.Kind != appres.KindPath && s.Kind != appres.KindVolume && s.Kind != appres.KindDocker {
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
		return plainUsage(rows, rep, top)
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
	blocks := []string{th.Panel("Apps", "by "+sortBy, lines, panelWidth)}
	if disk := storageRows(th, rep, top); len(disk) > 0 {
		blocks = append(blocks, th.Panel("Storage", "by size", disk, panelWidth))
	}
	body := strings.Join(blocks, "\n")
	foot := " " + th.Muted("measured from outside: processes, systemd units and containers. - means unknown.")
	return "\n" + header + "\n" + margin(th, []string{body}) + "\n" + foot + "\n"
}

// storageRows lists what holds disk: configured paths, Docker volumes, images
// and build cache, and the writable layer of each container.
func storageRows(th *ui.Theme, rep appres.Report, limit int) []string {
	var items []appres.Series
	for _, s := range rep.Series {
		if s.StorageBytes != nil {
			items = append(items, s)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return *items[i].StorageBytes > *items[j].StorageBytes })
	kinds := map[appres.Kind]string{appres.KindPath: "path", appres.KindVolume: "volume", appres.KindDocker: "docker", appres.KindContainer: "container"}
	var rows []string
	if len(items) > 0 {
		rows = append(rows, " "+th.Muted(padRight("NAME", 34)+padRight("WHAT", 12)+"SIZE"))
	}
	hidden := 0
	if limit > 0 && len(items) > limit {
		hidden, items = len(items)-limit, items[:limit]
	}
	for _, s := range items {
		size := compactBytes(float64(*s.StorageBytes))
		if s.StorageCapped {
			size = "≥ " + size
		}
		rows = append(rows, " "+padRight(clip(s.Name, 32), 34)+th.Muted(padRight(kinds[s.Kind], 12))+size)
	}
	if hidden > 0 {
		rows = append(rows, " "+th.Muted(fmt.Sprintf("… and %d more (orion-agent usage --top %d, or --json for all)", hidden, limit+hidden)))
	}
	return rows
}

func maxf(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
}

func plainUsage(rows []appres.Series, rep appres.Report, top int) string {
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
	if disk := storageRows(ui.Plain(), rep, top); len(disk) > 0 {
		b.WriteString("storage\n")
		for _, r := range disk[1:] {
			b.WriteString(strings.Join(strings.Fields(r), " ") + "\n")
		}
	}
	return b.String()
}
