package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func definitionsOf(t *testing.T, server *Server, nodeID string) map[string]NodeMonitorDefinitionResponse {
	t.Helper()
	var list struct {
		Data struct {
			Definitions []NodeMonitorDefinitionResponse `json:"definitions"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/nodes/"+nodeID+"/monitor-definitions", nil, ""), &list)
	out := map[string]NodeMonitorDefinitionResponse{}
	for _, d := range list.Data.Definitions {
		out[d.Name] = d
	}
	return out
}

func TestAcceptTakesAnIntervalAndFailAfter(t *testing.T) {
	server := setupTestServer(t)
	node := registerTestNode(t, server)
	id := node.Data.NodeID
	report := discoveryBody(
		map[string]any{"id": "service:nginx", "kind": "service", "name": "nginx", "confidence": "high", "status": "suggested"},
		map[string]any{"id": "service:redis", "kind": "service", "name": "redis", "confidence": "high", "status": "suggested"},
	)
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/nodes/"+id+"/discovery", report, node.Data.Token); r.Code != http.StatusOK {
		t.Fatalf("report = %d", r.Code)
	}
	accept := "/v1/nodes/" + id + "/discovery/accept"
	if r := performJSONRequest(t, server, http.MethodPost, accept, gin.H{"item_ids": []string{"service:nginx"}, "interval_seconds": 5}, ""); r.Code != http.StatusBadRequest {
		t.Fatalf("interval below 10 = %d", r.Code)
	}
	if r := performJSONRequest(t, server, http.MethodPost, accept, gin.H{"item_ids": []string{"service:nginx"}, "interval_seconds": 120, "fail_after": 4}, ""); r.Code != http.StatusOK {
		t.Fatalf("accept = %d %s", r.Code, r.Body.String())
	}
	if d := definitionsOf(t, server, id)["nginx"]; d.IntervalSeconds != 120 || d.FailAfter != 4 {
		t.Fatalf("nginx = %+v", d)
	}
	// Without options the defaults still apply.
	if r := performJSONRequest(t, server, http.MethodPost, accept, gin.H{"item_ids": []string{"service:redis"}}, ""); r.Code != http.StatusOK {
		t.Fatalf("accept defaults = %d", r.Code)
	}
	if d := definitionsOf(t, server, id)["redis"]; d.IntervalSeconds != 30 || d.FailAfter != 2 {
		t.Fatalf("redis = %+v", d)
	}
}

func TestMonitorDefinitionTimingCanBeChangedOneByOneOrInBulk(t *testing.T) {
	server := setupTestServer(t)
	node := registerTestNode(t, server)
	id := node.Data.NodeID
	base := "/v1/nodes/" + id + "/monitor-definitions"
	for _, name := range []string{"a", "b"} {
		if r := performJSONRequest(t, server, http.MethodPost, base, gin.H{"name": name, "check_type": "memory", "interval_seconds": 30, "fail_after": 2}, ""); r.Code != http.StatusCreated {
			t.Fatalf("create %s = %d", name, r.Code)
		}
	}
	defs := definitionsOf(t, server, id)
	a, b := defs["a"], defs["b"]

	if r := performJSONRequest(t, server, http.MethodPatch, base+"/"+a.ID, gin.H{"interval_seconds": 300}, ""); r.Code != http.StatusOK {
		t.Fatalf("patch one = %d %s", r.Code, r.Body.String())
	}
	if d := definitionsOf(t, server, id)["a"]; d.IntervalSeconds != 300 || d.FailAfter != 2 || d.ID != a.ID {
		t.Fatalf("a = %+v", d)
	}
	pulled := performJSONRequest(t, server, http.MethodPost, base+"/sync", nil, node.Data.Token)
	if pulled.Code != http.StatusOK || !strings.Contains(pulled.Body.String(), `"interval_seconds":300`) {
		t.Fatalf("node does not see the new interval: %s", pulled.Body.String())
	}

	if r := performJSONRequest(t, server, http.MethodPatch, base, gin.H{"ids": []string{a.ID, b.ID}, "interval_seconds": 60, "fail_after": 5}, ""); r.Code != http.StatusOK {
		t.Fatalf("bulk = %d %s", r.Code, r.Body.String())
	}
	for name, d := range definitionsOf(t, server, id) {
		if d.IntervalSeconds != 60 || d.FailAfter != 5 {
			t.Errorf("%s = %+v", name, d)
		}
	}

	// All or nothing, and invalid input is refused.
	if r := performJSONRequest(t, server, http.MethodPatch, base, gin.H{"ids": []string{a.ID, "nope"}, "interval_seconds": 900}, ""); r.Code != http.StatusNotFound {
		t.Fatalf("unknown id = %d", r.Code)
	}
	if d := definitionsOf(t, server, id)["a"]; d.IntervalSeconds != 60 {
		t.Fatalf("a changed despite the failure: %+v", d)
	}
	for name, body := range map[string]gin.H{"too fast": {"interval_seconds": 3}, "bad fail_after": {"fail_after": 99}, "nothing": {}} {
		if r := performJSONRequest(t, server, http.MethodPatch, base+"/"+a.ID, body, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", name, r.Code)
		}
	}
	if r := performJSONRequest(t, server, http.MethodPatch, "/v1/nodes/nope/monitor-definitions/"+a.ID, gin.H{"interval_seconds": 60}, ""); r.Code != http.StatusNotFound {
		t.Errorf("unknown node = %d", r.Code)
	}
}
