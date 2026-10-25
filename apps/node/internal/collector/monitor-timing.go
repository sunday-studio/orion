package collector

import (
	"encoding/json"
	"time"

	"orion/node/internal/config"
)

// CollectMonitorReport runs one check and records how long it took as
// metrics.duration_ms (whole milliseconds), which Core charts as the monitor's
// response time. A check that already reports its own duration_ms keeps it.
func CollectMonitorReport(internalMonitor config.InternalStateMonitor, userMonitorConfig config.UserMonitor) (*MonitorResult, error) {
	started := time.Now()
	result, err := collectMonitorReport(internalMonitor, userMonitorConfig)
	if result != nil {
		result.Metrics = withDuration(result.Metrics, time.Since(started))
	}
	return result, err
}

func withDuration(metrics interface{}, took time.Duration) interface{} {
	ms := took.Milliseconds()
	if ms == 0 && took > 0 {
		ms = 1 // a check that ran is never shown as instant
	}
	var m map[string]interface{}
	switch v := metrics.(type) {
	case nil:
		m = map[string]interface{}{}
	case *map[string]interface{}:
		if v == nil {
			m = map[string]interface{}{}
			metrics = &m
		} else {
			if *v == nil {
				*v = map[string]interface{}{}
			}
			m = *v
		}
	default:
		raw, err := json.Marshal(v)
		if err != nil || json.Unmarshal(raw, &m) != nil || m == nil {
			return metrics
		}
		metrics = m
	}
	if _, ok := m["duration_ms"]; !ok {
		m["duration_ms"] = ms
	}
	if metrics == nil {
		return m
	}
	return metrics
}
