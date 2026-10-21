package cli

import (
	"fmt"
	"strings"

	"orion/node/internal/config"
	"orion/node/internal/discover"
	"orion/node/internal/ui"
)

// originLabel names where a setting came from; "file" is the unremarkable
// default and is left off.
func originLabel(origin string) string {
	if origin == "" || origin == "file" {
		return ""
	}
	return origin
}

func monitorOrigin(m config.UserMonitor, src config.Source) string {
	if s, _ := m.Meta["source"].(string); s == "scan" {
		return "accepted from scan"
	}
	if s, _ := m.Meta["source"].(string); s == "console" {
		return "managed in Console"
	}
	return src.MonitorOrigins[m.Name]
}

// monitorKind is the type column of the tree: "disk-usage 85%", "tcp :6379".
func monitorKind(m config.UserMonitor) string {
	t := string(m.Type)
	switch {
	case m.DiskUsage != nil && m.DiskUsage.MaxPercent > 0:
		return fmt.Sprintf("%s %.0f%%", t, m.DiskUsage.MaxPercent)
	case m.TCP != nil && m.TCP.Port > 0:
		return fmt.Sprintf("%s :%d", t, m.TCP.Port)
	}
	return t
}

func discoverySummary(d config.DiscoveryConfig) string {
	if !d.IsEnabled() {
		return "off"
	}
	kinds := d.Kinds
	if len(kinds) == 0 {
		kinds = discover.AllKinds
	}
	s := "on · " + strings.Join(kinds, " ")
	if n := len(d.Ignore); n > 0 {
		s += fmt.Sprintf(" · ignore %d", n)
	}
	return s
}

// configShowScreen is `config show`: where the settings come from, then the
// settings with a small origin label each.
func configShowScreen(th *ui.Theme, cfg *config.UserConfig, src config.Source) string {
	var sources []string
	switch {
	case src.ZeroConf:
		sources = append(sources, " "+th.Muted(padRight("env", 10))+"environment only (no config file)")
	case src.FileFound:
		sources = append(sources, " "+th.Muted(padRight("file", 10))+src.File)
	}
	for _, p := range src.DropIns {
		sources = append(sources, " "+th.Muted(padRight("drop-in", 10))+p)
	}
	for _, e := range src.EnvKeys {
		what := envOriginKeyName(e)
		sources = append(sources, " "+th.Muted(padRight("env", 10))+e+" overrides "+what)
	}
	row := func(key, value, origin string) string {
		line := " " + th.Muted(padRight(key, 10)) + value
		if o := originLabel(origin); o != "" {
			line = padRight(line, 48) + th.Muted(o)
		}
		return line
	}
	settings := []string{
		row("core_url", cfg.CoreURL, src.Origins["core_url"]),
		row("interval", cfg.Interval, src.Origins["interval"]),
		row("discovery", discoverySummary(cfg.Discovery), src.Origins["discovery"]),
		row("logging", strings.TrimSpace(cfg.Logging.Level+" · "+cfg.Logging.Path), src.Origins["logging"]),
		row("monitors", fmt.Sprint(len(cfg.Monitors)), ""),
	}
	for i, m := range cfg.Monitors {
		glyph := "├─"
		if i == len(cfg.Monitors)-1 {
			glyph = "╰─"
		}
		settings = append(settings, " "+strings.Repeat(" ", 10)[:2]+th.Faint(glyph)+" "+padRight(clip(m.Name, 14), 15)+padRight(monitorKind(m), 20)+th.Muted(firstNonEmpty(monitorOrigin(m, src), "file")))
	}
	if n := len(cfg.Logs.Sources); n > 0 {
		first := cfg.Logs.Sources[0]
		detail := first.Name
		if first.Path != "" {
			detail += " · " + first.Path
		}
		line := " " + th.Muted(padRight("logs", 10)) + padRight(fmt.Sprint(n), 4) + padRight(clip(detail, 40), 41)
		if o := originLabel(src.LogOrigins[first.Name]); o != "" {
			line += th.Muted(o)
		} else {
			line += th.Muted("file")
		}
		settings = append(settings, line)
	}
	if th.Plain() {
		return plainConfigShow(cfg, src)
	}
	blocks := []string{th.Accent("◆") + " " + th.Bold("Config") + th.Muted("  effective settings"),
		th.Panel("Sources", "", sources, panelWidth), th.Panel("Settings", "", settings, panelWidth)}
	return "\n" + margin(th, blocks) + "\n"
}

func envOriginKeyName(env string) string {
	switch env {
	case config.EnvCoreURL:
		return "core_url"
	case config.EnvInterval:
		return "interval"
	}
	return "logging"
}

func plainConfigShow(cfg *config.UserConfig, src config.Source) string {
	var b strings.Builder
	fmt.Fprintf(&b, "config: %s\n", src.File)
	for _, p := range src.DropIns {
		fmt.Fprintf(&b, "drop-in: %s\n", p)
	}
	for _, e := range src.EnvKeys {
		fmt.Fprintf(&b, "env: %s overrides %s\n", e, envOriginKeyName(e))
	}
	fmt.Fprintf(&b, "core_url: %s\ninterval: %s\ndiscovery: %s\nlogging: %s %s\nmonitors: %d\n", cfg.CoreURL, cfg.Interval, discoverySummary(cfg.Discovery), cfg.Logging.Level, cfg.Logging.Path, len(cfg.Monitors))
	for _, m := range cfg.Monitors {
		fmt.Fprintf(&b, "  %s %s\n", m.Name, monitorKind(m))
	}
	fmt.Fprintf(&b, "logs: %d\n", len(cfg.Logs.Sources))
	return b.String()
}

// configDiffScreen is `config diff`: what is changed, then a one-line summary.
func configDiffScreen(th *ui.Theme, against string, changes []string, base bool) string {
	var b strings.Builder
	b.WriteString("\n " + th.Muted("against "+against) + "\n")
	plus := 0
	for _, l := range changes {
		if len(l) > 2 {
			l = l[:2] + config.RedactLine(l[2:]) // never print a credential value
		}
		switch {
		case strings.HasPrefix(l, "- "):
			b.WriteString("   " + th.Tone("down", l) + "\n")
		case strings.HasPrefix(l, "+ "):
			b.WriteString("   " + th.Tone("up", l) + "\n")
			plus++
		}
	}
	if plus == 0 && len(changes) == 0 {
		return b.String() + "\n   " + th.Pass() + " No differences\n"
	}
	what := "between the two"
	if base {
		what = "from the drop-in and environment"
	}
	return b.String() + "\n   " + th.Pass() + " " + fmt.Sprintf("%d %s %s", max(plus, 1), plural(max(plus, 1), "change", "changes"), what) + "\n"
}

// configValidScreen is the quiet success of `config validate`.
func configValidScreen(th *ui.Theme, path string, monitors int) string {
	return "\n" + credStep(th, "Config valid", fmt.Sprintf("%s · %d %s", path, monitors, plural(monitors, "monitor", "monitors"))) + "\n"
}
