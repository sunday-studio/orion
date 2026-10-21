package cli

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	"orion/agent/internal/logging"
	agentstate "orion/agent/internal/state"
	"orion/agent/internal/transport"
)

// discoveryResendEvery re-sends an unchanged scan this often, so Core's copy
// does not look stale and a Core that lost it gets it back.
const discoveryResendEvery = 24 * time.Hour

// discoveryReporter sends the latest scan to Core after each scan, once it
// changes (and daily). Failing to send never affects scanning or reporting.
type discoveryReporter struct {
	store *agentstate.Store
	cfg   func() *config.UserConfig
	send  func(coreURL, token, agentID string, rep transport.DiscoveryReport) error // replaced in tests
	now   func() time.Time

	mu          sync.Mutex
	lastHash    string
	lastAt      time.Time
	unsupported time.Time // an older Core: do not ask again before this
	warned      bool
	sending     bool
	pending     *control.Discovery
}

func newDiscoveryReporter(store *agentstate.Store, cfg func() *config.UserConfig) *discoveryReporter {
	return &discoveryReporter{store: store, cfg: cfg, now: time.Now,
		send: func(coreURL, token, agentID string, rep transport.DiscoveryReport) error {
			return transport.NewClient(coreURL, token).SendDiscovery(agentID, rep)
		}}
}

func reportOf(d control.Discovery) transport.DiscoveryReport {
	rep := transport.DiscoveryReport{ScannedAt: d.ScannedAt, Enabled: d.Enabled, Kinds: []transport.DiscoveredKind{}, Items: []transport.DiscoveredItem{}}
	for _, k := range d.Kinds {
		rep.Kinds = append(rep.Kinds, transport.DiscoveredKind{Kind: k.Kind, State: k.State, Message: k.Message, Fix: k.Fix, Why: k.Why})
	}
	for _, it := range d.Items {
		rep.Items = append(rep.Items, transport.DiscoveredItem{ID: it.ID, Kind: it.Kind, Name: it.Name, Detail: it.Detail, Confidence: it.Confidence,
			Status: it.Status, ManagedIn: it.ManagedIn, New: it.New, Dismissed: it.Dismissed})
	}
	return rep
}

// Report sends d unless reporting is off, the host is not enrolled, the scan
// is unchanged, or Core is known not to take it. Only one send runs at a time;
// scans that finish meanwhile are coalesced into the newest one.
func (r *discoveryReporter) Report(d control.Discovery) {
	r.mu.Lock()
	if r.sending {
		r.pending = &d
		r.mu.Unlock()
		return
	}
	r.sending = true
	r.mu.Unlock()
	for {
		r.sendOne(d)
		r.mu.Lock()
		if r.pending == nil {
			r.sending = false
			r.mu.Unlock()
			return
		}
		d, r.pending = *r.pending, nil
		r.mu.Unlock()
	}
}

func (r *discoveryReporter) sendOne(d control.Discovery) {
	if cfg := r.cfg(); cfg == nil || !cfg.Discovery.IsReporting() {
		return
	}
	st, err := r.store.Get()
	if err != nil || !st.IsRegistered() || st.AgentID == "" {
		return
	}
	rep := reportOf(d)
	hashed := rep
	hashed.ScannedAt = time.Time{}
	raw, _ := json.Marshal(hashed)
	hash := string(raw)

	r.mu.Lock()
	now := r.now()
	skip := now.Before(r.unsupported) || (hash == r.lastHash && now.Sub(r.lastAt) < discoveryResendEvery)
	r.mu.Unlock()
	if skip {
		return
	}
	err = r.send(st.CoreURL, st.Token, st.AgentID, rep) // not under the lock: it can take seconds
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		if errors.Is(err, transport.ErrDiscoveryUnsupported) {
			r.unsupported = now.Add(discoveryResendEvery)
		} else if !r.warned {
			logging.Warnf("Could not send the discovery scan to Core (will retry after the next scan): %v", err)
			r.warned = true
		}
		return
	}
	r.warned, r.lastHash, r.lastAt = false, hash, now
}
