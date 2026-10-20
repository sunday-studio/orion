package cli

import (
	"context"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	"orion/agent/internal/discover"
)

// hasSuggestions says there is something to pick.
func hasSuggestions(d control.Discovery) bool {
	for _, it := range d.Items {
		if !it.Dismissed && it.Status == discover.StatusSuggested {
			return true
		}
	}
	return false
}

// knownCoreURL is the Core this host reports to, from its registration or its
// config. Without one a monitor list cannot be saved yet.
func knownCoreURL(opts *Options) string {
	if st := currentSetupState(opts.StatePath); st != nil {
		return st.CoreURL
	}
	if cfg, _, err := config.LoadRaw(opts.ConfigPath); err == nil && cfg != nil {
		return cfg.CoreURL
	}
	return ""
}

// runDiscoverPicker is `discover` on a terminal: the same grouped list as
// setup, with the same keys, then the same review and apply. It skips the
// connect step because the host already knows its Core.
func runDiscoverPicker(ctx context.Context, opts *Options, coreURL string) error {
	opts.ElevateAs = "discover"
	f := newSetupFlow(ctx, opts, theme)
	f.title, f.enrolled, f.step = "Discover", true, flowScanning
	f.plan.CoreURL = coreURL
	err := runFlowWith(ctx, f)
	_ = controlClient(opts).DiscoveryAction("review", "") // a person has now seen the list
	return err
}
