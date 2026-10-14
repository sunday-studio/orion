package api

import (
	"encoding/json"
	"net/http"
	"orion/core/internal/config"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAlertChannelRejectsUnsafeWebhookURLs(t *testing.T) {
	server := setupTestServer(t)
	for name, url := range map[string]string{
		"cloud metadata":      "http://169.254.169.254/latest/meta-data/",
		"metadata ipv6":       "http://[fd00:ec2::254]/",
		"loopback ip":         "http://127.0.0.1:8999/v1/agents",
		"localhost name":      "http://localhost/hook",
		"private range":       "https://10.0.0.5/hook",
		"link local":          "https://169.254.10.10/hook",
		"embedded credential": "https://user:pass@alerts.example.com/hook",
		"non-http scheme":     "file:///etc/passwd",
		"ftp scheme":          "ftp://alerts.example.com/hook",
	} {
		t.Run(name, func(t *testing.T) {
			resp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{"name": "bad-" + name, "type": "webhook", "enabled": true, "webhook_url": url}, "")
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("create status = %d, body = %s, want 400", resp.Code, resp.Body.String())
			}
		})
	}
	var count int64
	server.db.Table("alert_channels").Count(&count)
	if count != 0 {
		t.Fatalf("channels persisted = %d, want none", count)
	}
}

func TestAlertChannelAcceptsPublicWebhookAndRejectsUnsafeUpdate(t *testing.T) {
	server := setupTestServer(t)
	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{"name": "ok", "type": "webhook", "enabled": true, "webhook_url": "https://alerts.example.com/hook"}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("public webhook status = %d, body = %s, want 201", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Channel struct {
				ID string `json:"id"`
			} `json:"channel"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createResp.Body.Bytes(), &created); err != nil || created.Data.Channel.ID == "" {
		t.Fatalf("decode created channel: %v, body = %s", err, createResp.Body.String())
	}

	patch := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/channels/"+created.Data.Channel.ID, gin.H{"webhook_url": "http://169.254.169.254/latest/meta-data/"}, "")
	if patch.Code != http.StatusBadRequest {
		t.Fatalf("unsafe update status = %d, body = %s, want 400", patch.Code, patch.Body.String())
	}
	var url string
	server.db.Table("alert_channels").Select("webhook_url").Where("id = ?", created.Data.Channel.ID).Scan(&url)
	if url != "https://alerts.example.com/hook" {
		t.Fatalf("stored webhook_url = %q, want unchanged after rejected update", url)
	}
}

func TestAlertChannelPrivateWebhookAllowedOnlyWithOptIn(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{AlertAllowPrivateWebhooks: true})
	if resp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{"name": "lan", "type": "webhook", "enabled": true, "webhook_url": "http://10.0.0.5:5678/hook"}, ""); resp.Code != http.StatusCreated {
		t.Fatalf("private webhook with opt-in status = %d, body = %s, want 201", resp.Code, resp.Body.String())
	}
	// Metadata and link-local stay blocked even with the opt-in.
	if resp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{"name": "meta", "type": "webhook", "enabled": true, "webhook_url": "http://169.254.169.254/"}, ""); resp.Code != http.StatusBadRequest {
		t.Fatalf("metadata webhook with opt-in status = %d, want 400", resp.Code)
	}
}
