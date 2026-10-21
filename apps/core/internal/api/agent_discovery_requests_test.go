package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/service"
)

type reqResp struct {
	Data struct {
		Request  service.AgentDiscoveryRequestView   `json:"request"`
		Requests []service.AgentDiscoveryRequestView `json:"requests"`
	} `json:"data"`
}

func reportWithCaps(t *testing.T, server *Server, agentID, token string, caps []string, items ...map[string]any) {
	t.Helper()
	body := discoveryBody(items...)
	body["capabilities"] = caps
	if resp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+agentID+"/discovery", body, token); resp.Code != http.StatusOK {
		t.Fatalf("report status = %d: %s", resp.Code, resp.Body.String())
	}
}

func TestRequestForAnAgentWithoutTheCapabilityIsStoredAsUnsupported(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	path := "/v1/agents/" + agent.Data.AgentID + "/discovery/requests"
	var got reqResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, path, map[string]any{"type": "rescan"}, ""), &got)
	if got.Data.Request.Status != service.ReqUnsupported || !strings.Contains(got.Data.Request.Result, "cannot take requests") {
		t.Fatalf("no discovery report yet: %+v", got.Data.Request)
	}
	reportWithCaps(t, server, agent.Data.AgentID, agent.Data.Token, nil, map[string]any{"id": "service:x", "kind": "service", "name": "x", "confidence": "high", "status": "suggested"})
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, path, map[string]any{"type": "rescan"}, ""), &got)
	if got.Data.Request.Status != service.ReqUnsupported {
		t.Fatalf("an agent that reports no capabilities cannot take requests: %+v", got.Data.Request)
	}
}

func TestRequestLifecycleQueuedDeliveredApplied(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	reportWithCaps(t, server, agent.Data.AgentID, agent.Data.Token, []string{"requests"})
	base := "/v1/agents/" + agent.Data.AgentID + "/discovery/requests"
	var got reqResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, base, map[string]any{"type": "dismiss", "payload": map[string]any{"ids": []string{"docker:legacy-cron"}}}, ""), &got)
	if got.Data.Request.Status != service.ReqQueued || got.Data.Request.ID == "" {
		t.Fatalf("created: %+v", got.Data.Request)
	}
	id := got.Data.Request.ID

	var pulled reqResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, base+"/pull", nil, agent.Data.Token), &pulled)
	if len(pulled.Data.Requests) != 1 || pulled.Data.Requests[0].ID != id || pulled.Data.Requests[0].Status != service.ReqDelivered {
		t.Fatalf("pulled: %+v", pulled.Data.Requests)
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, base+"/pull", nil, agent.Data.Token), &pulled)
	if len(pulled.Data.Requests) != 0 {
		t.Fatalf("a delivered request is not handed out again straight away: %+v", pulled.Data.Requests)
	}
	if resp := performJSONRequest(t, server, http.MethodDelete, base+"/"+id, nil, ""); resp.Code != http.StatusConflict {
		t.Fatalf("a delivered request cannot be cancelled, status %d", resp.Code)
	}
	if resp := performJSONRequest(t, server, http.MethodPost, base+"/"+id+"/result", map[string]any{"status": "applied", "result": "Dismissed docker:legacy-cron."}, agent.Data.Token); resp.Code != http.StatusOK {
		t.Fatalf("result status = %d: %s", resp.Code, resp.Body.String())
	}
	var list reqResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, base, nil, ""), &list)
	if len(list.Data.Requests) != 1 || list.Data.Requests[0].Status != service.ReqApplied || list.Data.Requests[0].Result != "Dismissed docker:legacy-cron." || list.Data.Requests[0].FinishedAt == nil {
		t.Fatalf("listed: %+v", list.Data.Requests)
	}
}

func TestQueuedRequestCanBeCancelledAndExpires(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	reportWithCaps(t, server, agent.Data.AgentID, agent.Data.Token, []string{"requests"})
	base := "/v1/agents/" + agent.Data.AgentID + "/discovery/requests"
	var a, b reqResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, base, map[string]any{"type": "rescan"}, ""), &a)
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, base, map[string]any{"type": "set_scanning", "payload": map[string]any{"enabled": false}}, ""), &b)
	if resp := performJSONRequest(t, server, http.MethodDelete, base+"/"+a.Data.Request.ID, nil, ""); resp.Code != http.StatusOK {
		t.Fatalf("cancel status = %d", resp.Code)
	}
	server.db.Model(&db.AgentDiscoveryRequest{}).Where("id = ?", b.Data.Request.ID).Update("expires_at", time.Now().UTC().Add(-time.Minute))
	var list reqResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, base, nil, ""), &list)
	status := map[string]string{}
	for _, r := range list.Data.Requests {
		status[r.ID] = r.Status
	}
	if status[a.Data.Request.ID] != service.ReqCancelled || status[b.Data.Request.ID] != service.ReqExpired {
		t.Fatalf("statuses = %v", status)
	}
	var pulled reqResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, base+"/pull", nil, agent.Data.Token), &pulled)
	if len(pulled.Data.Requests) != 0 {
		t.Fatalf("cancelled and expired requests are never handed to the agent: %+v", pulled.Data.Requests)
	}
	// Finished history older than the retention window is pruned on the next sweep.
	server.db.Model(&db.AgentDiscoveryRequest{}).Where("id = ?", a.Data.Request.ID).Update("finished_at", time.Now().UTC().Add(-31*24*time.Hour))
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, base, nil, ""), &list)
	for _, r := range list.Data.Requests {
		if r.ID == a.Data.Request.ID {
			t.Fatalf("a request finished over 30 days ago should be pruned: %+v", r)
		}
	}
}

