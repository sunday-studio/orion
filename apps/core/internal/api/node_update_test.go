package api

import (
	"errors"
	"net/http"
	"orion/core/internal/buildinfo"
	"orion/core/internal/service"
	"strings"
	"testing"
)

const nodeSum = "9f2c1a7e5b8d4c3a6e0f1b2d3c4a5b6c7d8e9f0a1b2c3d4e5f60718293a4b5c6"

func updateFixture(t *testing.T, caps []string, nodeVersion string) (*Server, string, string, *fakeReleases) {
	t.Helper()
	previous := buildinfo.Version
	buildinfo.Version = "0.9.3"
	t.Cleanup(func() { buildinfo.Version = previous })
	server := setupTestServer(t)
	fake := &fakeReleases{sums: map[string]string{"orion-node-linux-arm64": nodeSum, "orion-node-linux-amd64": "other"}}
	server.releases = fake
	node := registerTestNode(t, server)
	reportWithCaps(t, server, node.Data.NodeID, node.Data.Token, caps)
	if nodeVersion != "" {
		seedNodeReports(t, server, node.Data.NodeID, 1, 50) // reports version 0.9.2
	}
	return server, node.Data.NodeID, node.Data.Token, fake
}

func TestUpdateNodeQueuesAnUpdateWithTheChecksumForItsPlatform(t *testing.T) {
	server, id, token, _ := updateFixture(t, []string{"requests", "self_update"}, "0.9.2")
	var got reqResp
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/nodes/"+id+"/update", nil, "")
	decodeResponse(t, resp, &got)
	if resp.Code != http.StatusCreated || got.Data.Request.Type != service.ReqUpdateNode || got.Data.Request.Status != service.ReqQueued {
		t.Fatalf("update = %d %+v", resp.Code, got.Data.Request)
	}
	payload := got.Data.Request.Payload
	if payload["version"] != "0.9.3" || payload["tag"] != "v0.9.3" || payload["sha256"] != nodeSum {
		t.Fatalf("payload = %v", payload)
	}
	// The node pulls the same payload.
	var pulled reqResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/nodes/"+id+"/discovery/requests/pull", nil, token), &pulled)
	if len(pulled.Data.Requests) != 1 || pulled.Data.Requests[0].Payload["sha256"] != nodeSum {
		t.Fatalf("pulled = %+v", pulled.Data.Requests)
	}
	// A second update while one waits is refused.
	if resp := performJSONRequest(t, server, http.MethodPost, "/v1/nodes/"+id+"/update", nil, ""); resp.Code != http.StatusConflict {
		t.Errorf("second update = %d, want 409", resp.Code)
	}
}

func TestUpdateNodeRefusals(t *testing.T) {
	cases := []struct {
		name  string
		caps  []string
		node  string
		setup func(f *fakeReleases)
		want  int
		text  string
	}{
		{"node cannot self update", []string{"requests"}, "0.9.2", nil, http.StatusBadRequest, "cannot update itself"},
		{"node already current", []string{"requests", "self_update"}, "", nil, http.StatusConflict, "already on"},
		{"release has no file for the platform", []string{"requests", "self_update"}, "0.9.2", func(f *fakeReleases) { f.sums = map[string]string{} }, http.StatusBadGateway, "no file named"},
		{"release list unreachable", []string{"requests", "self_update"}, "0.9.2", func(f *fakeReleases) { f.sumsErr = errors.New("offline") }, http.StatusBadGateway, "offline"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server, id, _, fake := updateFixture(t, c.caps, c.node)
			if c.setup != nil {
				c.setup(fake)
			}
			resp := performJSONRequest(t, server, http.MethodPost, "/v1/nodes/"+id+"/update", nil, "")
			if resp.Code != c.want || !strings.Contains(resp.Body.String(), c.text) {
				t.Fatalf("got %d %s, want %d containing %q", resp.Code, resp.Body.String(), c.want, c.text)
			}
		})
	}
}

func TestUpdateNodeNeedsAReleaseBuildAndAKnownNode(t *testing.T) {
	server, id, _, _ := updateFixture(t, []string{"requests", "self_update"}, "0.9.2")
	buildinfo.Version = "dev"
	if resp := performJSONRequest(t, server, http.MethodPost, "/v1/nodes/"+id+"/update", nil, ""); resp.Code != http.StatusConflict {
		t.Errorf("dev core = %d, want 409", resp.Code)
	}
	buildinfo.Version = "0.9.3"
	if resp := performJSONRequest(t, server, http.MethodPost, "/v1/nodes/missing/update", nil, ""); resp.Code != http.StatusNotFound {
		t.Errorf("unknown node = %d, want 404", resp.Code)
	}
}

func TestGenericRequestRouteCannotQueueAnUpdate(t *testing.T) {
	server, id, _, _ := updateFixture(t, []string{"requests", "self_update"}, "0.9.2")
	body := map[string]any{"type": "update_node", "payload": map[string]any{"version": "9.9.9", "tag": "v9.9.9", "sha256": nodeSum}}
	if resp := performJSONRequest(t, server, http.MethodPost, "/v1/nodes/"+id+"/discovery/requests", body, ""); resp.Code != http.StatusBadRequest {
		t.Errorf("generic update = %d, want 400", resp.Code)
	}
}
