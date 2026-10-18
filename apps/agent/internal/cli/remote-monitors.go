package cli

import (
	"context"
	"sync"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/remotemon"
	agentstate "orion/agent/internal/state"
	"orion/agent/internal/transport"
)

// remoteMonitorPollInterval is how often the daemon asks Core for changes to
// the checks configured in the Console; a new check starts within this window.
const remoteMonitorPollInterval = time.Minute

// remoteMonitors keeps the last good set of Console-configured checks. A failed
// sync never clears it, so a Core blip cannot make the agent drop its monitors.
type remoteMonitors struct {
	store *agentstate.Store
	mu    sync.Mutex
	last  []config.UserMonitor
	fp    string
}

func (r *remoteMonitors) current() []config.UserMonitor {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]config.UserMonitor(nil), r.last...)
}

// sync pulls definitions and reports whether the set changed.
func (r *remoteMonitors) sync() (changed bool, err error) {
	st, err := r.store.Get()
	if err != nil || !st.IsRegistered() {
		return false, err
	}
	client := transport.NewClient(st.CoreURL, st.Token)
	defs, err := client.PullMonitorDefinitions(st.AgentID)
	if err != nil {
		return false, err
	}
	monitors, problems := remotemon.Convert(defs)
	for name, why := range problems {
		logging.Errorf("Console check %q cannot run on this agent: %s", name, why)
	}
	fp := remotemon.Fingerprint(monitors)
	r.mu.Lock()
	defer r.mu.Unlock()
	if fp == r.fp {
		return false, nil
	}
	r.last, r.fp = monitors, fp
	logging.Infof("Console monitor definitions changed: %d check(s)", len(monitors))
	return true, nil
}

// poll syncs on an interval and calls onChange when the set changes.
func (r *remoteMonitors) poll(ctx context.Context, every time.Duration, onChange func()) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if changed, err := r.sync(); err != nil {
				logging.Warnf("Monitor definition sync failed: %v", err)
			} else if changed {
				onChange()
			}
		}
	}
}

// effective returns the file config plus the Console checks.
func effective(file *config.UserConfig, remote []config.UserMonitor) *config.UserConfig {
	cp := *file
	cp.Monitors = append([]config.UserMonitor(nil), file.Monitors...)
	remotemon.Merge(&cp, remote)
	return &cp
}
