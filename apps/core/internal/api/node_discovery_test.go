package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/service"
)

func discoveryBody(items ...map[string]any) map[string]any {
	return map[string]any{
		"scanned_at": time.Now().UTC().Format(time.RFC3339),
		"enabled":    true,
		"kinds": []map[string]any{
			{"kind": "services", "state": "scanned"},
			{"kind": "containers", "state": "blocked", "message": "Docker is not readable by the node", "fix": "sudo usermod -aG docker orion"},
		},
		"items": items,
	}
}

func TestNodeReportsDiscoveryAndConsoleReadsIt(t *testing.T) {
	server := setupTestServer(t)
	node := registerTestNode(t, server)
	path := "/v1/nodes/" + node.Data.NodeID + "/discovery"

	empty := performJSONRequest(t, server, http.MethodGet, path, nil, "")
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"discovery":null`) {
		t.Fatalf("before any report discovery is null: %d %s", empty.Code, empty.Body.String())
	}

	body := discoveryBody(
		map[string]any{"id": "service:nginx", "kind": "service", "name": "nginx", "confidence": "high", "status": "suggested", "new": true},
		map[string]any{"id": "service:redis", "kind": "service", "name": "redis", "confidence": "high", "status": "monitored", "managed_in": "file"},
		map[string]any{"id": "port:5432", "kind": "port", "name": ":5432", "confidence": "medium", "status": "suggested", "dismissed": true},
	)
	if resp := performJSONRequest(t, server, http.MethodPost, path, body, node.Data.Token); resp.Code != http.StatusOK {
		t.Fatalf("report status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var got struct {
		Data struct {
			Discovery service.NodeDiscoveryView `json:"discovery"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, path, nil, ""), &got)
	d := got.Data.Discovery
	if len(d.Items) != 3 || d.Things != 2 || d.Suggested != 1 || d.Monitored != 1 || d.NewItems != 1 {
		t.Fatalf("counts leave out dismissed items: %+v", d)
	}
	if len(d.Kinds) != 2 || d.Kinds[1].Fix != "sudo usermod -aG docker orion" || d.ReceivedAt.IsZero() {
		t.Fatalf("kinds/received_at: %+v", d)
	}

	// The latest report replaces the previous one.
	next := discoveryBody(map[string]any{"id": "service:nginx", "kind": "service", "name": "nginx", "confidence": "high", "status": "monitored"})
	performJSONRequest(t, server, http.MethodPost, path, next, node.Data.Token)
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, path, nil, ""), &got)
	if len(got.Data.Discovery.Items) != 1 || got.Data.Discovery.Monitored != 1 {
		t.Fatalf("replace, not append: %+v", got.Data.Discovery)
	}
}

func TestDiscoveryReportNeedsTheNodesOwnToken(t *testing.T) {
	server := setupTestServer(t)
	node := registerTestNode(t, server)
	path := "/v1/nodes/" + node.Data.NodeID + "/discovery"
	body := discoveryBody(map[string]any{"id": "service:nginx", "kind": "service", "name": "nginx", "confidence": "high", "status": "suggested"})
	for name, token := range map[string]string{"no token": "", "wrong token": "not-the-token"} {
		if resp := performJSONRequest(t, server, http.MethodPost, path, body, token); resp.Code != http.StatusUnauthorized && resp.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 401/403", name, resp.Code)
		}
	}
}

