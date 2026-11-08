package api

import (
	"net/http"
	"orion/core/internal/db"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUpdatingAnAppKeepsItsServiceLink(t *testing.T) {
	server := setupTestServer(t)
	seedServiceTargets(t, server)
	id := createServiceVia(t, server, gin.H{"name": "Checkout"})
	if resp := performJSONRequest(t, server, http.MethodPost, "/v1/services/"+id+"/links", gin.H{"kind": "app", "id": "app-1"}, ""); resp.Code != http.StatusOK {
		t.Fatalf("link = %d %s", resp.Code, resp.Body.String())
	}
	if resp := performJSONRequest(t, server, http.MethodPatch, "/v1/apps/app-1", gin.H{"description": "edited"}, ""); resp.Code != http.StatusOK {
		t.Fatalf("patch app = %d %s", resp.Code, resp.Body.String())
	}
	var app db.App
	server.db.Where("id = ?", "app-1").First(&app)
	if app.Description != "edited" || app.ServiceID == nil || *app.ServiceID != id {
		t.Fatalf("app after edit: description=%q service_id=%v", app.Description, app.ServiceID)
	}
}

func TestUnlinkQueryValuesAreTrimmed(t *testing.T) {
	server := setupTestServer(t)
	seedServiceTargets(t, server)
	id := createServiceVia(t, server, gin.H{"name": "Checkout"})
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+id+"/links", gin.H{"kind": "app", "id": "app-1"}, "")
	if resp := performJSONRequest(t, server, http.MethodDelete, "/v1/services/"+id+"/links?kind=app&target_id=%20app-1%20", nil, ""); resp.Code != http.StatusOK {
		t.Fatalf("unlink = %d %s", resp.Code, resp.Body.String())
	}
}
