package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	"orion/agent/internal/discover"
	"orion/agent/internal/logging"
	agentstate "orion/agent/internal/state"
	"orion/agent/internal/transport"
)

// consoleRequestPollInterval is how often the daemon asks Core whether a person
// asked for something in the Console (rescan, dismiss, track storage, ...).
const consoleRequestPollInterval = time.Minute

// requestApplier carries out Console requests about this host. Every request
// ends with a result Core shows verbatim, so the text says what happened here.
type requestApplier struct {
	store      *agentstate.Store
	configPath string
	scan       *scanner
	reload     func() error // validates and reloads the config file
	pull       func(st *config.InternalState) ([]transport.PulledRequest, error)
	report     func(st *config.InternalState, id, status, text string) error

	reloadPending bool // the file changed but the daemon has not confirmed a reload yet

	unsupportedUntil time.Time // an older Core: stop asking for a while
}

func newRequestApplier(store *agentstate.Store, configPath string, scan *scanner, reload func() error) *requestApplier {
	return &requestApplier{store: store, configPath: configPath, scan: scan, reload: reload,
		pull: func(st *config.InternalState) ([]transport.PulledRequest, error) {
			return transport.NewClient(st.CoreURL, st.Token).PullRequests(st.AgentID)
		},
		report: func(st *config.InternalState, id, status, text string) error {
			return transport.NewClient(st.CoreURL, st.Token).SendRequestResult(st.AgentID, id, status, text)
		}}
}

// run polls until ctx ends. Failures are logged and retried on the next tick.
func (a *requestApplier) run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	first := time.After(5 * time.Second) // a request queued while the agent was down is answered soon after start
	for {
		select {
		case <-ctx.Done():
			return
		case <-first:
		case <-t.C:
		}
		if err := a.poll(ctx); err != nil {
			logging.Warnf("Console request poll failed: %v", err)
		}
	}
}

func (a *requestApplier) poll(ctx context.Context) error {
	if time.Now().Before(a.unsupportedUntil) {
		return nil
	}
	st, err := a.store.Get()
	if err != nil || !st.IsRegistered() || st.AgentID == "" {
		return err
	}
	reqs, err := a.pull(st)
	if errors.Is(err, transport.ErrDiscoveryUnsupported) {
		a.unsupportedUntil = time.Now().Add(discoveryResendEvery)
		return nil
	}
	if err != nil {
		return err
	}
	for _, r := range reqs {
		status, text := a.apply(ctx, r)
		if err := a.report(st, r.ID, status, text); err != nil {
			// Core redelivers an unconfirmed request; every handler is safe to repeat.
			logging.Warnf("Could not report the result of Console request %s: %v", r.ID, err)
		}
	}
	return nil
}

// apply runs one request and returns the status and the text Core shows.
func (a *requestApplier) apply(ctx context.Context, r transport.PulledRequest) (status, text string) {
	text, err := a.handle(ctx, r)
	switch {
	case errors.Is(err, errUnsupportedRequest):
		return "unsupported", err.Error()
	case err != nil:
		return "failed", err.Error()
	}
	return "applied", text
}

var errUnsupportedRequest = errors.New("this agent does not know that request; update the agent")

