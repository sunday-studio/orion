package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite golden --json schema files")

// shapeOf reduces a JSON document to its keys and value types, so goldens pin
// the schema without depending on the host (paths, service state, versions).
func shapeOf(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range t {
			out[k] = shapeOf(val)
		}
		return out
	case []any:
		if len(t) == 0 {
			return []any{}
		}
		return []any{shapeOf(t[0])}
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	}
	return "unknown"
}

func goldenCheck(t *testing.T, name string, out []byte) {
	t.Helper()
	var doc any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("%s: output is not JSON: %v\n%s", name, err, out)
	}
	if m, ok := doc.(map[string]any); !ok || m["schema_version"] == nil {
		t.Fatalf("%s: JSON object must carry schema_version: %s", name, out)
	}
	got, _ := json.MarshalIndent(shapeOf(doc), "", "  ")
	path := filepath.Join("testdata", "golden", name+".json")
	if *updateGolden {
		os.WriteFile(path, append(got, '\n'), 0o644)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: missing golden (run go test ./internal/cli -run Golden -update): %v", name, err)
	}
	if strings.TrimSpace(string(want)) != strings.TrimSpace(string(got)) {
		keys := func(b []byte) []string { var m map[string]any; json.Unmarshal(b, &m); k := []string{}; for x := range m { k = append(k, x) }; sort.Strings(k); return k }
		t.Fatalf("%s: --json schema changed.\nwant keys %v\n got keys %v\nfull got:\n%s\n(rerun with -update if intentional, and bump schema_version for breaking changes)", name, keys(want), keys(got), got)
	}
}

func runJSON(t *testing.T, args ...string) []byte {
	t.Helper()
	var out, errOut bytes.Buffer
	Execute(context.Background(), append([]string{"--json", "--no-color"}, args...), &out, &errOut)
	return out.Bytes()
}

func TestGoldenJSONSchemas(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfg, []byte("core_url: https://core.example.com\ninterval: 60s\nmonitors: []\n"), 0o600)
	base := []string{"--config", cfg, "--state", filepath.Join(dir, "state.db"), "--socket", filepath.Join(dir, "none.sock")}
	with := func(rest ...string) []string { return append(append([]string{}, base...), rest...) }

	goldenCheck(t, "version", runJSON(t, "version"))
	goldenCheck(t, "status", runJSON(t, with("status")...))
	goldenCheck(t, "doctor", runJSON(t, with("doctor")...))
	goldenCheck(t, "config-show", runJSON(t, with("config", "show")...))
	goldenCheck(t, "config-validate", runJSON(t, with("config", "validate")...))
	goldenCheck(t, "config-diff", runJSON(t, with("config", "diff")...))

	// enroll against a stub Core
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":true,"data":{"agent_id":"a","token":"t"}}`))
	}))
	defer core.Close()
	goldenCheck(t, "enroll", runJSON(t, with("enroll", "--core-url", core.URL, "--token", "x")...))

	// socket-backed commands
	sock, _ := startDaemonControl(t)
	live := []string{"--config", cfg, "--state", filepath.Join(dir, "missing.db"), "--socket", sock}
	goldenCheck(t, "maintenance", runJSON(t, append(append([]string{}, live...), "maintenance", "down", "why")...))
	goldenCheck(t, "status-daemon", runJSON(t, append(append([]string{}, live...), "status")...))
}
