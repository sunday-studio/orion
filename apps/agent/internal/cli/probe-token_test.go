package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbeTokenUsesTheAgentRouteAndFallsBackOnAnOlderCore(t *testing.T) {
	var paths []string
	newCore := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer tok" || r.URL.Path != "/v1/agents/ag/token-check" {
			w.WriteHeader(401) // an admin route would refuse an agent token
			return
		}
		w.WriteHeader(200)
	}))
	defer newCore.Close()
	if code, err := probeToken(context.Background(), newCore.URL, "ag", "tok"); err != nil || code != 200 {
		t.Fatalf("new Core: %d %v", code, err)
	}
	if code, _ := probeToken(context.Background(), newCore.URL, "ag", "wrong"); code != 401 {
		t.Fatalf("a wrong token must still be refused, got %d", code)
	}

	paths = nil
	oldCore := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/v1/agents/ag/monitors" {
			w.WriteHeader(200)
			return
		}
		http.NotFound(w, r)
	}))
	defer oldCore.Close()
	if code, err := probeToken(context.Background(), oldCore.URL, "ag", "tok"); err != nil || code != 200 {
		t.Fatalf("older Core: %d %v", code, err)
	}
	if len(paths) != 2 || paths[0] != "POST /v1/agents/ag/token-check" || paths[1] != "GET /v1/agents/ag/monitors" {
		t.Fatalf("paths = %v", paths)
	}
}
