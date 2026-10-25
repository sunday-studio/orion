package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"
)

func registerWith(t *testing.T, server *Server, token, machine string) int {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"machine_id": machine, "name": "edge-05", "os": "linux", "arch": "amd64", "reporting_interval_seconds": 60})
	req := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(enrollmentTokenHeader, token)
	}
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	return rec.Code
}

func TestEnrollmentSessionLifecycle(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{EnrollmentToken: "shared-secret", PublicURL: "https://core.example.com"})
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/enrollments", map[string]any{"platform": "linux"}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", resp.Code, resp.Body.String())
	}
	var created struct{ Data EnrollmentCreatedResponse }
	decodeResponse(t, resp, &created)
	tok, id := created.Data.Token, created.Data.ID
	if !strings.HasPrefix(tok, "orn_enr_") || created.Data.Status != "waiting" || !strings.Contains(created.Data.InstallCommand, "ORION_ENROLLMENT_TOKEN='"+tok+"'") || !strings.Contains(created.Data.InstallCommand, "--core-url 'https://core.example.com'") {
		t.Fatalf("created = %+v", created.Data)
	}
	if d := time.Until(created.Data.ExpiresAt); d < 14*time.Minute || d > 15*time.Minute+time.Second {
		t.Fatalf("expires in %v, want 15 minutes", d)
	}
	var stored db.EnrollmentSession
	server.db.First(&stored, "id = ?", id)
	if strings.Contains(stored.TokenHash, "orn_enr_") || stored.TokenHash == tok {
		t.Fatal("token must be stored hashed")
	}

	status := func() EnrollmentResponse {
		var got struct{ Data EnrollmentResponse }
		decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/enrollments/"+id, nil, ""), &got)
		return got.Data
	}
	if status().Status != "waiting" {
		t.Fatal("want waiting")
	}
	if code := registerWith(t, server, "orn_enr_wrong", "m1"); code != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d", code)
	}
	if code := registerWith(t, server, tok, "machine-a"); code != http.StatusOK {
		t.Fatalf("register with session token = %d", code)
	}
	// The bound machine is still authorized on retry (409 is the existing "already registered" answer, not 401).
	if code := registerWith(t, server, tok, "machine-a"); code == http.StatusUnauthorized {
		t.Fatalf("same machine retry was refused")
	}
	if code := registerWith(t, server, tok, "machine-b"); code != http.StatusUnauthorized {
		t.Fatalf("second machine must be refused, got %d", code)
	}
	st := status()
	if st.Status != "registered" || st.NodeName != "edge-05" || st.NodeOS != "linux" || st.NodeArch != "amd64" || st.NodeID == "" {
		t.Fatalf("after register = %+v", st)
	}
	server.db.Create(&db.NodeReport{ID: "r1", NodeID: st.NodeID, NodeVersion: "0.9.2", CreatedAt: time.Now().UTC()})
	if st := status(); st.Status != "reporting" || st.FirstReportAt == nil || st.NodeVersion != "0.9.2" {
		t.Fatalf("after first report = %+v", st)
	}
	// The shared token keeps working alongside session tokens.
	if code := registerWith(t, server, "shared-secret", "machine-c"); code != http.StatusOK {
		t.Fatalf("shared token = %d", code)
	}
}

func TestEnrollmentExpiryAndCancel(t *testing.T) {
	server := setupTestServer(t)
	var created struct{ Data EnrollmentCreatedResponse }
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/enrollments", nil, ""), &created)
	server.db.Model(&db.EnrollmentSession{}).Where("id = ?", created.Data.ID).Update("expires_at", time.Now().UTC().Add(-time.Second))
	if code := registerWith(t, server, created.Data.Token, "m"); code != http.StatusUnauthorized {
		t.Fatalf("expired token = %d", code)
	}
	var got struct{ Data EnrollmentResponse }
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/enrollments/"+created.Data.ID, nil, ""), &got)
	if got.Data.Status != "expired" {
		t.Fatalf("status = %s", got.Data.Status)
	}
	var second struct{ Data EnrollmentCreatedResponse }
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/enrollments", nil, ""), &second)
	if r := performJSONRequest(t, server, http.MethodDelete, "/v1/enrollments/"+second.Data.ID, nil, ""); r.Code != http.StatusOK {
		t.Fatalf("cancel = %d", r.Code)
	}
	if code := registerWith(t, server, second.Data.Token, "m"); code != http.StatusUnauthorized {
		t.Fatalf("cancelled token = %d", code)
	}
	if r := performJSONRequest(t, server, http.MethodGet, "/v1/enrollments/nope", nil, ""); r.Code != http.StatusNotFound {
		t.Fatalf("unknown = %d", r.Code)
	}
}

