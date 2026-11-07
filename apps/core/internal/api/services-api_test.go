package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type serviceEnvelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Service     json.RawMessage `json:"service"`
		Services    json.RawMessage `json:"services"`
		Link        json.RawMessage `json:"link"`
		Unlinked    json.RawMessage `json:"unlinked"`
		Suggestions json.RawMessage `json:"suggestions"`
	} `json:"data"`
}

func createServiceVia(t *testing.T, server *Server, body gin.H) string {
	t.Helper()
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/services", body, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("create service status = %d body = %s", resp.Code, resp.Body.String())
	}
	var env serviceEnvelope
	decodeResponse(t, resp, &env)
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env.Data.Service, &created); err != nil || created.ID == "" {
		t.Fatalf("created service = %s, %v", env.Data.Service, err)
	}
	return created.ID
}

func seedServiceTargets(t *testing.T, server *Server) {
	t.Helper()
	now := time.Now().UTC()
	if err := server.db.Create(&db.Node{ID: "node-1", MachineId: "m-1", Name: "web-1", OS: "linux", Arch: "amd64", Token: "tok", CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	server.db.Create(&db.App{ID: "app-1", Name: "checkout-api", Slug: "checkout-api", RetentionDays: 14, DebugSampleRate: 1, DisplayJSON: "{}", RedactionJSON: "{}", CreatedAt: now, UpdatedAt: now})
	server.db.Create(&db.Monitor{ID: "mon-1", NodeID: "node-1", Name: "checkout-api health", Type: "http", Lifecycle: "active", Health: "up", ComputedHealth: "up", IncidentState: "unknown", CreatedAt: now, UpdatedAt: now})
	server.db.Create(&db.NodeAppResource{NodeID: "node-1", Key: "unit:checkout-api", Kind: "unit", Name: "checkout-api.service", UpdatedAt: now})
}

func TestServicesAPIStatusCodes(t *testing.T) {
	server := setupTestServer(t)
	seedServiceTargets(t, server)
	id := createServiceVia(t, server, gin.H{"name": "Checkout", "environment": "production"})

	tests := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
		msg    string
	}{
		{"create: name required", http.MethodPost, "/v1/services", gin.H{"name": ""}, http.StatusBadRequest, "name is required"},
		{"create: bad payload", http.MethodPost, "/v1/services", "not an object", http.StatusBadRequest, "Invalid request payload"},
		{"create: slug conflict", http.MethodPost, "/v1/services", gin.H{"name": "Other", "slug": "checkout"}, http.StatusConflict, "slug already exists"},
		{"get: unknown", http.MethodGet, "/v1/services/svc-nope", nil, http.StatusNotFound, "Service not found"},
		{"update: unknown", http.MethodPatch, "/v1/services/svc-nope", gin.H{"name": "x"}, http.StatusNotFound, "Service not found"},
		{"update: bad runbook", http.MethodPatch, "/v1/services/" + id, gin.H{"runbook_url": "javascript:alert(1)"}, http.StatusBadRequest, "runbook_url"},
		{"delete: unknown", http.MethodDelete, "/v1/services/svc-nope", nil, http.StatusNotFound, "Service not found"},
		{"link: unknown service", http.MethodPost, "/v1/services/svc-nope/links", gin.H{"kind": "app", "id": "app-1"}, http.StatusNotFound, "Service not found"},
		{"link: unknown app", http.MethodPost, "/v1/services/" + id + "/links", gin.H{"kind": "app", "id": "nope"}, http.StatusNotFound, "App not found"},
		{"link: bad kind", http.MethodPost, "/v1/services/" + id + "/links", gin.H{"kind": "node", "id": "node-1"}, http.StatusBadRequest, "kind must be"},
		{"link: bad payload", http.MethodPost, "/v1/services/" + id + "/links", "x", http.StatusBadRequest, "Invalid request payload"},
		{"unlink: not linked", http.MethodDelete, "/v1/services/" + id + "/links?kind=app&target_id=app-1", nil, http.StatusNotFound, "not linked to this service"},
		{"unlink: missing kind", http.MethodDelete, "/v1/services/" + id + "/links", nil, http.StatusBadRequest, "kind must be"},
		{"suggestions: unknown service", http.MethodGet, "/v1/services/suggestions?service_id=svc-nope", nil, http.StatusNotFound, "Service not found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := performJSONRequest(t, server, tc.method, tc.path, tc.body, "")
			if resp.Code != tc.want {
				t.Fatalf("status = %d, want %d; body = %s", resp.Code, tc.want, resp.Body.String())
			}
			var env struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
				Error   string `json:"error"`
			}
			decodeResponse(t, resp, &env)
			if env.Success || !strings.Contains(env.Message, tc.msg) {
				t.Fatalf("body = %s, want success=false and message containing %q", resp.Body.String(), tc.msg)
			}
		})
	}
}

