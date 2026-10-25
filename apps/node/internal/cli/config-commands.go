package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"orion/node/internal/config"

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
	fmt.Fprint(outputWriter, configShowScreen(theme, cfg, src))
	return nil
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
	for i, l := range diff {
		if len(l) > 2 {
			diff[i] = l[:2] + config.RedactLine(l[2:]) // never print a credential value
		}
	}
	if opts.JSON {
		return renderJSON(outputWriter, map[string]any{"against": label, "identical": len(diff) == 0, "changes": diff})
	}
	var kept []string
	for _, l := range diff {
		if strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "+ ") {
			kept = append(kept, l)
		}
	}
	fmt.Fprint(outputWriter, configDiffScreen(theme, label, kept, other == ""))
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

// lineDiff returns a compact unified diff using a simple LCS. Two unchanged
// context lines surround each change so the output still answers where a
// changed setting lives without dumping an unrelated long configuration.
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
	var all []string
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			all = append(all, "  "+a[i])
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			all = append(all, "- "+a[i])
			i++
		default:
			all = append(all, "+ "+b[j])
			j++
		}
	}
	for ; i < n; i++ {
		all = append(all, "- "+a[i])
	}
	for ; j < m; j++ {
		all = append(all, "+ "+b[j])
	}
	changed := make([]bool, len(all))
	hasChanges := false
	for i, line := range all {
		changed[i] = strings.HasPrefix(line, "+ ") || strings.HasPrefix(line, "- ")
		hasChanges = hasChanges || changed[i]
	}
	if !hasChanges {
		return nil
	}
	var out []string
	skipped := false
	for i, line := range all {
		near := changed[i]
		for j := max(0, i-2); !near && j <= min(len(all)-1, i+2); j++ {
			near = changed[j]
		}
		if near {
			if skipped {
				out = append(out, "  …")
				skipped = false
			}
			out = append(out, line)
		} else {
			skipped = true
		}
	}
	return out
}

// runConfigEdit opens the config in $EDITOR on a scratch copy, validates the
// result, shows what changed, and only then replaces the real file. On a
// terminal it asks first (save, edit again, discard).
func runConfigEdit(_ context.Context, opts *Options) error {
	editor := firstNonEmpty(os.Getenv("VISUAL"), os.Getenv("EDITOR"), "vi")
	original, err := os.ReadFile(opts.ConfigPath)
	if err != nil {
		return NewCommandError("could not read config", err, "create one with: orion-node setup")
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
	interactive := canPromptSecret(opts)
	parts := strings.Fields(editor)
	for {
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
			fmt.Fprint(outputWriter, "\n"+credNote(theme, "No changes", "the file was left as it was")+"\n")
			return nil
		}
		fmt.Fprint(outputWriter, "\n"+credStep(theme, "Opened in $EDITOR", "")+"\n")
		if _, _, err := config.Load(tmp); err != nil {
			return NewCommandError("edited config is invalid; nothing was saved", err, "orion-node config edit")
		}
		fmt.Fprint(outputWriter, credStep(theme, "Valid", "")+"\n\n "+theme.Muted("Review")+"\n"+reviewHunks(theme, original, edited))
		if interactive {
			choice, err := chooseAction(theme, os.Stdin, outputWriter, "Save and reload the running node?", "", []string{"Save", "Edit again", "Discard"}, "")
			if err != nil || choice == 2 || choice < 0 {
				fmt.Fprint(outputWriter, "\n"+credNote(theme, "Discarded", "nothing was saved")+"\n")
				return nil
			}
			if choice == 1 {
				continue
			}
		}
		if err := os.WriteFile(opts.ConfigPath, edited, 0o640); err != nil {
			return NewCommandError("could not save config", err, "sudo orion-node config edit")
		}
		detail := "daemon reloaded the config"
		if err := controlClient(opts).Reload(); err != nil {
			detail = "restart the node to apply it: orion-node restart"
		}
		fmt.Fprint(outputWriter, "\n"+credStep(theme, "Saved", "· "+detail)+"\n")
		return nil
	}
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
