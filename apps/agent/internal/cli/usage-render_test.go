package cli

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"orion/agent/internal/appres"
	"orion/agent/internal/ui"
)

func f64(v float64) *float64 { return &v }
func u64(v uint64) *uint64   { return &v }

func boardUsage() appres.Report {
	return appres.Report{IntervalSeconds: 3, Series: []appres.Series{
		{Kind: appres.KindUnit, Name: "redis", CPUPercent: f64(0.2), MemoryBytes: u64(31 << 20), NetRxBps: f64(12 << 10), NetTxBps: f64(8 << 10)},
		{Kind: appres.KindUnit, Name: "nginx", CPUPercent: f64(4.1), MemoryBytes: u64(86 << 20), NetRxBps: f64(1.2 * (1 << 20)), NetTxBps: f64(3.4 * (1 << 20))},
		{Kind: appres.KindContainer, Name: "api", CPUPercent: f64(1.2), MemoryBytes: u64(204 << 20), NetRxBps: f64(320 << 10), NetTxBps: f64(90 << 10)},
		{Kind: appres.KindUnit, Name: "postgresql", CPUPercent: f64(2.8), MemoryBytes: u64(412 << 20)},
		{Kind: appres.KindPath, Name: "/var/lib", StorageBytes: u64(1 << 30)},
	}}
}

func TestUsageScreenMatchesBoard(t *testing.T) {
	board := boardBlock(t, "cli-discover-usage.txt", "usage --watch")
	_, board, _ = strings.Cut(board, "\n")
	board = strings.Replace(board, "\n  s  sort   q  quit\n", "\n", 1)
	for _, dark := range []bool{false, true} {
		got := stripANSI(usagePanel(ui.WithMode(dark), boardUsage(), "CPU", true, 10))
		sameAsBoard(t, "usage", board, got)
	}
	if p := usagePanel(ui.Plain(), boardUsage(), "CPU", false, 10); strings.ContainsAny(p, "━╸╭│●") || !strings.Contains(p, "postgresql 2.8% 412 MB - -") {
		t.Errorf("plain usage:\n%s", p)
	}
}

func TestUsageUnknownIsADashAndSortKeyReorders(t *testing.T) {
	rows := usageRows(boardUsage(), "network", 10)
	if rows[0].Name != "nginx" || rows[len(rows)-1].Name != "postgresql" {
		t.Errorf("network order: first %s last %s", rows[0].Name, rows[len(rows)-1].Name)
	}
	if rows := usageRows(boardUsage(), "memory", 2); len(rows) != 2 || rows[0].Name != "postgresql" {
		t.Errorf("memory order / top: %+v", rows)
	}
	m := newUsageModel(ui.WithMode(false), func() appres.Report { return boardUsage() }, 1, 10)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if !strings.Contains(stripANSI(next.(usageModel).View()), "by memory") {
		t.Error("s cycles the sort")
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); cmd == nil {
		t.Error("q quits")
	}
}