func TestDiscoveryReportRejectsBadAndOversizedInput(t *testing.T) {
	server := setupTestServer(t)
	node := registerTestNode(t, server)
	path := "/v1/nodes/" + node.Data.NodeID + "/discovery"
	good := map[string]any{"id": "service:x", "kind": "service", "name": "x", "confidence": "high", "status": "suggested"}
	cases := map[string]map[string]any{
		"unknown confidence": discoveryBody(map[string]any{"id": "a", "kind": "service", "name": "a", "confidence": "certain", "status": "suggested"}),
		"unknown status":     discoveryBody(map[string]any{"id": "a", "kind": "service", "name": "a", "confidence": "high", "status": "maybe"}),
		"missing name":       discoveryBody(map[string]any{"id": "a", "kind": "service", "confidence": "high", "status": "suggested"}),
	}
	many := make([]map[string]any, 501)
	for i := range many {
		many[i] = map[string]any{"id": fmt.Sprint("service:", i), "kind": "service", "name": "n", "confidence": "high", "status": "suggested"}
	}
	cases["too many items"] = map[string]any{"enabled": true, "items": many}
	for name, body := range cases {
		if resp := performJSONRequest(t, server, http.MethodPost, path, body, node.Data.Token); resp.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (%s)", name, resp.Code, resp.Body.String())
		}
	}
	huge := discoveryBody(good)
	huge["items"] = []map[string]any{{"id": "a", "kind": "service", "name": strings.Repeat("x", 600<<10), "confidence": "high", "status": "suggested"}}
	if resp := performJSONRequest(t, server, http.MethodPost, path, huge, node.Data.Token); resp.Code != http.StatusBadRequest && resp.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body: status = %d", resp.Code)
	}
	if resp := performJSONRequest(t, server, http.MethodGet, "/v1/nodes/does-not-exist/discovery", nil, ""); resp.Code != http.StatusNotFound {
		t.Errorf("unknown node: status = %d", resp.Code)
	}
}

func TestAnotherNodeCannotReportDiscoveryForThisOne(t *testing.T) {
	server := setupTestServer(t)
	first := registerTestNode(t, server)
	other := db.Node{ID: "node-discovery-other", MachineId: "machine-discovery-other", Name: "other", OS: "linux", Arch: "arm64", Token: "token-discovery-other", LastSeen: time.Now()}
	if err := server.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	body := discoveryBody(map[string]any{"id": "service:x", "kind": "service", "name": "x", "confidence": "high", "status": "suggested"})
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/nodes/"+first.Data.NodeID+"/discovery", body, "token-discovery-other")
	if resp.Code != http.StatusUnauthorized && resp.Code != http.StatusForbidden && resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want a refusal (%s)", resp.Code, resp.Body.String())
	}
	var rows int64
	server.db.Model(&db.NodeDiscovery{}).Count(&rows)
	if rows != 0 {
		t.Fatalf("a refused report stored %d rows", rows)
	}
}

func TestFirstReportWithScanningOffReadsBackAsOff(t *testing.T) {
	server := setupTestServer(t)
	node := registerTestNode(t, server)
	path := "/v1/nodes/" + node.Data.NodeID + "/discovery"
	if resp := performJSONRequest(t, server, http.MethodPost, path, map[string]any{"enabled": false, "kinds": []any{}, "items": []any{}}, node.Data.Token); resp.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.Code, resp.Body.String())
	}
	var got struct {
		Data struct {
			Discovery service.NodeDiscoveryView `json:"discovery"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, path, nil, ""), &got)
	if got.Data.Discovery.Enabled {
		t.Fatal("scanning was reported off, the Console must not show it on")
	}
}

func TestRemovingAServerRemovesItsDiscovery(t *testing.T) {
	server := setupTestServer(t)
	node := registerTestNode(t, server)
	path := "/v1/nodes/" + node.Data.NodeID + "/discovery"
	performJSONRequest(t, server, http.MethodPost, path, discoveryBody(map[string]any{"id": "service:x", "kind": "service", "name": "x", "confidence": "high", "status": "suggested"}), node.Data.Token)
	if resp := performJSONRequest(t, server, http.MethodDelete, "/v1/nodes/"+node.Data.NodeID+"?keep_history=false", nil, ""); resp.Code != http.StatusOK {
		t.Fatalf("remove status = %d: %s", resp.Code, resp.Body.String())
	}
	var rows int64
	server.db.Model(&db.NodeDiscovery{}).Where("node_id = ?", node.Data.NodeID).Count(&rows)
	if rows != 0 {
		t.Fatalf("%d discovery rows left behind", rows)
	}
}
