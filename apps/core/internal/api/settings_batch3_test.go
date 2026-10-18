package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func authedServer(t *testing.T) (*Server, string) {
	t.Helper()
	server := setupTestServerWithConfig(t, &config.Config{FrontendAuthOn: true, AdminUsername: "admin", AdminPassword: "pw-pw-pw-pw", JWTSecret: "jwt-secret-jwt-secret-jwt", SessionHours: 12, AlertRecoveryNotifications: true, DataDir: t.TempDir()})
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/auth/login", gin.H{"username": "admin", "password": "pw-pw-pw-pw"}, "")
	var login struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &login)
	if login.Data.Token == "" {
		t.Fatalf("login = %d %s", resp.Code, resp.Body.String())
	}
	return server, login.Data.Token
}

func TestEveryMutatingAdminRouteIsAuditedOrExempt(t *testing.T) {
	server := setupTestServer(t)
	server.auditRouteProbe = true
	var missing []string
	for _, r := range server.router.Routes() {
		if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" || !strings.HasPrefix(r.Path, "/v1/") {
			continue
		}
		path := r.Path
		req := httptest.NewRequest(r.Method, path, strings.NewReader("{}"))
		req.Header.Set("X-Orion-Route-Probe", "1")
		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)
		if !strings.Contains(rec.Body.String(), `"admin_route"`) {
			continue // not an admin route: agents, ingest, registration, heartbeats, login
		}
		key := r.Method + " " + path
		_, audited := auditRoutes[key]
		_, exempt := auditExempt[key]
		if !audited && !exempt {
			missing = append(missing, key)
		}
		if audited && exempt {
			t.Errorf("%s is both audited and exempt", key)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("admin routes neither audited nor exempt (add them to audit_log_routes.go or audit_log_exempt.go):\n%s", strings.Join(missing, "\n"))
	}
	// Stale table entries point at routes that no longer exist.
	known := map[string]bool{}
	for _, r := range server.router.Routes() {
		known[r.Method+" "+r.Path] = true
	}
	for key := range auditRoutes {
		if !known[key] {
			t.Errorf("auditRoutes has %q but no such route is registered", key)
		}
	}
	for key := range auditExempt {
		if !known[key] {
			t.Errorf("auditExempt has %q but no such route is registered", key)
		}
	}
}

func bearer(t *testing.T, server *Server, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return performJSONRequest(t, server, method, path, body, token)
}

