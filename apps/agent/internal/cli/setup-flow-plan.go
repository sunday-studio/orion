package cli

import (
	"fmt"
	"strings"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	"orion/agent/internal/discover"
)

// reviewRows describes the change the plan would make: the new monitors in a
// compact form, other changed settings, and the suggestions that cannot be
// written (log sources need an ingest key). added lists the new monitor names.
func reviewRows(opts *Options, plan setupPlan, items []control.DiscoveryItem) (rows, notWritten, added []string, err error) {
	next, _, err := buildSetupConfig(opts, plan)
	if err != nil {
		return nil, nil, nil, err
	}
	var prev config.UserConfig
	if existing, _, lerr := config.LoadRaw(opts.ConfigPath); lerr == nil && existing != nil && !plan.Force {
		prev = *existing
	}
	if next.CoreURL != prev.CoreURL {
		if prev.CoreURL != "" {
			rows = append(rows, "- core_url: "+prev.CoreURL)
		}
		rows = append(rows, "+ core_url: "+next.CoreURL)
	}
	have := map[string]bool{}
	for _, m := range prev.Monitors {
		have[m.Name] = true
	}
	var fresh []config.UserMonitor
	width := 0
	for _, m := range next.Monitors {
		if !have[m.Name] {
			fresh = append(fresh, m)
			width = max(width, len([]rune(m.Name)))
		}
	}
	if len(fresh) > 0 {
		if len(prev.Monitors) == 0 {
			rows = append(rows, "+ monitors:")
		} else {
			rows = append(rows, "  monitors:")
		}
		for _, m := range fresh {
			line := fmt.Sprintf("+   - name: %s type: %s", padRight(m.Name, width+3), m.Type)
			if m.DiskUsage != nil && m.DiskUsage.MaxPercent > 0 {
				line += fmt.Sprintf("  threshold: %.0f%%", m.DiskUsage.MaxPercent)
			}
			rows = append(rows, strings.TrimRight(line, " "))
			added = append(added, m.Name)
		}
	}
	for _, p := range next.AppResources.Paths {
		if !containsString(prev.AppResources.Paths, p) {
			rows = append(rows, "+ app_resources.paths: "+p)
		}
	}
	for _, it := range items {
		if it.Kind == string(discover.KindLog) && selectable(it) && !it.Dismissed {
			notWritten = append(notWritten, displayName(it)+" needs a log key. Create one in the Console.")
		}
	}
	return rows, notWritten, added, nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
