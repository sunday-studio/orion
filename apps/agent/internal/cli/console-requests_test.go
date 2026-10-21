package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orion/agent/internal/config"
	"orion/agent/internal/transport"
)

type appliedRequest struct{ id, status, text string }

func newApplierFixture(t *testing.T, configYAML string, queued ...transport.PulledRequest) (*requestApplier, *daemonControl, *[]appliedRequest, *int, string) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(configYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	d, store := newTestDaemon(t, &config.UserConfig{})
	d.scan.env = scanFixture("myapp")
	var results []appliedRequest
	reloads := 0
	a := newRequestApplier(store, cfgPath, d.scan, func() error { reloads++; return nil })
	a.pull = func(*config.InternalState) ([]transport.PulledRequest, error) { return queued, nil }
	a.report = func(_ *config.InternalState, id, status, text string) error {
		results = append(results, appliedRequest{id, status, text})
		return nil
	}
	return a, d, &results, &reloads, cfgPath
}

func TestConsoleRequestsEditTheConfigReloadAndAnswerInTheAgentsWords(t *testing.T) {
	a, d, _, reloads, cfgPath := newApplierFixture(t, "# mine\ndiscovery:\n  report: true\n")
	ctx := context.Background()
	cases := []struct {
		req    transport.PulledRequest
		status string
		text   string
	}{
		{transport.PulledRequest{ID: "1", Type: "ignore", Payload: map[string]any{"ids": []any{"port:5432"}}}, "applied", "Added 1 item(s)"},
		{transport.PulledRequest{ID: "2", Type: "ignore", Payload: map[string]any{"ids": []any{"port:5432"}}}, "applied", "already so"},
		{transport.PulledRequest{ID: "3", Type: "set_kind", Payload: map[string]any{"kind": "logs", "enabled": false}}, "applied", "logs turned off"},
		{transport.PulledRequest{ID: "4", Type: "set_scanning", Payload: map[string]any{"enabled": false}}, "applied", "Scanning turned off"},
		{transport.PulledRequest{ID: "5", Type: "track_storage", Payload: map[string]any{"paths": []any{"/var/lib/app"}}}, "applied", "1 path"},
		{transport.PulledRequest{ID: "6", Type: "set_kind", Payload: map[string]any{"kind": "nope", "enabled": true}}, "failed", "set_kind needs"},
		{transport.PulledRequest{ID: "7", Type: "create_log_source"}, "unsupported", "update the agent"},
	}
	for _, c := range cases {
		status, text := a.apply(ctx, c.req)
		if status != c.status || !strings.Contains(text, c.text) {
			t.Errorf("%s %s: got %s %q, want %s containing %q", c.req.ID, c.req.Type, status, text, c.status, c.text)
		}
	}
	if *reloads != 4 {
		t.Errorf("reloads = %d, want 4 (the repeated ignore changed nothing, the failures never reached the file)", *reloads)
	}
	got, _ := os.ReadFile(cfgPath)
	for _, want := range []string{"# mine", "port:5432", "/var/lib/app", "enabled: false"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("config lost %q:\n%s", want, got)
		}
	}
	if !d.scan.rescanOnConfig.Load() {
		t.Error("a config change should make the next reload scan again")
	}
}

func TestConsoleRescanAndDismissRunAgainstTheScan(t *testing.T) {
	a, d, _, _, _ := newApplierFixture(t, "")
	ctx := context.Background()
	if status, text := a.apply(ctx, transport.PulledRequest{ID: "1", Type: "rescan"}); status != "applied" || !strings.Contains(text, "2 item") {
		t.Fatalf("rescan = %s %q", status, text)
	}
	if status, text := a.apply(ctx, transport.PulledRequest{ID: "2", Type: "dismiss", Payload: map[string]any{"ids": []any{"service:nginx"}}}); status != "applied" {
		t.Fatalf("dismiss = %s %q", status, text)
	}
	v, _ := d.scan.View()
	dismissed := 0
	for _, it := range v.Items {
		if it.Dismissed {
			dismissed++
		}
	}
	if len(v.Items) != 2 || dismissed != 1 {
		t.Fatalf("view = %+v", v.Items)
	}
	if status, text := a.apply(ctx, transport.PulledRequest{ID: "3", Type: "dismiss", Payload: map[string]any{"ids": []any{"service:ghost"}}}); status != "failed" || !strings.Contains(text, "service:ghost") {
		t.Fatalf("unknown id = %s %q", status, text)
	}
}

func TestPollAnswersEveryPulledRequestAndSkipsWhenNotEnrolled(t *testing.T) {
	a, _, results, _, _ := newApplierFixture(t, "", transport.PulledRequest{ID: "1", Type: "rescan"}, transport.PulledRequest{ID: "2", Type: "bogus"})
	if err := a.poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*results) != 0 {
		t.Fatalf("an agent that is not enrolled has nobody to ask: %+v", *results)
	}
}
