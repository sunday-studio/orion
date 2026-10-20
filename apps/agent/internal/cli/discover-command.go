package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	"orion/agent/internal/discover"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newDiscoverCommand(ctx context.Context, opts *Options) *cobra.Command {
	var rescan, dismissed bool
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "See what Orion found on this server and could monitor",
		Long: "Orion scans this server read-only for services, containers, ports, disks, log files and the space apps use,\n" +
			"and suggests what to monitor. Nothing is added until you apply it (orion-agent setup). What you dismiss or\n" +
			"ignore stays declined; hand-written config always wins.\n\n" +
			"  orion-agent discover                 the suggestions\n" +
			"  orion-agent discover --rescan        look again now and show what could not be scanned\n" +
			"  orion-agent discover --dismissed     what you dismissed\n" +
			"  orion-agent discover ignore <id>     never suggest it again (saved to discovery.ignore)\n" +
			"  orion-agent discover restore <id>    suggest it again",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, host, err := loadDiscovery(ctx, opts, rescan)
			if err != nil {
				return NewCommandError("could not scan this server", err)
			}
			if opts.JSON {
				return renderJSON(outputWriter, d)
			}
			if dismissed {
				fmt.Fprint(outputWriter, dismissedScreen(theme, d, terminalWidth()))
				return nil
			}
			fmt.Fprint(outputWriter, discoverScreen(theme, d, host, terminalWidth()))
			if rescan && !hasBlocked(d.Kinds) && d.Enabled {
				fmt.Fprintln(outputWriter, scanPanel(theme, d.Kinds, min(terminalWidth()-2, 86)))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&rescan, "rescan", false, "Scan again now instead of showing the last scan")
	cmd.Flags().BoolVar(&dismissed, "dismissed", false, "List dismissed suggestions")
	cmd.AddCommand(
		discoverIDCommand(ctx, opts, "ignore", "Never suggest this again (saved to discovery.ignore)", runDiscoverIgnore),
		discoverIDCommand(ctx, opts, "restore", "Suggest a dismissed or ignored item again", runDiscoverRestore),
		discoverIDCommand(ctx, opts, "dismiss", "Hide a suggestion (restore it later)", runDiscoverDismiss),
	)
	return cmd
}

func discoverIDCommand(ctx context.Context, opts *Options, name, short string, run func(context.Context, *Options, string) error) *cobra.Command {
	return &cobra.Command{
		Use: name + " <id>", Short: short, Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "ignore" || name == "restore" {
				if commandNeedsElevation("discover", commandArgs(opts, "discover")) {
					if err := ensurePrivilegeFor(opts, "discover"); err != nil {
						return NewCommandError("could not elevate privileges", err)
					}
				}
			}
			return run(ctx, opts, strings.TrimSpace(args[0]))
		},
	}
}

// loadDiscovery asks the daemon (which remembers choices and what is new). With
// no daemon it scans locally, read-only and without remembering anything.
func loadDiscovery(ctx context.Context, opts *Options, rescan bool) (control.Discovery, string, error) {
	host, _ := os.Hostname()
	c := controlClient(opts)
	if st, err := c.Status(); err == nil {
		if st.Name != "" {
			host = st.Name
		}
		var d control.Discovery
		if rescan {
			d, err = c.Rescan()
		} else if d, err = c.Discovery(); err == nil && d.ScannedAt.IsZero() && d.Enabled {
			d, err = c.Rescan()
		}
		return d, host, err
	}
	cfg, _, err := config.LoadRaw(opts.ConfigPath)
	if err != nil || cfg == nil {
		cfg = &config.UserConfig{}
	}
	if !cfg.Discovery.IsEnabled() {
		return control.Discovery{Enabled: false, Ignore: cfg.Discovery.Ignore}, host, nil
	}
	res := discover.Run(ctx, hostDiscoverEnv(cfg))
	return viewOf(res, cfg), host, nil
}

