package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDiffShowsDropInOverride(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfg, []byte("core_url: https://a.example.com\ninterval: 30s\nmonitors: []\n"), 0o600)
	os.MkdirAll(filepath.Join(dir, "config.d"), 0o755)
	os.WriteFile(filepath.Join(dir, "config.d", "10.yaml"), []byte("interval: 90s\n"), 0o600)

	var out, errOut bytes.Buffer
	code := Execute(context.Background(), []string{"--config", cfg, "--no-color", "config", "diff"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "- interval: 30s") || !strings.Contains(out.String(), "+ interval: 90s") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
}

func TestConfigShowListsSources(t *testing.T) {
	t.Setenv("ORION_LOG_LEVEL", "debug")
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfg, []byte("core_url: https://a.example.com\ninterval: 30s\n"), 0o600)
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), []string{"--config", cfg, "--no-color", "config", "show"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "ORION_LOG_LEVEL") || !strings.Contains(out.String(), "level: debug") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
}

func TestLineDiffIdentical(t *testing.T) {
	if d := lineDiff([]string{"a", "b"}, []string{"a", "b"}); len(d) != 0 {
		t.Fatalf("diff = %v", d)
	}
}
