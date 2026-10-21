package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSendDiscoveryPostsTheScanWithTheAgentToken(t *testing.T) {
	var got DiscoveryReport
	var auth, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path = r.Header.Get("Authorization"), r.URL.Path
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()
	rep := DiscoveryReport{ScannedAt: time.Now().UTC(), Enabled: true,
		Kinds: []DiscoveredKind{{Kind: "containers", State: "blocked", Fix: "sudo usermod -aG docker orion"}},
		Items: []DiscoveredItem{{ID: "service:nginx", Kind: "service", Name: "nginx", Confidence: "high", Status: "suggested", New: true}}}
	if err := NewClient(srv.URL, "tok").SendDiscovery("agent-1", rep); err != nil {
		t.Fatal(err)
	}
	if path != "/v1/agents/agent-1/discovery" || auth != "Bearer tok" || len(got.Items) != 1 || got.Kinds[0].Fix == "" {
		t.Fatalf("path=%q auth=%q body=%+v", path, auth, got)
	}
}

func TestSendDiscoveryToAnOlderCoreIsRecognisedNotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; http.NotFound(w, r) }))
	defer srv.Close()
	err := NewClient(srv.URL, "tok").SendDiscovery("agent-1", DiscoveryReport{})
	if !errors.Is(err, ErrDiscoveryUnsupported) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
