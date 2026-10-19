package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// hidden keeps a legacy top-level command working (for one release) without
// listing it in --help.
func hidden(c *cobra.Command) *cobra.Command {
	c.Hidden = true
	return c
}

// newServiceCommand groups everything that manages the installed service.
// Legacy top-level start/stop/restart still work as hidden aliases.
func newServiceCommand(ctx context.Context, opts *Options) *cobra.Command {
	svc := newGroupCommand("service", "Install, start, stop and inspect the Orion Agent service")
	install := &cobra.Command{
		Use:   "install",
		Short: "Show how to install the service on this host",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(outputWriter, "The installer sets up the binary, the unprivileged orion user, the service and enrollment:")
			fmt.Fprintln(outputWriter, "  curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- --core-url <url> --enrollment-token-file <file>")
			fmt.Fprintln(outputWriter, "Add --user for a per-user install without root; add --dry-run to preview.")
			return nil
		},
	}
	uninstall := &cobra.Command{
		Use:   "uninstall",
		Short: "Show how to uninstall the service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(outputWriter, "  curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- uninstall   (add --purge to remove config, state and the service user)")
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
