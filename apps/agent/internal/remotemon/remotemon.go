// Package remotemon turns the checks configured in the Console (pulled from
// Core) into agent monitors, so they run through the same registration,
// collection and reporting path as monitors from the config file.
package remotemon

import (
	"encoding/json"
	"fmt"
	"sort"

	"orion/agent/internal/config"
	"orion/agent/internal/transport"
)

// DefaultInterval applies when a definition has no interval.
const DefaultInterval = 60

// Convert maps definitions to monitors. Definitions it cannot run are returned
// as problems (name -> reason) so they can be logged, never silently dropped.
func Convert(defs []transport.PulledDefinition) (monitors []config.UserMonitor, problems map[string]string) {
	problems = map[string]string{}
	for _, d := range defs {
		m, err := convertOne(d)
		if err != nil {
			problems[d.Name] = err.Error()
			continue
		}
		monitors = append(monitors, m)
	}
	sort.Slice(monitors, func(i, j int) bool { return monitors[i].Name < monitors[j].Name })
	return monitors, problems
}

func convertOne(d transport.PulledDefinition) (config.UserMonitor, error) {
	interval := d.IntervalSeconds
	if interval <= 0 {
		interval = DefaultInterval
	}
	m := config.UserMonitor{
		Name:        d.Name,
		Description: fmt.Sprintf("%s check configured in Console", d.CheckType),
		Interval:    fmt.Sprintf("%ds", interval),
		Meta:        map[string]interface{}{"source": "console", "definition_id": d.ID, "fail_after": d.FailAfter},
	}
	switch d.CheckType {
	case "disk":
		path := stringParam(d.Params, "path")
		if path == "" {
			path = "/"
		}
		m.Type = config.UserMonitorTypeDiskUsage
		m.DiskUsage = &config.DiskUsageConfig{Path: path, MaxPercent: numberParam(d.Params, "threshold_percent", 90)}
	case "memory":
		m.Type = config.UserMonitorTypeResource
		m.Resource = &config.ResourceThresholdConfig{MaxMemoryPercent: numberParam(d.Params, "threshold_percent", 90)}
	case "process":
		name := stringParam(d.Params, "process_name")
		if name == "" {
			return m, fmt.Errorf("process check needs params.process_name")
		}
		m.Type = config.UserMonitorTypeProcess
		m.ProcessRunning = &config.ProcessRunningConfig{Name: name}
	case "port":
		port := int(numberParam(d.Params, "port", 0))
		if port <= 0 || port > 65535 {
			return m, fmt.Errorf("port check needs params.port between 1 and 65535")
		}
		m.Type = config.UserMonitorTypeTCP
		m.TCP = &config.TCPMonitorConfig{Host: "127.0.0.1", Port: port, Timeout: "3s"}
	default:
		return m, fmt.Errorf("this agent version does not support check type %q; update the agent", d.CheckType)
	}
	if err := m.Validate(); err != nil {
		return m, err
	}
	return m, nil
}

func stringParam(p map[string]any, k string) string {
	s, _ := p[k].(string)
	return s
}

// numberParam reads JSON numbers (float64) and numeric strings.
func numberParam(p map[string]any, k string, def float64) float64 {
	switch v := p[k].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f
		}
	}
	return def
}

// Merge adds remote monitors to cfg. A monitor from the config file keeps its
// name, so a hand-written entry is never replaced by a Console one.
func Merge(cfg *config.UserConfig, remote []config.UserMonitor) {
	have := map[string]bool{}
	for _, m := range cfg.Monitors {
		have[m.Name] = true
	}
	for _, m := range remote {
		if !have[m.Name] {
			cfg.Monitors = append(cfg.Monitors, m)
		}
	}
}

// Fingerprint identifies a definition set so the daemon can tell when it changed.
func Fingerprint(monitors []config.UserMonitor) string {
	b, _ := json.Marshal(monitors)
	return string(b)
}
