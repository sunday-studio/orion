package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newDiscoverCommand(ctx context.Context, opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "List services, containers, ports and log files worth monitoring on this host",
		Long:  "Read-only. Each row has an id you can pass to `orion-agent setup --monitor <id>`. Confidence is high for well-known apps and managed workloads.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			res := discoverHost(ctx)
			if opts.JSON {
				return renderJSON(outputWriter, res)
			}
			PrintHeader("discover")
			rows := make([][]string, 0, len(res.Candidates))
			for _, c := range res.Candidates {
				rows = append(rows, []string{c.ID, string(c.Confidence), c.Detail})
			}
			if len(rows) == 0 {
				PrintSkip("nothing found; host metrics still report")
			} else {
				fmt.Fprintln(outputWriter, theme.Table([]string{"ID", "Confidence", "Detail"}, rows, min(terminalWidth(), 100)))
			}
			for _, n := range res.Notes {
				PrintSkip(n)
			}
			return nil
		},
	}
}
