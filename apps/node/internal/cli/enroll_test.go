package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnrollRegistersWithTokenHeader(t *testing.T) {
	var gotToken string
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Orion-Enrollment-Token")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":{"node_id":"ag-9","token":"node-secret"}}`))
	}))
	defer core.Close()

	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "enroll-token")
	os.WriteFile(tokenFile, []byte("one-time-123\n"), 0o600)
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), []string{
		"--json", "--config", filepath.Join(dir, "config.yaml"), "--state", filepath.Join(dir, "state.db"), "--socket", filepath.Join(dir, "s"),
		"enroll", "--core-url", core.URL, "--token-file", tokenFile,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code=%d err=%s out=%s", code, errOut.String(), out.String())
	}
	if gotToken != "one-time-123" {
		t.Fatalf("enrollment header = %q", gotToken)
	}
	var res map[string]any
	if err := json.Unmarshal(out.Bytes(), &res); err != nil || res["node_id"] != "ag-9" {
		t.Fatalf("res=%v err=%v out=%s", res, err, out.String())
	}
	if strings.Contains(out.String()+errOut.String(), "one-time-123") || strings.Contains(out.String(), "node-secret") {
		t.Fatal("secrets leaked to output")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatal("starter config not written")
	}
}

func TestEnrollRejectedTokenExplainsFix(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer core.Close()
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), []string{
		"--config", filepath.Join(dir, "c.yaml"), "--state", filepath.Join(dir, "s.db"), "--no-color",
		"enroll", "--core-url", core.URL, "--token", "bad",
	}, &out, &errOut)
	if code == 0 || !strings.Contains(errOut.String()+out.String(), "Add server") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
}