func TestAPITokensScopesAuditAndSessionOnlyManagement(t *testing.T) {
	server, jwt := authedServer(t)
	create := func(name, scope string) (APITokenCreated, int) {
		r := bearer(t, server, http.MethodPost, "/v1/settings/api-tokens", jwt, gin.H{"name": name, "scope": scope})
		var out struct {
			Data struct {
				Token APITokenCreated `json:"token"`
			} `json:"data"`
		}
		json.Unmarshal(r.Body.Bytes(), &out)
		return out.Data.Token, r.Code
	}
	reader, code := create("grafana", "read")
	if code != http.StatusCreated || !strings.HasPrefix(reader.Token, "orn_api_") || reader.Scope != "read" || reader.Prefix == "" || strings.Contains(reader.Prefix, reader.Token[12:]) {
		t.Fatalf("create reader = %d %+v", code, reader)
	}
	writer, _ := create("ci-deploys", "write")
	if _, code := create("grafana", "read"); code != http.StatusConflict {
		t.Errorf("duplicate name = %d, want 409", code)
	}
	if _, code := create("x", "admin"); code != http.StatusBadRequest {
		t.Errorf("bad scope = %d, want 400", code)
	}

	// Read scope: GET works, writes are refused.
	if r := bearer(t, server, http.MethodGet, "/v1/agents", reader.Token, nil); r.Code != http.StatusOK {
		t.Errorf("reader GET = %d", r.Code)
	}
	if r := bearer(t, server, http.MethodPost, "/v1/incidents/bulk", reader.Token, gin.H{}); r.Code != http.StatusForbidden || !strings.Contains(r.Body.String(), "read only") {
		t.Errorf("reader POST = %d %s", r.Code, r.Body.String())
	}
	// Write scope can work, but never manages tokens or the registration token.
	server.db.Create(&db.Monitor{ID: "m1", Name: "api-https", Type: "http", AgentID: "a1", Lifecycle: "active", Health: "up"})
	server.db.Create(&db.CoreMonitorConfig{MonitorID: "m1", Kind: "http", ConfigJSON: `{"url":"https://x.test"}`, SecretRefJSON: "{}", IntervalSeconds: 60, TimeoutSeconds: 10, NextRunAt: time.Now().UTC().Add(time.Hour)})
	if r := bearer(t, server, http.MethodPost, "/v1/monitors/m1/pause", writer.Token, nil); r.Code != http.StatusOK {
		t.Errorf("writer pause = %d %s", r.Code, r.Body.String())
	}
	for name, req := range map[string][2]string{"list": {http.MethodGet, "/v1/settings/api-tokens"}, "create": {http.MethodPost, "/v1/settings/api-tokens"}, "rotate": {http.MethodPost, "/v1/settings/system/registration-token/rotate"}} {
		if r := bearer(t, server, req[0], req[1], writer.Token, gin.H{"name": "sneaky"}); r.Code != http.StatusForbidden {
			t.Errorf("writer %s = %d, want 403", name, r.Code)
		}
	}
	// Unknown and revoked tokens are refused; last use is recorded.
	if r := bearer(t, server, http.MethodGet, "/v1/agents", "orn_api_deadbeef", nil); r.Code != http.StatusUnauthorized {
		t.Errorf("unknown token = %d", r.Code)
	}
	var list struct {
		Data struct {
			Tokens []APIToken `json:"tokens"`
		} `json:"data"`
	}
	json.Unmarshal(bearer(t, server, http.MethodGet, "/v1/settings/api-tokens", jwt, nil).Body.Bytes(), &list)
	if len(list.Data.Tokens) != 2 || strings.Contains(bearer(t, server, http.MethodGet, "/v1/settings/api-tokens", jwt, nil).Body.String(), reader.Token) {
		t.Fatalf("token list = %+v", list.Data.Tokens)
	}
	var used bool
	for _, tok := range list.Data.Tokens {
		if tok.Name == "grafana" && tok.LastUsedAt != nil {
			used = true
		}
	}
	if !used {
		t.Error("last_used_at was not recorded")
	}
	if r := bearer(t, server, http.MethodDelete, "/v1/settings/api-tokens/"+reader.ID, jwt, nil); r.Code != http.StatusOK {
		t.Fatalf("revoke = %d", r.Code)
	}
	if r := bearer(t, server, http.MethodGet, "/v1/agents", reader.Token, nil); r.Code != http.StatusUnauthorized {
		t.Errorf("revoked token = %d, want 401", r.Code)
	}

	// The audit log names the actors: the admin for settings, the token for its own change.
	var audit struct {
		Data AuditListResponse `json:"data"`
	}
	json.Unmarshal(bearer(t, server, http.MethodGet, "/v1/settings/audit", jwt, nil).Body.Bytes(), &audit)
	seen := map[string]AuditEntry{}
	for _, e := range audit.Data.Entries {
		seen[e.ActionKey+"|"+e.Actor] = e
	}
	if e, ok := seen["api_token_created|admin"]; !ok || e.Action != "Created API token" || (e.Target != "grafana" && e.Target != "ci-deploys") || e.ActorType != "user" {
		t.Errorf("token created entry = %+v (all: %v)", e, audit.Data.Entries)
	}
	if e := seen["api_token_created|admin"]; e.ObjectID != reader.ID && e.ObjectID != writer.ID {
		t.Errorf("created token audit should carry the new token id, got %q", e.ObjectID)
	}
	if e, ok := seen["monitor_paused|ci-deploys"]; !ok || e.Target != "api-https" || e.ActorType != "api_token" || e.Action != "Paused monitor" {
		t.Errorf("monitor paused entry = %+v", e)
	}
	if e, ok := seen["api_token_revoked|admin"]; !ok || e.Target != "grafana" {
		t.Errorf("revoke entry = %+v", e)
	}
	if len(audit.Data.Actors) < 2 {
		t.Errorf("actors = %+v", audit.Data.Actors)
	}
	var byActor struct {
		Data AuditListResponse `json:"data"`
	}
	json.Unmarshal(bearer(t, server, http.MethodGet, "/v1/settings/audit?actor=ci-deploys&q=paused", jwt, nil).Body.Bytes(), &byActor)
	if len(byActor.Data.Entries) != 1 {
		t.Errorf("actor+q filter = %+v", byActor.Data.Entries)
	}
}

