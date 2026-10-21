package transport

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPullRequestsAndSendResultsUseTheAgentToken(t *testing.T) {
	var paths []string
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			return
		}
		if r.URL.Path == "/v1/agents/ag/discovery/requests/pull" {
			w.Write([]byte(`{"success":true,"data":{"requests":[{"id":"r1","type":"ignore","payload":{"ids":["port:5432"]}}]}}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &posted)
		w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "tok")
	reqs, err := c.PullRequests("ag")
	if err != nil || len(reqs) != 1 || reqs[0].Type != "ignore" || reqs[0].Payload["ids"] == nil {
		t.Fatalf("pull = %+v %v", reqs, err)
	}
	if err := c.SendRequestResult("ag", "r1", "failed", "no such id"); err != nil || posted["status"] != "failed" || posted["result"] != "no such id" {
		t.Fatalf("result = %v %v", posted, err)
	}
	if err := c.SendDefinitionResults("ag", []DefinitionResult{{DefinitionID: "d1", OK: false, Reason: "no docker"}}); err != nil || posted["results"] == nil {
		t.Fatalf("definition results = %v %v", posted, err)
	}
	if paths[1] != "/v1/agents/ag/discovery/requests/r1/result" || paths[2] != "/v1/agents/ag/monitor-definitions/results" {
		t.Fatalf("paths = %v", paths)
	}
}

func TestPullRequestsFromAnOlderCoreIsRecognised(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(http.NotFound))
	defer srv.Close()
	if _, err := NewClient(srv.URL, "tok").PullRequests("ag"); !errors.Is(err, ErrDiscoveryUnsupported) {
		t.Fatalf("err = %v", err)
	}
}