func TestRequestPayloadsAreValidated(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	reportWithCaps(t, server, agent.Data.AgentID, agent.Data.Token, []string{"requests"})
	base := "/v1/agents/" + agent.Data.AgentID + "/discovery/requests"
	bad := map[string]map[string]any{
		"unknown type":       {"type": "reboot"},
		"dismiss no ids":     {"type": "dismiss", "payload": map[string]any{}},
		"dismiss bad id":     {"type": "dismiss", "payload": map[string]any{"ids": []string{"not an id"}}},
		"scanning no bool":   {"type": "set_scanning", "payload": map[string]any{"enabled": "yes"}},
		"unknown kind":       {"type": "set_kind", "payload": map[string]any{"kind": "kernel", "enabled": true}},
		"relative path":      {"type": "track_storage", "payload": map[string]any{"paths": []string{"var/lib"}}},
		"path traversal":     {"type": "track_storage", "payload": map[string]any{"paths": []string{"/var/../etc"}}},
		"log source (later)": {"type": "create_log_source", "payload": map[string]any{}},
	}
	for name, body := range bad {
		if resp := performJSONRequest(t, server, http.MethodPost, base, body, ""); resp.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (%s)", name, resp.Code, resp.Body.String())
		}
	}
	good := []map[string]any{
		{"type": "set_kind", "payload": map[string]any{"kind": "containers", "enabled": false}},
		{"type": "track_storage", "payload": map[string]any{"paths": []string{"/var/lib/postgresql"}}},
		{"type": "ignore", "payload": map[string]any{"ids": []string{"port:5432"}}},
	}
	for _, body := range good {
		if resp := performJSONRequest(t, server, http.MethodPost, base, body, ""); resp.Code != http.StatusCreated {
			t.Errorf("%v: status = %d: %s", body["type"], resp.Code, resp.Body.String())
		}
	}
}

func TestAnotherAgentCannotPullOrAnswerThisAgentsRequests(t *testing.T) {
	server := setupTestServer(t)
	first := registerTestAgent(t, server)
	other := db.Agent{ID: "agent-requests-other", MachineId: "machine-requests-other", Name: "other", OS: "linux", Arch: "arm64", Token: "token-requests-other", LastSeen: time.Now()}
	if err := server.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	reportWithCaps(t, server, first.Data.AgentID, first.Data.Token, []string{"requests"})
	base := "/v1/agents/" + first.Data.AgentID + "/discovery/requests"
	var got reqResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, base, map[string]any{"type": "rescan"}, ""), &got)
	for name, resp := range map[string]int{
		"pull":   performJSONRequest(t, server, http.MethodPost, base+"/pull", nil, "token-requests-other").Code,
		"result": performJSONRequest(t, server, http.MethodPost, base+"/"+got.Data.Request.ID+"/result", map[string]any{"status": "applied"}, "token-requests-other").Code,
	} {
		if resp == http.StatusOK {
			t.Errorf("%s with another agent's token must be refused, got %d", name, resp)
		}
	}
}

func TestTokenCheckAcceptsTheAgentTokenWhenConsoleAuthIsOn(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{AdminUsername: "admin", AdminPassword: "pw", JWTSecret: "secret-secret-secret-secret-1234", FrontendAuthOn: true})
	agent := registerTestAgent(t, server)
	path := "/v1/agents/" + agent.Data.AgentID + "/token-check"
	if resp := performJSONRequest(t, server, http.MethodPost, path, nil, agent.Data.Token); resp.Code != http.StatusOK {
		t.Fatalf("the agent's own token: %d %s", resp.Code, resp.Body.String())
	}
	if resp := performJSONRequest(t, server, http.MethodPost, path, nil, "not-the-token"); resp.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong token: %d", resp.Code)
	}
}