func TestServicesAPILinkFlowShowsUpEverywhere(t *testing.T) {
	server := setupLogsServer(t, nil)
	seedServiceTargets(t, server)
	id := createServiceVia(t, server, gin.H{"name": "Checkout", "slug": "checkout-api", "node_id": "node-1"})

	for _, body := range []gin.H{{"kind": "app", "id": "app-1"}, {"kind": "monitor", "id": "mon-1"}, {"kind": "resource", "node_id": "node-1", "key": "unit:checkout-api"}} {
		if resp := performJSONRequest(t, server, http.MethodPost, "/v1/services/"+id+"/links", body, ""); resp.Code != http.StatusOK {
			t.Fatalf("link %v status = %d body = %s", body, resp.Code, resp.Body.String())
		}
	}

	get := performJSONRequest(t, server, http.MethodGet, "/v1/services/"+id, nil, "")
	var env serviceEnvelope
	decodeResponse(t, get, &env)
	var detail struct {
		Health        string `json:"health"`
		AppCount      int    `json:"app_count"`
		MonitorCount  int    `json:"monitor_count"`
		ResourceCount int    `json:"resource_count"`
		Apps          []struct{ ID string }
		Monitors      []struct{ ID string }
		Resources     []struct{ Key string }
	}
	if err := json.Unmarshal(env.Data.Service, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Health != "up" || detail.AppCount != 1 || detail.MonitorCount != 1 || detail.ResourceCount != 1 ||
		len(detail.Apps) != 1 || detail.Apps[0].ID != "app-1" || detail.Monitors[0].ID != "mon-1" || detail.Resources[0].Key != "unit:checkout-api" {
		t.Fatalf("detail = %+v", detail)
	}

	// The link shows on the app, the monitor and the node's resources.
	var app struct {
		Data struct {
			App struct {
				ServiceID string `json:"service_id"`
			} `json:"app"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/apps/app-1", nil, ""), &app)
	if app.Data.App.ServiceID != id {
		t.Fatalf("app.service_id = %q, want %q", app.Data.App.ServiceID, id)
	}
	var mon struct {
		Data struct {
			Monitors []struct {
				ID        string `json:"id"`
				ServiceID string `json:"service_id"`
			} `json:"monitors"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/nodes/node-1/monitors", nil, ""), &mon)
	if len(mon.Data.Monitors) != 1 || mon.Data.Monitors[0].ServiceID != id {
		t.Fatalf("node monitors = %+v, want service_id %q", mon.Data.Monitors, id)
	}
	var res struct {
		Data struct {
			AppResources *struct {
				Series []struct {
					Key       string `json:"key"`
					ServiceID string `json:"service_id"`
				} `json:"series"`
			} `json:"app_resources"`
		} `json:"data"`
	}
	if err := server.db.Create(&db.NodeAppResourceReport{NodeID: "node-1", Notes: "[]", ReportedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/nodes/node-1/app-resources", nil, ""), &res)
	if res.Data.AppResources == nil || len(res.Data.AppResources.Series) != 1 || res.Data.AppResources.Series[0].ServiceID != id {
		t.Fatalf("app resources = %+v", res.Data.AppResources)
	}

	// The list rolls up counts.
	var list serviceEnvelope
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/services", nil, ""), &list)
	var rows []struct {
		ID            string `json:"id"`
		AppCount      int    `json:"app_count"`
		MonitorCount  int    `json:"monitor_count"`
		ResourceCount int    `json:"resource_count"`
	}
	json.Unmarshal(list.Data.Services, &rows)
	if len(rows) != 1 || rows[0].AppCount != 1 || rows[0].MonitorCount != 1 || rows[0].ResourceCount != 1 {
		t.Fatalf("list = %s", list.Data.Services)
	}

	// Unlink each one: 200, and the app, monitor and resource are still there.
	for _, q := range []url.Values{{"kind": {"app"}, "target_id": {"app-1"}}, {"kind": {"monitor"}, "target_id": {"mon-1"}}, {"kind": {"resource"}, "node_id": {"node-1"}, "key": {"unit:checkout-api"}}} {
		if resp := performJSONRequest(t, server, http.MethodDelete, "/v1/services/"+id+"/links?"+q.Encode(), nil, ""); resp.Code != http.StatusOK {
			t.Fatalf("unlink %v status = %d body = %s", q, resp.Code, resp.Body.String())
		}
	}
	var apps, monitors, resources int64
	server.db.Model(&db.App{}).Count(&apps)
	server.db.Model(&db.Monitor{}).Count(&monitors)
	server.db.Model(&db.NodeAppResource{}).Count(&resources)
	if apps != 1 || monitors != 1 || resources != 1 {
		t.Fatalf("after unlink: apps=%d monitors=%d resources=%d, want 1 each", apps, monitors, resources)
	}

	// Delete reports what it unlinked (nothing now) and the service is gone.
	del := performJSONRequest(t, server, http.MethodDelete, "/v1/services/"+id, nil, "")
	if del.Code != http.StatusOK {
		t.Fatalf("delete status = %d body = %s", del.Code, del.Body.String())
	}
	if again := performJSONRequest(t, server, http.MethodGet, "/v1/services/"+id, nil, ""); again.Code != http.StatusNotFound {
		t.Fatalf("get after delete status = %d", again.Code)
	}
}

func TestServicesAPISuggestionsAreAdviceOnly(t *testing.T) {
	server := setupLogsServer(t, nil)
	seedServiceTargets(t, server)
	id := createServiceVia(t, server, gin.H{"name": "checkout-api"})

	resp := performJSONRequest(t, server, http.MethodGet, "/v1/services/suggestions", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.Code, resp.Body.String())
	}
	var env serviceEnvelope
	decodeResponse(t, resp, &env)
	var suggestions []struct {
		ServiceID string `json:"service_id"`
		Members   []struct {
			Kind   string `json:"kind"`
			Reason string `json:"reason"`
		} `json:"members"`
	}
	if err := json.Unmarshal(env.Data.Suggestions, &suggestions); err != nil {
		t.Fatal(err)
	}
	if len(suggestions) != 1 || suggestions[0].ServiceID != id || len(suggestions[0].Members) != 3 {
		t.Fatalf("suggestions = %s", env.Data.Suggestions)
	}
	for _, m := range suggestions[0].Members {
		if m.Reason == "" {
			t.Fatalf("member %+v has no reason", m)
		}
	}
	var linked int64
	server.db.Model(&db.Monitor{}).Where("service_id IS NOT NULL").Count(&linked)
	if linked != 0 {
		t.Fatal("fetching suggestions linked something")
	}
}

func TestServicesAPISuggestionsEmptyIsAList(t *testing.T) {
	server := setupTestServer(t)
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/services/suggestions", nil, "")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"suggestions":[]`) {
		t.Fatalf("status = %d body = %s", resp.Code, resp.Body.String())
	}
}

func TestServicesAPIWritesAreAudited(t *testing.T) {
	server := setupTestServer(t)
	seedServiceTargets(t, server)
	id := createServiceVia(t, server, gin.H{"name": "Checkout"})
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+id+"/links", gin.H{"kind": "app", "id": "app-1"}, "")
	performJSONRequest(t, server, http.MethodDelete, "/v1/services/"+id, nil, "")

	var actions []string
	server.db.Model(&db.AuditEvent{}).Where("affected_object_type = ?", "service").Order("created_at ASC").Pluck("action", &actions)
	if strings.Join(actions, ",") != "service_created,service_item_linked,service_deleted" {
		t.Fatalf("audit actions = %v", actions)
	}
}

func TestServicesAPIAuditNamesTheServiceAndWhatWasLinked(t *testing.T) {
	server := setupTestServer(t)
	seedServiceTargets(t, server)
	id := createServiceVia(t, server, gin.H{"name": "Checkout"})
	other := createServiceVia(t, server, gin.H{"name": "Billing"})
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+other+"/links", gin.H{"kind": "app", "id": "app-1"}, "")
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+id+"/links", gin.H{"kind": "app", "id": "app-1"}, "") // moves it
	performJSONRequest(t, server, http.MethodDelete, "/v1/services/"+id+"/links?kind=app&target_id=app-1", nil, "")
	performJSONRequest(t, server, http.MethodDelete, "/v1/services/"+id, nil, "")

	metaOf := func(action string) map[string]any {
		var events []db.AuditEvent
		server.db.Where("action = ? AND affected_object_id = ?", action, id).Order("created_at ASC").Find(&events)
		if len(events) == 0 {
			t.Fatalf("no %s audit event for %s", action, id)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(events[len(events)-1].MetadataJSON), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	linked := metaOf("service_item_linked")
	if linked["target_label"] != "Checkout" || linked["link_kind"] != "app" || linked["link_target"] != "app-1" || linked["previous_service_id"] != other {
		t.Errorf("link audit metadata = %v", linked)
	}
	unlinked := metaOf("service_item_unlinked")
	if unlinked["link_kind"] != "app" || unlinked["link_target"] != "app-1" {
		t.Errorf("unlink audit metadata = %v", unlinked)
	}
	if deleted := metaOf("service_deleted"); deleted["target_label"] != "Checkout" {
		t.Errorf("a deleted service should keep its name in the audit entry, got %v", deleted)
	}
}