func (a *requestApplier) handle(ctx context.Context, r transport.PulledRequest) (string, error) {
	switch r.Type {
	case "rescan":
		d, err := a.scan.Rescan(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Scanned: %d item(s) found", len(d.Items)), nil
	case "dismiss", "restore":
		return a.dismissOrRestore(r)
	case "ignore", "unignore":
		return a.editIgnore(r)
	case "set_scanning":
		on, ok := r.Payload["enabled"].(bool)
		if !ok {
			return "", errors.New("set_scanning needs enabled true or false")
		}
		return a.editConfig(fmt.Sprintf("Scanning turned %s", map[bool]string{true: "on", false: "off"}[on]),
			func() (bool, error) { return editDiscoveryEnabled(a.configPath, on) })
	case "set_kind":
		kind, _ := r.Payload["kind"].(string)
		on, ok := r.Payload["enabled"].(bool)
		if !ok || !containsString(discover.AllKinds, kind) {
			return "", fmt.Errorf("set_kind needs a kind (%s) and enabled", strings.Join(discover.AllKinds, ", "))
		}
		return a.editConfig(fmt.Sprintf("Scanning %s turned %s", kind, map[bool]string{true: "on", false: "off"}[on]),
			func() (bool, error) { return editDiscoveryKind(a.configPath, kind, on, discover.AllKinds) })
	case "track_storage":
		paths := stringsOf(r.Payload["paths"])
		if len(paths) == 0 {
			return "", errors.New("track_storage needs at least one path")
		}
		if len(paths) > maxRequestPaths {
			return "", fmt.Errorf("track_storage takes at most %d paths", maxRequestPaths)
		}
		for _, p := range paths {
			if err := validateStoragePath(p); err != nil {
				return "", err
			}
		}
		return a.editConfig(fmt.Sprintf("Tracking %d path(s) for storage size", len(paths)),
			func() (bool, error) { return editStoragePaths(a.configPath, paths) })
	}
	return "", errUnsupportedRequest
}

func (a *requestApplier) dismissOrRestore(r transport.PulledRequest) (string, error) {
	ids := stringsOf(r.Payload["ids"])
	if len(ids) == 0 {
		return "", fmt.Errorf("%s needs at least one id", r.Type)
	}
	if len(ids) > maxRequestIDs {
		return "", fmt.Errorf("%s takes at most %d ids", r.Type, maxRequestIDs)
	}
	var failed []string
	for _, id := range ids {
		// A repeat (Core redelivers an unconfirmed request) finds it already so, which is success.
		var already notDismissedError
		if err := a.scan.Action(control.DiscoveryAction{Action: r.Type, ID: id}); err != nil && !errors.As(err, &already) {
			failed = append(failed, err.Error())
		}
	}
	if len(failed) > 0 {
		return "", errors.New(strings.Join(failed, "; "))
	}
	a.scan.requestScan()
	return fmt.Sprintf("%s %d item(s)", map[string]string{"dismiss": "Dismissed", "restore": "Restored"}[r.Type], len(ids)), nil
}

func (a *requestApplier) editIgnore(r transport.PulledRequest) (string, error) {
	ids := stringsOf(r.Payload["ids"])
	if len(ids) == 0 {
		return "", fmt.Errorf("%s needs at least one id", r.Type)
	}
	if len(ids) > maxRequestIDs {
		return "", fmt.Errorf("%s takes at most %d ids", r.Type, maxRequestIDs)
	}
	add := r.Type == "ignore"
	return a.editConfig(fmt.Sprintf("%s %d item(s) in discovery.ignore", map[bool]string{true: "Added", false: "Removed"}[add], len(ids)),
		func() (bool, error) {
			changed := false
			for _, id := range ids {
				c, err := editDiscoveryIgnore(a.configPath, id, add)
				if err != nil {
					return changed, err
				}
				changed = changed || c
			}
			return changed, nil
		})
}

// editConfig edits the config file, asks the daemon to reload it and scan
// again. A change that is already in place is still a success.
func (a *requestApplier) editConfig(done string, edit func() (bool, error)) (string, error) {
	if a.configPath == "" {
		return "", errors.New("this agent has no config file to edit")
	}
	changed, err := edit()
	if err != nil {
		return "", fmt.Errorf("could not edit %s: %w", a.configPath, err)
	}
	if !changed && !a.reloadPending {
		return done + " (already so)", nil
	}
	// A repeat after an earlier delivery changed the file but failed to reload still reloads.
	a.reloadPending = true
	a.scan.rescanOnConfig.Store(true)
	if a.reload != nil {
		if err := a.reload(); err != nil {
			return "", fmt.Errorf("edited %s but %w", a.configPath, err)
		}
	}
	a.reloadPending = false
	return done, nil
}

const (
	maxRequestIDs   = 100
	maxRequestPaths = 20
	maxPathLen      = 1024
)

// validateStoragePath accepts only a clean absolute path below the root. Core
// checks the same, but the agent walks these paths, so it does not rely on that.
func validateStoragePath(p string) error {
	switch {
	case len(p) > maxPathLen:
		return fmt.Errorf("a storage path is longer than %d characters", maxPathLen)
	case !filepath.IsAbs(p) || filepath.Clean(p) != p || p == "/":
		return fmt.Errorf("storage path %q must be a clean absolute path other than /", p)
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("storage path %q has a control character", p)
		}
	}
	return nil
}

func stringsOf(v any) []string {
	list, _ := v.([]any)
	var out []string
	for _, x := range list {
		if s, ok := x.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
