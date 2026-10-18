package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type fakeCore struct {
	mu     sync.Mutex
	bodies []string
	auth   []string
	hb     []string
	srv    *httptest.Server
}

func newFakeCore(t *testing.T) *fakeCore {
	f := &fakeCore{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if strings.HasPrefix(r.URL.Path, "/v1/heartbeats/") {
			f.hb = append(f.hb, r.URL.Path)
			w.WriteHeader(200)
			return
		}
		b, _ := io.ReadAll(r.Body)
		f.bodies = append(f.bodies, string(b))
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		w.WriteHeader(202)
		w.Write([]byte(`{"accepted":1,"rejected":0}`))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), append([]string{"--no-color"}, args...), &out, &errOut)
	return out.String(), errOut.String(), code
}

func TestEmitSendsOneEntryWithIngestKey(t *testing.T) {
	core := newFakeCore(t)
	key := filepath.Join(t.TempDir(), "key")
	os.WriteFile(key, []byte("orion_ik_test\n"), 0o600)
	_, errOut, code := run(t, "emit", "--endpoint", core.srv.URL+"/v1/ingest/logs", "--key-file", key,
		"--severity", "error", "--event", "backup.failed", "--attr", "host=db1", "--app", "backup", "backup failed")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
	body := core.bodies[0]
	for _, want := range []string{`"severity":"error"`, `"event":"backup.failed"`, `"body":"backup failed"`, `"host":"db1"`, `"service.name":"backup"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %s: %s", want, body)
		}
	}
	if core.auth[0] != "Bearer orion_ik_test" || strings.Contains(body, "_source") {
		t.Fatalf("auth=%q body=%s", core.auth[0], body)
	}
}

func TestEmitExplainsMissingKeyAndBadInput(t *testing.T) {
	t.Setenv("ORION_INGEST_KEY", "")
	t.Setenv("ORION_CORE_URL", "")
	_, errOut, code := run(t, "--config", filepath.Join(t.TempDir(), "none.yaml"), "emit", "--endpoint", "http://127.0.0.1:1/x", "hello")
	if code == 0 || !strings.Contains(strings.Join(strings.Fields(errOut), " "), "no ingest key") {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
	_, errOut, code = run(t, "emit", "--severity", "loud", "--endpoint", "http://x", "--key-file", "/nonexistent", "hi")
	if code == 0 {
		t.Fatalf("a bad severity or key file must fail: %s", errOut)
	}
}

func TestRunJobShipsOutputFailureAndHeartbeat(t *testing.T) {
	core := newFakeCore(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(cfg, []byte("core_url: "+core.srv.URL+"\ninterval: 60s\n"), 0o600)
	key := filepath.Join(t.TempDir(), "key")
	os.WriteFile(key, []byte("orion_ik_job"), 0o600)

	out, _, code := run(t, "--config", cfg, "run-job", "--name", "nightly", "--key-file", key, "--heartbeat-token", "tok123", "--", "sh", "-c", "echo hello; echo oops 1>&2; exit 3")
	if code != 3 {
		t.Fatalf("exit code must pass through: got %d", code)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("job stdout should still reach the caller: %q", out)
	}
	body := core.bodies[0]
	for _, want := range []string{`"event":"job.output"`, `"body":"hello"`, `"severity":"warn"`, `"event":"job.failed"`, `"exit_code":3`, `"job":"nightly"`} {
		if !strings.Contains(body, want) {
			t.Errorf("shipped entries missing %s:\n%s", want, body)
		}
	}
	if len(core.hb) != 1 || core.hb[0] != "/v1/heartbeats/tok123/failure" {
		t.Fatalf("a failed job must ping the failure heartbeat: %v", core.hb)
	}

	_, _, code = run(t, "--config", cfg, "run-job", "--name", "nightly", "--key-file", key, "--heartbeat-token", "tok123", "--", "true")
	if code != 0 || core.hb[len(core.hb)-1] != "/v1/heartbeats/tok123/success" || !strings.Contains(core.bodies[len(core.bodies)-1], `"event":"job.finished"`) {
		t.Fatalf("success path: code=%d hb=%v", code, core.hb)
	}
}

func TestRunJobStillRunsWhenCoreIsUnreachable(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(cfg, []byte("core_url: http://127.0.0.1:1\ninterval: 60s\n"), 0o600)
	t.Setenv("ORION_INGEST_KEY", "k")
	out, errOut, code := run(t, "--config", cfg, "run-job", "--name", "x", "--", "echo", "still-ran")
	if code != 0 || !strings.Contains(out, "still-ran") || !strings.Contains(errOut, "not shipped") {
		t.Fatalf("a monitoring outage must not break the job: code=%d out=%q err=%q", code, out, errOut)
	}
}
