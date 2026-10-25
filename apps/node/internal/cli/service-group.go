package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// newServiceCommand groups everything that manages the installed service.
// Top-level start/stop/restart remain available for the direct workflow shown
// on the command map; the grouped forms are convenient discoverable aliases.
func newServiceCommand(ctx context.Context, opts *Options) *cobra.Command {
	svc := newGroupCommand("service", "Install, start, stop and inspect the Orion Node service")
	install := &cobra.Command{
		Use:   "install",
		Short: "Show how to install the service on this host",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(outputWriter, installHint(theme, false))
			return nil
		},
	}
	uninstall := &cobra.Command{
		Use:   "uninstall",
		Short: "Show how to uninstall the service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(outputWriter, installHint(theme, true))
			return nil
		},
	}
	svc.AddCommand(
		install,
		uninstall,
		newStartCommand(ctx, opts),
		newStopCommand(ctx, opts),
		newRestartCommand(ctx, opts),
		newStatusCommand(ctx, opts),
	)
	return svc
}
