package api

import (
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"strings"
	"testing"
)

func TestCoreMonitorManagementRejectsInvalidConfig(t *testing.T) {
	server := setupTestServer(t)
	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Missing URL", "kind": "http", "config": map[string]interface{}{}}, "")
	if createResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	validResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Valid URL", "kind": "http", "config": map[string]interface{}{"url": "https://example.com/health"}}, "")
	if validResp.Code != http.StatusCreated {
		t.Fatalf("valid create status = %d, body = %s", validResp.Code, validResp.Body.String())
	}
	var created struct {
		Data struct {
			Monitor struct {
				ID string `json:"id"`
			} `json:"monitor"`
		} `json:"data"`
	}
	decodeResponse(t, validResp, &created)
	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+created.Data.Monitor.ID, map[string]interface{}{"config": map[string]interface{}{"url": "ftp://example.com/health"}}, "")
	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid update status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	if err := server.db.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", created.Data.Monitor.ID).Update("config_json", `{"url":"ftp://example.com/health"}`).Error; err != nil {
		t.Fatalf("poison core monitor config: %v", err)
	}
	testResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+created.Data.Monitor.ID+"/test", nil, "")
	if testResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid test-now status = %d, body = %s", testResp.Code, testResp.Body.String())
	}
}
func TestCoreMonitorManagementEnforcesTargetPolicy(t *testing.T) {
	server := setupTestServer(t)
	localhostResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Localhost Target", "kind": "http", "config": map[string]interface{}{"url": "http://localhost:8080/health?token=secret"}}, "")
	if localhostResp.Code != http.StatusBadRequest {
		t.Fatalf("localhost create status = %d, body = %s", localhostResp.Code, localhostResp.Body.String())
	}
	if strings.Contains(localhostResp.Body.String(), "token=secret") {
		t.Fatalf("localhost create leaked query secret: %s", localhostResp.Body.String())
	}
	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Metadata Target", "kind": "http", "config": map[string]interface{}{"url": "http://169.254.169.254/latest/meta-data?token=secret"}}, "")
	if createResp.Code != http.StatusBadRequest {
		t.Fatalf("blocked create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	if strings.Contains(createResp.Body.String(), "token=secret") {
		t.Fatalf("blocked create leaked query secret: %s", createResp.Body.String())
	}
	validResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Public Target", "kind": "http", "config": map[string]interface{}{"url": "https://example.com/health?token=secret"}}, "")
	if validResp.Code != http.StatusCreated {
		t.Fatalf("valid create status = %d, body = %s", validResp.Code, validResp.Body.String())
	}
	var created struct {
		Data struct {
			Monitor struct {
				ID string `json:"id"`
			} `json:"monitor"`
		} `json:"data"`
	}
	decodeResponse(t, validResp, &created)
	configResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+created.Data.Monitor.ID+"/config", nil, "")
	if configResp.Code != http.StatusOK {
		t.Fatalf("config status = %d, body = %s", configResp.Code, configResp.Body.String())
	}
	if strings.Contains(configResp.Body.String(), "token=secret") {
		t.Fatalf("config response leaked query secret: %s", configResp.Body.String())
	}
	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+created.Data.Monitor.ID, map[string]interface{}{"config": map[string]interface{}{"url": "http://127.0.0.1:8080/health"}}, "")
	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf("blocked update status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	if err := server.db.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", created.Data.Monitor.ID).Update("config_json", `{"url":"http://169.254.169.254/latest/meta-data"}`).Error; err != nil {
		t.Fatalf("poison core monitor config: %v", err)
	}
	testResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+created.Data.Monitor.ID+"/test", nil, "")
	if testResp.Code != http.StatusBadRequest {
		t.Fatalf("blocked test-now status = %d, body = %s", testResp.Code, testResp.Body.String())
	}
}
func TestCoreMonitorManagementAllowsPrivateTargetsWhenConfigured(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{CoreMonitorAllowPrivateTargets: true})
	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Private Target", "kind": "tcp", "config": map[string]interface{}{"host": "10.0.0.5", "port": 443}}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("private create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
}
func TestCoreMonitorManagementValidatesCatalogConfigTypes(t *testing.T) {
	server := setupTestServer(t)
	cases := []struct {
		kind          string
		validConfig   map[string]interface{}
		invalidConfig map[string]interface{}
	}{{kind: "heartbeat", validConfig: map[string]interface{}{"grace_seconds": 30}, invalidConfig: map[string]interface{}{"grace_seconds": -1}}, {kind: "http", validConfig: map[string]interface{}{"url": "https://example.com/health", "method": "HEAD", "expected_status": 204}, invalidConfig: map[string]interface{}{"url": "ftp://example.com/health"}}, {kind: "http_keyword", validConfig: map[string]interface{}{"url": "https://example.com/health", "required_contains": []string{"ok"}}, invalidConfig: map[string]interface{}{"url": "https://example.com/health", "method": "POST"}}, {kind: "expected_status", validConfig: map[string]interface{}{"url": "https://example.com/health", "expected_statuses": []int{200, 204}}, invalidConfig: map[string]interface{}{"url": "https://example.com/health", "expected_status": 99}}, {kind: "tcp", validConfig: map[string]interface{}{"host": "example.com", "port": 443}, invalidConfig: map[string]interface{}{"host": "example.com", "port": 70000}}, {kind: "dns", validConfig: map[string]interface{}{"host": "example.com", "record_type": "AAAA"}, invalidConfig: map[string]interface{}{"host": "example.com", "record_type": "SOA"}}, {kind: "tls", validConfig: map[string]interface{}{"host": "example.com", "warning_days": 14}, invalidConfig: map[string]interface{}{"host": "", "port": 443}}, {kind: "udp", validConfig: map[string]interface{}{"host": "example.com", "port": 53, "payload": "ping", "expected_response": "pong"}, invalidConfig: map[string]interface{}{"host": "example.com", "port": 53, "payload": "ping"}}, {kind: "api_request", validConfig: map[string]interface{}{"url": "https://example.com/api", "method": "POST", "expected_status": 201}, invalidConfig: map[string]interface{}{"url": "https://example.com/api", "method": "TRACE"}}, {kind: "domain_expiration", validConfig: map[string]interface{}{"domain": "example.com", "warning_days": 30}, invalidConfig: map[string]interface{}{"domain": "https://example.com"}}, {kind: "ping", validConfig: map[string]interface{}{"host": "example.com", "method": "tcp", "port": 443}, invalidConfig: map[string]interface{}{"host": "example.com", "method": "icmp", "port": 443}}, {kind: "smtp", validConfig: map[string]interface{}{"protocol": "smtp", "host": "mail.example.com", "port": 25, "tls_mode": "starttls"}, invalidConfig: map[string]interface{}{"protocol": "smtp", "host": "mail.example.com", "auth_enabled": true}}, {kind: "imap", validConfig: map[string]interface{}{"protocol": "imap", "host": "mail.example.com", "port": 993, "tls_mode": "implicit"}, invalidConfig: map[string]interface{}{"protocol": "imap", "host": "mail.example.com", "tls_mode": "bad"}}, {kind: "pop3", validConfig: map[string]interface{}{"protocol": "pop", "host": "mail.example.com", "port": 995, "tls_mode": "implicit"}, invalidConfig: map[string]interface{}{"protocol": "pop", "host": "", "port": 995}}, {kind: "synthetic", validConfig: map[string]interface{}{"steps": []map[string]interface{}{{"type": "api", "url": "https://example.com/api", "method": "GET"}}}, invalidConfig: map[string]interface{}{"steps": []map[string]interface{}{{"type": "api", "url": "https://example.com/api", "method": "TRACE"}}}}, {kind: "playwright", validConfig: map[string]interface{}{"url": "https://example.com", "browser": "chromium", "steps": []map[string]interface{}{{"action": "goto", "url": "https://example.com"}}}, invalidConfig: map[string]interface{}{"steps": []map[string]interface{}{{"action": "click"}}}}}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Valid " + tc.kind, "kind": tc.kind, "type": tc.kind, "config": tc.validConfig}, "")
			if createResp.Code != http.StatusCreated {
				t.Fatalf("valid create %s status = %d, body = %s", tc.kind, createResp.Code, createResp.Body.String())
			}
			invalidResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Invalid " + tc.kind, "kind": tc.kind, "type": tc.kind, "config": tc.invalidConfig}, "")
			if invalidResp.Code != http.StatusBadRequest {
				t.Fatalf("invalid create %s status = %d, body = %s", tc.kind, invalidResp.Code, invalidResp.Body.String())
			}
		})
	}
}
