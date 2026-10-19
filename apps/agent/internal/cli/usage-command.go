package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"orion/agent/internal/appres"
	"orion/agent/internal/config"

	"github.com/spf13/cobra"
)

func newUsageCommand(ctx context.Context, opts *Options) *cobra.Command {
	var window time.Duration
	var top int
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Show what apps on this host cost right now (CPU, memory, disk I/O, network)",
		Long: "Measured from the outside, with no app integration: processes, systemd units and containers. " +
			"Takes two samples a few seconds apart to compute rates. This is what the agent reports to Core.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if window < 0 {
				return NewCommandError("invalid --window", fmt.Errorf("duration cannot be negative"))
			}
			if top <= 0 {
				return NewCommandError("invalid --top", fmt.Errorf("must be greater than zero"))
			}
			var paths []string
			if cfg, _, err := config.Load(opts.ConfigPath); err == nil {
				paths = cfg.AppResources.Paths
			}
			s := appres.NewSampler(appres.Options{TopN: top, Paths: paths})
			s.Sample()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(window):
			}
			rep := s.Sample()
			if opts.JSON {
				return renderJSON(outputWriter, rep)
			}
			PrintHeader("usage")
			PrintInfo("window", fmt.Sprintf("%.1fs", rep.IntervalSeconds))
			sort.SliceStable(rep.Series, func(i, j int) bool { return cpuOf(rep.Series[i]) > cpuOf(rep.Series[j]) })
			rows := make([][]string, 0, len(rep.Series))
			for _, s := range rep.Series {
				rows = append(rows, []string{string(s.Kind), s.Name, pct(s.CPUPercent), bytesStr(s.MemoryBytes), rate(s.IOReadBps, s.IOWriteBps), rate(s.NetRxBps, s.NetTxBps), bytesStr(s.StorageBytes)})
			}
			fmt.Fprintln(outputWriter, theme.Table([]string{"Kind", "Name", "CPU", "Memory", "Disk r/w /s", "Net rx/tx /s", "Storage"}, rows, min(terminalWidth(), 110)))
			for _, n := range rep.Notes {
				PrintSkip(n)
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&window, "window", 3*time.Second, "Time between the two samples")
	cmd.Flags().IntVar(&top, "top", 10, "Processes shown per ranking (CPU and memory)")
	return cmd
}

func cpuOf(s appres.Series) float64 {
	if s.CPUPercent == nil {
		return -1
	}
	return *s.CPUPercent
}

func pct(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", *v)
}

func bytesStr(v *uint64) string {
	if v == nil {
		return "-"
	}
	return humanBytes(float64(*v))
}

func rate(a, b *float64) string {
	if a == nil || b == nil {
		return "-"
	}
	return humanBytes(*a) + " / " + humanBytes(*b)
}

func humanBytes(n float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for n >= 1024 && i < len(units)-1 {
		n /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", n, units[i])
	}
	return fmt.Sprintf("%.1f %s", n, units[i])
}
