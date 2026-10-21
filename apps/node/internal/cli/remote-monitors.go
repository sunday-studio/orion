package cli

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"orion/node/internal/config"
	"orion/node/internal/logging"
	"orion/node/internal/remotemon"
	nodestate "orion/node/internal/state"
	"orion/node/internal/transport"
)

// remoteMonitorPollInterval is how often the daemon asks Core for changes to
// the checks configured in the Console; a new check starts within this window.
const remoteMonitorPollInterval = time.Minute

// remoteMonitors keeps the last good set of Console-configured checks. A failed
// sync never clears it, so a Core blip cannot make the node drop its monitors.
type remoteMonitors struct {
	store *nodestate.Store
	path  string // persisted last good set, so a restart during a Core outage keeps them
	mu    sync.Mutex
	last  []config.UserMonitor
	fp    string
	// posted is the last outcome list Core accepted, so an unchanged one is not sent every minute.
	posted string
	// sendResults replaces the real call in tests.
	sendResults func(st *config.InternalState, res []transport.DefinitionResult) error
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
	defs, err := client.PullMonitorDefinitions(st.NodeID)
	if err != nil {
		return false, err
	}
	monitors, problems := remotemon.Convert(defs)
	for name, why := range problems {
		logging.Errorf("Console check %q cannot run on this node: %s", name, why)
	}
	r.postResults(st, remotemon.Results(defs, problems))
	fp := remotemon.Fingerprint(monitors)
	r.mu.Lock()
	defer r.mu.Unlock()
	if fp == r.fp {
		return false, nil
	}
	r.last, r.fp = monitors, fp
	r.saveLocked()
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

// load restores the last good set persisted by a previous run.
func (r *remoteMonitors) load() {
	if r.path == "" {
		return
	}
	data, err := os.ReadFile(r.path)
	if err != nil {
		return
	}
	var ms []config.UserMonitor
	if json.Unmarshal(data, &ms) != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last, r.fp = ms, remotemon.Fingerprint(ms)
}

func (r *remoteMonitors) saveLocked() {
	if r.path == "" {
		return
	}
	data, err := json.Marshal(r.last)
	if err != nil {
		return
	}
	tmp := r.path + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, r.path)
	}
}

// postResults tells Core which checks this node cannot run (and clears ones that now run).
// An older Core without the endpoint just keeps showing the check as waiting.
func (r *remoteMonitors) postResults(st *config.InternalState, res []transport.DefinitionResult) {
	raw, _ := json.Marshal(res)
	r.mu.Lock()
	same := r.posted == string(raw)
	r.mu.Unlock()
	if same || len(res) == 0 {
		return
	}
	send := r.sendResults
	if send == nil {
		send = func(st *config.InternalState, res []transport.DefinitionResult) error {
			return transport.NewClient(st.CoreURL, st.Token).SendDefinitionResults(st.NodeID, res)
		}
	}
	if err := send(st, res); err != nil {
		logging.Warnf("Could not report Console check outcomes: %v", err)
		return
	}
	r.mu.Lock()
	r.posted = string(raw)
	r.mu.Unlock()
}
