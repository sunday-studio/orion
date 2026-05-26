package cli

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeLegacyArgsRewritesSingleDashLongFlags(t *testing.T) {
	t.Parallel()

	got := NormalizeLegacyArgs([]string{"run", "-once", "-config", "/tmp/config.yaml", "-state=/tmp/state.db"})
	want := []string{"run", "--once", "--config", "/tmp/config.yaml", "--state=/tmp/state.db"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeLegacyArgs() = %#v, want %#v", got, want)
	}
}

func TestNormalizeLegacyArgsRewritesVersionShortcut(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"-v"}, {"-version"}, {"--version"}} {
		got := NormalizeLegacyArgs(args)
		want := []string{"version"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("NormalizeLegacyArgs(%#v) = %#v, want %#v", args, got, want)
		}
	}
}

func TestNormalizeLegacyArgsRewritesMaintenanceDashActions(t *testing.T) {
	t.Parallel()

	got := NormalizeLegacyArgs([]string{"maintenance", "-down", "deploying", "-state", "/tmp/state.db"})
	want := []string{"maintenance", "down", "deploying", "--state", "/tmp/state.db"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeLegacyArgs() = %#v, want %#v", got, want)
	}
}

func TestCommandArgsIncludesGlobalFlagsBeforeCommand(t *testing.T) {
	t.Parallel()

	opts := &Options{normalizedArgs: []string{"--state", "/tmp/state.db", "maintenance", "up"}}

	got := commandArgs(opts, "maintenance")
	want := []string{"--state", "/tmp/state.db", "up"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commandArgs() = %#v, want %#v", got, want)
	}
	if commandNeedsElevation("maintenance", got) {
		t.Fatal("commandNeedsElevation() = true, want false with global --state")
	}
}

func TestNewRootCommandParsesGlobalFlagsBeforeCommand(t *testing.T) {
	t.Parallel()

	opts := &Options{}
	cmd := NewRootCommand(context.Background(), opts, nil, nil)
	cmd.SetArgs([]string{"--state", "/tmp/state.db", "version"})
	if err := cmd.ParseFlags([]string{"--state", "/tmp/state.db"}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}
	if opts.StatePath != "/tmp/state.db" {
		t.Fatalf("StatePath = %q, want /tmp/state.db", opts.StatePath)
	}
}

func TestExecuteDisablesColorForNonFileWriters(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Execute(context.Background(), []string{"config", "validate", "--config", "/tmp/missing-orion-config.yaml"}, &out, &errOut)
	if code == 0 {
		t.Fatal("Execute() code = 0, want failure for missing config")
	}
	combined := out.String() + errOut.String()
	if strings.Contains(combined, "\x1b[") {
		t.Fatalf("Execute() output contains ANSI escapes for buffer writer: %q", combined)
	}
}