// viewOf converts a local scan into the same shape the daemon serves.
func viewOf(res discover.Result, cfg *config.UserConfig) control.Discovery {
	d := control.Discovery{Enabled: true, Ignore: cfg.Discovery.Ignore}
	for _, k := range res.Kinds {
		d.Kinds = append(d.Kinds, control.KindState{Kind: k.Kind, State: k.State, Message: k.Message, Fix: k.Fix, Why: k.Why})
	}
	for _, c := range res.Candidates {
		d.Items = append(d.Items, control.DiscoveryItem{ID: c.ID, Kind: string(c.Kind), Name: c.Name, Detail: c.Detail,
			Confidence: string(c.Confidence), Status: c.Status, ManagedIn: c.ManagedIn})
	}
	return d
}

// editConfigFile applies fn to the raw config file and writes it back
// atomically, then tells a running daemon to reload. It never writes defaults.
func editConfigFile(opts *Options, fn func(*config.UserConfig) error) error {
	cfg, _, err := config.LoadRaw(opts.ConfigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "not found") {
			cfg = &config.UserConfig{Interval: config.DefaultInterval}
		} else {
			return err
		}
	}
	if err := fn(cfg); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := writeConfigAtomic(opts.ConfigPath, data); err != nil {
		return fmt.Errorf("%w (re-run with sufficient permissions: sudo orion-agent discover ...)", err)
	}
	handOverToServiceUser(opts.ConfigPath)
	_ = controlClient(opts).Reload()
	return nil
}

func runDiscoverIgnore(_ context.Context, opts *Options, id string) error {
	if !strings.Contains(id, ":") {
		return NewCommandError("that is not a suggestion id", fmt.Errorf("%q must look like kind:name, for example port:5432", id), "orion-agent discover")
	}
	err := editConfigFile(opts, func(c *config.UserConfig) error {
		for _, have := range c.Discovery.Ignore {
			if have == id {
				return nil
			}
		}
		c.Discovery.Ignore = append(c.Discovery.Ignore, id)
		return nil
	})
	if err != nil {
		return NewCommandError("could not save discovery.ignore", err)
	}
	return printResult(opts, "Will not suggest "+id+" again", "saved to discovery.ignore", map[string]any{"ignored": id})
}

func runDiscoverRestore(_ context.Context, opts *Options, id string) error {
	changed := false
	if err := controlClient(opts).DiscoveryAction("restore", id); err == nil {
		changed = true
	}
	err := editConfigFile(opts, func(c *config.UserConfig) error {
		var keep []string
		for _, have := range c.Discovery.Ignore {
			if have == id {
				changed = true
				continue
			}
			keep = append(keep, have)
		}
		c.Discovery.Ignore = keep
		return nil
	})
	if err != nil && !changed {
		return NewCommandError("could not restore "+id, err)
	}
	if !changed {
		return NewCommandError(id+" is not dismissed or ignored", errors.New("nothing to restore"), "orion-agent discover --dismissed")
	}
	return printResult(opts, id+" is a suggestion again", "", map[string]any{"restored": id})
}

func runDiscoverDismiss(_ context.Context, opts *Options, id string) error {
	if err := controlClient(opts).DiscoveryAction("dismiss", id); err != nil {
		return NewCommandError("could not dismiss "+id, err, "orion-agent discover")
	}
	return printResult(opts, id+" is hidden", "restore it with: orion-agent discover restore "+id, map[string]any{"dismissed": id})
}

// printResult prints the one-line outcome the board shows (✓ text  muted note).
func printResult(opts *Options, msg, note string, js map[string]any) error {
	if opts.JSON {
		return renderJSON(outputWriter, js)
	}
	line := "   " + theme.Pass() + " " + msg
	if note != "" {
		line += "  " + theme.Muted(note)
	}
	fmt.Fprintln(outputWriter, line)
	return nil
}
