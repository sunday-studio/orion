package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestHeartbeatAcceptsBearerHeaderToken(t *testing.T) {
	server := setupTestServer(t)
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Header heartbeat", "kind": "heartbeat", "interval_seconds": 300, "config": map[string]interface{}{"grace_seconds": 60}}, "")
	var created struct {
		Data struct {
			Config struct {
				HeartbeatToken string `json:"heartbeat_token"`
			} `json:"config"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &created)
	token := created.Data.Config.HeartbeatToken
	if token == "" {
		t.Fatal("no heartbeat token")
	}
	for path, wantHealth := range map[string]string{"/v1/heartbeat/success": "up", "/v1/heartbeat/failure": "down"} {
		r := performRawRequest(t, server, http.MethodPost, path, strings.NewReader(`{"job":"x"}`), token)
		if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"health":"`+wantHealth+`"`) {
			t.Fatalf("%s = %d %s", path, r.Code, r.Body.String())
		}
	}
	if r := performRawRequest(t, server, http.MethodPost, "/v1/heartbeat/success", nil, "wrong-"+token); r.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d", r.Code)
	}
	if r := performRawRequest(t, server, http.MethodPost, "/v1/heartbeat/success", nil, ""); r.Code != http.StatusUnauthorized {
		t.Fatalf("missing token = %d", r.Code)
	}
}