func TestGeneralSystemRegistrationAndSession(t *testing.T) {
	server, jwt := authedServer(t)
	type general struct {
		Data GeneralSettings `json:"data"`
	}
	var g general
	json.Unmarshal(bearer(t, server, http.MethodGet, "/v1/settings/general", jwt, nil).Body.Bytes(), &g)
	if g.Data.Timezone != "UTC" || g.Data.TimeFormat != "24" {
		t.Fatalf("defaults = %+v", g.Data)
	}
	for name, body := range map[string]gin.H{"bad tz": {"timezone": "Mars/Base"}, "bad fmt": {"time_format": "13"}, "bad url": {"public_url": "ftp://x"}, "url creds": {"public_url": "https://u:p@x.test"}} {
		if r := bearer(t, server, http.MethodPut, "/v1/settings/general", jwt, body); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", name, r.Code)
		}
	}
	r := bearer(t, server, http.MethodPut, "/v1/settings/general", jwt, gin.H{"instance_name": "Acme Orion", "public_url": "https://orion.acme.test/", "timezone": "Europe/London", "time_format": "12"})
	json.Unmarshal(r.Body.Bytes(), &g)
	if r.Code != http.StatusOK || g.Data.InstanceName != "Acme Orion" || g.Data.PublicURL != "https://orion.acme.test" || g.Data.PublicURLSource != "settings" || g.Data.TimeFormat != "12" {
		t.Fatalf("save = %d %+v", r.Code, g.Data)
	}
	if server.cfg.PublicBaseURL() != "https://orion.acme.test" {
		t.Errorf("public url not applied: %q", server.cfg.PublicBaseURL())
	}

	var sys struct {
		Data SystemSettings `json:"data"`
	}
	json.Unmarshal(bearer(t, server, http.MethodGet, "/v1/settings/system", jwt, nil).Body.Bytes(), &sys)
	if sys.Data.SignIn.Username != "admin" || sys.Data.SignIn.SessionHours != 12 || sys.Data.Database.Engine != "SQLite" || sys.Data.Registration.Configured || sys.Data.Registration.Source != "none" || sys.Data.Worker.Status != "missing" || sys.Data.Version == "" {
		t.Fatalf("system = %+v", sys.Data)
	}
	// Open registration until a token is rotated in; then only the new token registers.
	register := func(token string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/register", strings.NewReader(`{"machine_id":"m-`+token+`","name":"n","os":"linux","arch":"amd64"}`))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set(enrollmentTokenHeader, token)
		}
		return rawRequest(server, req).Code
	}
	if code := register(""); code != http.StatusOK && code != http.StatusCreated {
		t.Fatalf("open registration = %d", code)
	}
	var rotated struct {
		Data RegistrationTokenResponse `json:"data"`
	}
	json.Unmarshal(bearer(t, server, http.MethodPost, "/v1/settings/system/registration-token/rotate", jwt, nil).Body.Bytes(), &rotated)
	if !strings.HasPrefix(rotated.Data.Token, "orn_reg_") {
		t.Fatalf("rotated = %+v", rotated.Data)
	}
	if code := register(""); code != http.StatusUnauthorized {
		t.Errorf("registration without the token = %d, want 401", code)
	}
	if code := register(rotated.Data.Token); code != http.StatusOK && code != http.StatusCreated {
		t.Errorf("registration with the new token = %d", code)
	}
	json.Unmarshal(bearer(t, server, http.MethodGet, "/v1/settings/system", jwt, nil).Body.Bytes(), &sys)
	if !sys.Data.Registration.Configured || sys.Data.Registration.Token != rotated.Data.Token || sys.Data.Registration.Source != "settings" || sys.Data.Registration.RotatedAt == nil {
		t.Errorf("system after rotate = %+v", sys.Data.Registration)
	}

	// Session length comes from the config.
	login := performJSONRequest(t, server, http.MethodPost, "/v1/auth/login", gin.H{"username": "admin", "password": "pw-pw-pw-pw"}, "")
	var tok struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(login.Body.Bytes(), &tok)
	parts := strings.Split(tok.Data.Token, ".")
	payload, _ := decodeSegment(parts[1])
	var claims struct {
		Exp int64 `json:"exp"`
		Iat int64 `json:"iat"`
	}
	json.Unmarshal(payload, &claims)
	if hours := time.Duration(claims.Exp-claims.Iat) * time.Second; hours != 12*time.Hour {
		t.Errorf("session lasts %v, want 12h", hours)
	}
}

func decodeSegment(seg string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(seg) }
