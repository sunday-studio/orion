package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"orion/agent/internal/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type configShowPayload struct {
	configSummaryPayload
	Source config.Source `json:"source"`
}

// runConfigShow prints the effective config (file + drop-ins + env) and where
// each layer came from.
func runConfigShow(_ context.Context, opts *Options) error {
	cfg, src, err := config.Load(opts.ConfigPath)
	if err != nil {
		return NewCommandError("could not load config", err)
	}
	if opts.JSON {
		return renderJSON(outputWriter, configShowPayload{configSummaryJSON(opts.ConfigPath, cfg), src})
	}
	PrintHeader("config show")
	describeSource(src)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return NewCommandError("could not render config", err)
	}
	fmt.Fprintln(outputWriter)
	fmt.Fprint(outputWriter, string(data))
	return nil
}

func describeSource(src config.Source) {
	switch {
	case src.ZeroConf:
		PrintInfo("source", "environment only (no config file)")
	case src.FileFound:
		PrintInfo("config", src.File)
	}
	for _, p := range src.DropIns {
		PrintInfo("drop-in", p)
	}
	if len(src.EnvKeys) > 0 {
		PrintInfo("env overrides", strings.Join(src.EnvKeys, ", "))
	}
}

// runConfigDiff compares the effective config with another config file, or,
// with no argument, with the base file alone (showing what drop-ins and env
// change).
func runConfigDiffAgainst(_ context.Context, opts *Options, other string) error {
	effective, _, err := config.Load(opts.ConfigPath)
	if err != nil {
		return NewCommandError("could not load config", err)
	}
	var baseline *config.UserConfig
	label := "base file only"
	if other != "" {
		baseline, _, err = config.Load(other)
		label = other
	} else {
		baseline, err = baseFileOnly(opts.ConfigPath)
	}
	if err != nil {
		return NewCommandError("could not load comparison config", err)
	}
	a, _ := yaml.Marshal(baseline)
	b, _ := yaml.Marshal(effective)
	diff := lineDiff(strings.Split(strings.TrimRight(string(a), "\n"), "\n"), strings.Split(strings.TrimRight(string(b), "\n"), "\n"))
	if opts.JSON {
		return renderJSON(outputWriter, map[string]any{"against": label, "identical": len(diff) == 0, "changes": diff})
	}
	PrintHeader("config diff")
	PrintInfo("against", label)
	if len(diff) == 0 {
		PrintOK("no differences")
		return nil
	}
	for _, l := range diff {
		fmt.Fprintln(outputWriter, l)
	}
	return nil
}

// baseFileOnly loads just the main file by pointing at a directory without
// drop-ins is not possible, so decode through a copy in a scratch directory.
func baseFileOnly(path string) (*config.UserConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "orion-config-base")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	tmp := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return nil, err
	}
	cfg, _, err := config.Load(tmp)
	return cfg, err
}

// lineDiff returns "-"/"+" prefixed lines using a simple LCS.
func lineDiff(a, b []string) []string {
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []string
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, "- "+a[i])
			i++
		default:
			out = append(out, "+ "+b[j])
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, "- "+a[i])
	}
	for ; j < m; j++ {
		out = append(out, "+ "+b[j])
	}
	return out
}

// runConfigEdit opens the config in $EDITOR on a scratch copy, validates the
// result, and only then replaces the real file.
func runConfigEdit(_ context.Context, opts *Options) error {
	editor := firstNonEmpty(os.Getenv("VISUAL"), os.Getenv("EDITOR"), "vi")
	original, err := os.ReadFile(opts.ConfigPath)
	if err != nil {
		return NewCommandError("could not read config", err, "create one with: orion-agent setup")
	}
	dir, err := os.MkdirTemp("", "orion-config-edit")
	if err != nil {
		return NewCommandError("could not create scratch copy", err)
	}
	defer os.RemoveAll(dir)
	tmp := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(tmp, original, 0o600); err != nil {
		return NewCommandError("could not create scratch copy", err)
	}
	parts := strings.Fields(editor)
	cmd := exec.Command(parts[0], append(parts[1:], tmp)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return NewCommandError("editor exited with an error", err)
	}
	edited, err := os.ReadFile(tmp)
	if err != nil {
		return NewCommandError("could not read edited config", err)
	}
	if string(edited) == string(original) {
		PrintSkip("no changes")
		return nil
	}
	if _, _, err := config.Load(tmp); err != nil {
		return NewCommandError("edited config is invalid; nothing was saved", err, "run `orion-agent config edit` to try again")
	}
	if err := os.WriteFile(opts.ConfigPath, edited, 0o640); err != nil {
		return NewCommandError("could not save config", err, "re-run with sufficient permissions (sudo orion-agent config edit)")
	}
	PrintOK("config saved")
	if err := controlClient(opts).Reload(); err != nil {
		PrintInfo("next", "orion-agent restart")
	} else {
		PrintOK("daemon reloaded the config")
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func newConfigEditCommand(ctx context.Context, opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit the config in $EDITOR with validation before saving",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return runConfigEdit(ctx, opts) },
	}
}