func TestOverviewSearchAndMe(t *testing.T) {
	server := setupTestServer(t)
	var empty struct{ Data OverviewResponse }
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/overview", nil, ""), &empty)
	if empty.Data.State != "empty" || empty.Data.FirstRun.HasServers || empty.Data.NeedsYou != nil {
		t.Fatalf("empty overview = %+v", empty.Data)
	}

	node := registerTestNode(t, server)
	now := time.Now().UTC()
	server.db.Create(&db.Incident{ID: "inc-1", Status: "open", Severity: "critical", Title: "api.example.com is down", NodeID: node.Data.NodeID, MonitorID: "m1", OpenedAt: now.Add(-30 * time.Minute), LastEventAt: now, LatestEvent: "3 checks failed", NotificationStatus: "sent"})
	server.db.Create(&db.Incident{ID: "inc-2", Status: "resolved", Severity: "low", Title: "old blip", NodeID: node.Data.NodeID, MonitorID: "m1", OpenedAt: now.Add(-3 * time.Hour), ResolvedAt: ptrTime(now.Add(-2 * time.Hour)), LastEventAt: now})
	server.db.Create(&db.AlertDelivery{ID: "d1", IncidentID: "inc-1", EventType: "incident_opened", Channel: "c", Type: "webhook", Status: "sent", CreatedAt: now.Add(-29 * time.Minute)})
	server.db.Create(&db.Monitor{ID: "mon-1", Type: "http", Name: "api-https", NodeID: node.Data.NodeID, Lifecycle: "active", ComputedHealth: "down", Health: "down"})

	var ov struct{ Data OverviewResponse }
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/overview", nil, ""), &ov)
	o := ov.Data
	if o.State != "issues" || o.NeedsYou == nil || o.NeedsYou.ID != "inc-1" || o.NeedsYou.NotifiedAt == nil || o.NeedsYou.DurationSeconds < 1790 ||
		o.OngoingIncidentCount != 1 || len(o.RecentIncidents) != 1 || o.RecentIncidents[0].ID != "inc-2" || o.RecentIncidents[0].DurationSeconds != 3600 ||
		o.Stats.MonitorsTotal != 1 || o.Stats.MonitorsUp != 0 || o.Stats.ServersTotal != 1 || o.Stats.ServersReporting != 1 || o.HealthIssueCount != 1 ||
		len(o.Services) != 1 || len(o.Services[0].Days) != 90 || o.Services[0].Status != "down" || !o.FirstRun.HasServers {
		t.Fatalf("overview = %+v", o)
	}

	var found struct {
		Data struct{ Hits []SearchHitResponse }
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/search?q=API", nil, ""), &found)
	if len(found.Data.Hits) != 2 || found.Data.Hits[0].Type != "monitor" || found.Data.Hits[1].Type != "incident" {
		t.Fatalf("hits = %+v", found.Data.Hits)
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/search?q=%25", nil, ""), &found)
	if len(found.Data.Hits) != 0 {
		t.Fatalf("wildcard must be literal, got %+v", found.Data.Hits)
	}
	var me struct{ Data MeResponse }
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/auth/me", nil, ""), &me)
	if me.Data.Username != "admin" {
		t.Fatalf("me = %+v", me.Data)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestEnrollmentRefusesUnsafeHostInInstallCommand(t *testing.T) {
	server := setupTestServer(t) // no ORION_PUBLIC_URL: the URL comes from the Host header
	req := httptest.NewRequest(http.MethodPost, "/v1/enrollments", strings.NewReader("{}"))
	req.Host = "evil.example.com;rm -rf /"
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("unsafe host = %d %s", rec.Code, rec.Body.String())
	}
	if shellQuote("a'b") != `'a'\''b'` {
		t.Fatalf("shellQuote = %s", shellQuote("a'b"))
	}
}
