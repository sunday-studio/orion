package cli

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	"orion/agent/internal/discover"
	"orion/agent/internal/logging"
	agentstate "orion/agent/internal/state"
)

// rescanEvery is how often the daemon looks at the host again on its own.
const rescanEvery = 6 * time.Hour

// scanner runs the read-only discovery scan for the daemon, remembers the
// result and what the person decided, and notices things that are new.
type scanner struct {
	store *agentstate.Store
	cfg   func() *config.UserConfig
	env   func(*config.UserConfig) discover.Env // replaced in tests
	// onScan is told about each finished scan (the daemon sends it to Core).
	onScan func(control.Discovery)

	// kick asks the loop for a scan now; rescanOnConfig makes the next config
	// reload do that, so a Console change shows up in Core without waiting.
	kick           chan struct{}
	rescanOnConfig atomic.Bool

	mu       sync.Mutex
	scanning bool
	last     discover.Result
	at       time.Time
	enabled  bool
}

func newScanner(store *agentstate.Store, cfg func() *config.UserConfig) *scanner {
	return &scanner{store: store, cfg: cfg, env: hostDiscoverEnv, kick: make(chan struct{}, 1)}
}

// hostDiscoverEnv is the real-host scan configured by discovery.* and the
// current monitors (so hand-written entries show as already monitored).
func hostDiscoverEnv(cfg *config.UserConfig) discover.Env {
	e := discover.HostEnv(runtime.GOOS)
	e.Kinds = map[string]bool{}
	for _, k := range discover.AllKinds {
		e.Kinds[k] = cfg.Discovery.KindEnabled(k)
	}
	e.Ignore = cfg.Discovery.Ignore
	e.Existing = cfg.Monitors
	e.Tracked = cfg.AppResources.Paths
	return e
}

// loop scans at start and then slowly, until ctx ends.
func (s *scanner) loop(ctx context.Context) {
	for {
		if _, err := s.Rescan(ctx); err != nil {
			logging.Warnf("Discovery scan failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(rescanEvery):
		case <-s.kick:
		}
	}
}

// requestScan asks the loop to scan soon; it never blocks.
func (s *scanner) requestScan() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// Rescan scans now, records the result and returns the view.
func (s *scanner) Rescan(ctx context.Context) (control.Discovery, error) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return control.Discovery{}, errors.New("a scan is already running")
	}
	s.scanning = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.scanning = false; s.mu.Unlock() }()

	cfg := s.cfg()
	if cfg == nil || !cfg.Discovery.IsEnabled() {
		s.mu.Lock()
		s.enabled, s.last, s.at = false, discover.Result{}, time.Now()
		s.mu.Unlock()
		view, err := s.View()
		if err == nil && s.onScan != nil {
			go s.onScan(view) // tell Core scanning is off, so the Console does not show a stale list
		}
		return view, err
	}
	res := discover.Run(ctx, s.env(cfg))
	now := time.Now()
	baseline := !s.store.HasScanned()
	items := make([]agentstate.DiscoveryItem, 0, len(res.Candidates))
	for _, c := range res.Candidates {
		items = append(items, agentstate.DiscoveryItem{ID: c.ID, Kind: string(c.Kind), Name: c.Name, Detail: c.Detail,
			Confidence: string(c.Confidence), Monitored: c.Status == discover.StatusMonitored, ManagedIn: c.ManagedIn})
	}
	scanned := map[string]bool{}
	for _, k := range res.Kinds {
		scanned[k.Kind] = k.State == discover.ScanOK
	}
	ignored := map[string]bool{}
	for _, id := range cfg.Discovery.Ignore {
		ignored[id] = true
	}
	canForget := func(it agentstate.DiscoveryItem) bool {
		return scanned[discover.ConfigKind(discover.Kind(it.Kind))] && !ignored[it.ID]
	}
	if _, err := s.store.RecordScan(items, now, baseline, canForget); err != nil {
		return control.Discovery{}, fmt.Errorf("record scan: %w", err)
	}
	s.mu.Lock()
	s.enabled, s.last, s.at = true, res, now
	s.mu.Unlock()
	view, err := s.View()
	if err == nil && s.onScan != nil {
		go s.onScan(view)
	}
	return view, err
}

// View merges the latest scan with the stored choices (new, dismissed).
func (s *scanner) View() (control.Discovery, error) {
	stored, err := s.store.ListDiscovery()
	if err != nil {
		return control.Discovery{}, err
	}
	byID := map[string]agentstate.DiscoveryItem{}
	for _, it := range stored {
		byID[it.ID] = it
	}
	s.mu.Lock()
	last, at, enabled, scanning := s.last, s.at, s.enabled, s.scanning
	s.mu.Unlock()
	cfg := s.cfg()
	d := control.Discovery{Enabled: enabled, Scanning: scanning, ScannedAt: at}
	if cfg != nil {
		d.Ignore = cfg.Discovery.Ignore
		d.Enabled = cfg.Discovery.IsEnabled()
	}
	for _, k := range last.Kinds {
		d.Kinds = append(d.Kinds, control.KindState{Kind: k.Kind, State: k.State, Message: k.Message, Fix: k.Fix, Why: k.Why})
	}
	present := map[string]bool{}
	ignored := map[string]bool{}
	if cfg != nil {
		for _, id := range cfg.Discovery.Ignore {
			ignored[id] = true
		}
	}
	for _, c := range last.Candidates {
		if ignored[c.ID] { // ignoring takes effect at once, not after the next scan
			continue
		}
		present[c.ID] = true
		st := byID[c.ID]
		d.Items = append(d.Items, control.DiscoveryItem{ID: c.ID, Kind: string(c.Kind), Name: c.Name, Detail: c.Detail, Confidence: string(c.Confidence),
			Status: c.Status, ManagedIn: c.ManagedIn, New: st.New, Dismissed: st.Dismissed, FirstSeen: st.FirstSeenAt, DismissedAt: st.DismissedAt})
	}
	for _, it := range stored { // dismissed things that are not on the host right now stay listed
		if it.Dismissed && !present[it.ID] && !ignored[it.ID] {
			d.Items = append(d.Items, control.DiscoveryItem{ID: it.ID, Kind: it.Kind, Name: it.Name, Detail: it.Detail, Confidence: it.Confidence,
				Status: discover.StatusSuggested, Dismissed: true, FirstSeen: it.FirstSeenAt, DismissedAt: it.DismissedAt})
		}
	}
	return d, nil
}

// Action applies a person's choice about a suggestion.
func (s *scanner) Action(a control.DiscoveryAction) error {
	switch a.Action {
	case "dismiss", "restore":
		if a.Action == "restore" {
			items, err := s.store.ListDiscovery()
			if err != nil {
				return err
			}
			for _, it := range items {
				if it.ID == a.ID && !it.Dismissed {
					return fmt.Errorf("%s is not dismissed", a.ID)
				}
			}
		}
		ok, err := s.store.SetDismissed(a.ID, a.Action == "dismiss", time.Now())
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%q is not a known suggestion (run: orion-agent discover)", a.ID)
		}
		return nil
	case "review":
		return s.store.MarkReviewed()
	}
	return fmt.Errorf("unknown discovery action %q", a.Action)
}

// pending counts suggestions nobody has handled, and lists the new ones.
func pendingFromView(d control.Discovery) (int, []control.DiscoveryItem) {
	n := 0
	var fresh []control.DiscoveryItem
	for _, it := range d.Items {
		if it.Status != discover.StatusSuggested || it.Dismissed {
			continue
		}
		n++
		if it.New {
			fresh = append(fresh, it)
		}
	}
	return n, fresh
}
