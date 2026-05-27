package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestStatusPageAdminAPIRequiresFrontendJWTWhenConfigured(t *testing.T) {
	server := setupStatusPageAuthTestServer(t)

	unauthorized := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages", nil, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, body = %s, want 401", unauthorized.Code, unauthorized.Body.String())
	}

	token := loginStatusPageTestAdmin(t, server)
	authorized := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages", nil, token)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorized status = %d, body = %s", authorized.Code, authorized.Body.String())
	}
}

func TestStatusPageAdminAPIFlow(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	if err := server.db.Model(&db.Monitor{}).Where("id = ?", registeredMonitor.Data.MonitorID).Updates(map[string]interface{}{
		"health":          "down",
		"computed_health": "down",
	}).Error; err != nil {
		t.Fatalf("update monitor health: %v", err)
	}

	createPageResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{
		"slug":           "main-status",
		"title":          "Main Status",
		"description":    "Customer-facing availability",
		"theme_settings": gin.H{"accent": "green"},
	}, "")
	if createPageResp.Code != http.StatusCreated {
		t.Fatalf("create status page status = %d, body = %s", createPageResp.Code, createPageResp.Body.String())
	}
	var createdPage struct {
		Data struct {
			Page struct {
				ID            string                 `json:"id"`
				Slug          string                 `json:"slug"`
				Title         string                 `json:"title"`
				Visibility    string                 `json:"visibility"`
				ThemeSettings map[string]interface{} `json:"theme_settings"`
			} `json:"page"`
		} `json:"data"`
	}
	decodeResponse(t, createPageResp, &createdPage)
	if createdPage.Data.Page.ID == "" || createdPage.Data.Page.Slug != "main-status" || createdPage.Data.Page.Visibility != "draft" {
		t.Fatalf("created page = %+v, want draft main-status page", createdPage.Data.Page)
	}
	if createdPage.Data.Page.ThemeSettings["accent"] != "green" {
		t.Fatalf("theme settings = %+v, want accent green", createdPage.Data.Page.ThemeSettings)
	}

	createSectionResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/sections", gin.H{
		"name":                 "API",
		"sort_order":           10,
		"collapsed_by_default": false,
	}, "")
	if createSectionResp.Code != http.StatusCreated {
		t.Fatalf("create section status = %d, body = %s", createSectionResp.Code, createSectionResp.Body.String())
	}
	var createdSection struct {
		Data struct {
			Section struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				SortOrder int    `json:"sort_order"`
			} `json:"section"`
		} `json:"data"`
	}
	decodeResponse(t, createSectionResp, &createdSection)
	if createdSection.Data.Section.ID == "" || createdSection.Data.Section.Name != "API" || createdSection.Data.Section.SortOrder != 10 {
		t.Fatalf("created section = %+v, want API section", createdSection.Data.Section)
	}

	updateSectionResp := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+createdPage.Data.Page.ID+"/sections/"+createdSection.Data.Section.ID, gin.H{
		"name":                 "Core API",
		"collapsed_by_default": true,
	}, "")
	if updateSectionResp.Code != http.StatusOK {
		t.Fatalf("update section status = %d, body = %s", updateSectionResp.Code, updateSectionResp.Body.String())
	}

	createComponentResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components", gin.H{
		"section_id":         createdSection.Data.Section.ID,
		"public_name":        "REST API",
		"public_description": "Primary API",
		"display_mode":       "single_resource",
		"sort_order":         5,
		"visible":            true,
	}, "")
	if createComponentResp.Code != http.StatusCreated {
		t.Fatalf("create component status = %d, body = %s", createComponentResp.Code, createComponentResp.Body.String())
	}
	var createdComponent struct {
		Data struct {
			Component struct {
				ID         string `json:"id"`
				SectionID  string `json:"section_id"`
				PublicName string `json:"public_name"`
				Visible    bool   `json:"visible"`
			} `json:"component"`
		} `json:"data"`
	}
	decodeResponse(t, createComponentResp, &createdComponent)
	if createdComponent.Data.Component.ID == "" || createdComponent.Data.Component.SectionID != createdSection.Data.Section.ID ||
		createdComponent.Data.Component.PublicName != "REST API" || !createdComponent.Data.Component.Visible {
		t.Fatalf("created component = %+v, want visible REST API component", createdComponent.Data.Component)
	}

	createMappingResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components/"+createdComponent.Data.Component.ID+"/mappings", gin.H{
		"resource_type":          "monitor",
		"resource_id":            registeredMonitor.Data.MonitorID,
		"health_rollup_strategy": "worst",
		"uptime_rollup_strategy": "worst",
	}, "")
	if createMappingResp.Code != http.StatusCreated {
		t.Fatalf("create mapping status = %d, body = %s", createMappingResp.Code, createMappingResp.Body.String())
	}
	var createdMapping struct {
		Data struct {
			Mapping struct {
				ID                   string `json:"id"`
				ResourceType         string `json:"resource_type"`
				ResourceID           string `json:"resource_id"`
				HealthRollupStrategy string `json:"health_rollup_strategy"`
			} `json:"mapping"`
		} `json:"data"`
	}
	decodeResponse(t, createMappingResp, &createdMapping)
	if createdMapping.Data.Mapping.ID == "" || createdMapping.Data.Mapping.ResourceType != "monitor" ||
		createdMapping.Data.Mapping.ResourceID != registeredMonitor.Data.MonitorID {
		t.Fatalf("created mapping = %+v, want monitor mapping", createdMapping.Data.Mapping)
	}

	updateMappingResp := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components/"+createdComponent.Data.Component.ID+"/mappings/"+createdMapping.Data.Mapping.ID, gin.H{
		"health_rollup_strategy": "average",
	}, "")
	if updateMappingResp.Code != http.StatusOK {
		t.Fatalf("update mapping status = %d, body = %s", updateMappingResp.Code, updateMappingResp.Body.String())
	}

	now := time.Now().UTC()
	createIncidentResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents", gin.H{
		"title":                  "Elevated API errors",
		"public_status":          "investigating",
		"severity":               "high",
		"impact_summary":         "Some requests are failing.",
		"visibility":             "draft",
		"affected_component_ids": []string{createdComponent.Data.Component.ID},
	}, "")
	if createIncidentResp.Code != http.StatusCreated {
		t.Fatalf("create public incident status = %d, body = %s", createIncidentResp.Code, createIncidentResp.Body.String())
	}
	var createdIncident struct {
		Data struct {
			Incident struct {
				ID                   string   `json:"id"`
				Title                string   `json:"title"`
				PublicStatus         string   `json:"public_status"`
				AffectedComponentIDs []string `json:"affected_component_ids"`
			} `json:"incident"`
		} `json:"data"`
	}
	decodeResponse(t, createIncidentResp, &createdIncident)
	if createdIncident.Data.Incident.ID == "" || createdIncident.Data.Incident.Title != "Elevated API errors" ||
		len(createdIncident.Data.Incident.AffectedComponentIDs) != 1 ||
		createdIncident.Data.Incident.AffectedComponentIDs[0] != createdComponent.Data.Component.ID {
		t.Fatalf("created incident = %+v, want API incident", createdIncident.Data.Incident)
	}

	updateIncidentResp := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents/"+createdIncident.Data.Incident.ID, gin.H{
		"public_status": "identified",
		"visibility":    "published",
		"published_at":  now,
	}, "")
	if updateIncidentResp.Code != http.StatusOK {
		t.Fatalf("update public incident status = %d, body = %s", updateIncidentResp.Code, updateIncidentResp.Body.String())
	}

	createUpdateResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents/"+createdIncident.Data.Incident.ID+"/updates", gin.H{
		"status":       "monitoring",
		"message":      "A mitigation is in place.",
		"created_by":   "ops",
		"published_at": now,
	}, "")
	if createUpdateResp.Code != http.StatusCreated {
		t.Fatalf("create public incident update status = %d, body = %s", createUpdateResp.Code, createUpdateResp.Body.String())
	}

	detailResp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+createdPage.Data.Page.ID, nil, "")
	if detailResp.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body = %s", detailResp.Code, detailResp.Body.String())
	}
	var detail struct {
		Data struct {
			Page struct {
				ID string `json:"id"`
			} `json:"page"`
			Sections []struct {
				ID                 string `json:"id"`
				Name               string `json:"name"`
				CollapsedByDefault bool   `json:"collapsed_by_default"`
			} `json:"sections"`
			Components []struct {
				ID       string `json:"id"`
				Mappings []struct {
					ID                   string `json:"id"`
					HealthRollupStrategy string `json:"health_rollup_strategy"`
				} `json:"mappings"`
			} `json:"components"`
			Incidents []struct {
				ID           string `json:"id"`
				PublicStatus string `json:"public_status"`
				Updates      []struct {
					Message string `json:"message"`
				} `json:"updates"`
			} `json:"incidents"`
		} `json:"data"`
	}
	decodeResponse(t, detailResp, &detail)
	if detail.Data.Page.ID != createdPage.Data.Page.ID || len(detail.Data.Sections) != 1 || len(detail.Data.Components) != 1 || len(detail.Data.Incidents) != 1 {
		t.Fatalf("detail = %+v, want one section/component/incident", detail.Data)
	}
	if detail.Data.Sections[0].Name != "Core API" || !detail.Data.Sections[0].CollapsedByDefault {
		t.Fatalf("detail section = %+v, want updated Core API section", detail.Data.Sections[0])
	}
	if len(detail.Data.Components[0].Mappings) != 1 || detail.Data.Components[0].Mappings[0].HealthRollupStrategy != "average" {
		t.Fatalf("detail mappings = %+v, want updated average mapping", detail.Data.Components[0].Mappings)
	}
	if detail.Data.Incidents[0].PublicStatus != "monitoring" || len(detail.Data.Incidents[0].Updates) != 1 ||
		detail.Data.Incidents[0].Updates[0].Message != "A mitigation is in place." {
		t.Fatalf("detail incident = %+v, want monitoring incident with update", detail.Data.Incidents[0])
	}

	previewResp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+createdPage.Data.Page.ID+"/preview", nil, "")
	if previewResp.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", previewResp.Code, previewResp.Body.String())
	}
	assertNotContains(t, previewResp.Body.String(), registeredMonitor.Data.MonitorID)
	var preview struct {
		Data struct {
			Preview struct {
				Page struct {
					Slug string `json:"slug"`
				} `json:"page"`
				Sections []struct {
					Components []struct {
						Name   string `json:"name"`
						Status string `json:"status"`
					} `json:"components"`
				} `json:"sections"`
			} `json:"preview"`
		} `json:"data"`
	}
	decodeResponse(t, previewResp, &preview)
	if preview.Data.Preview.Page.Slug != "main-status" || len(preview.Data.Preview.Sections) != 1 ||
		len(preview.Data.Preview.Sections[0].Components) != 1 ||
		preview.Data.Preview.Sections[0].Components[0].Status != "major_outage" {
		t.Fatalf("preview = %+v, want public-safe major outage preview", preview.Data.Preview)
	}

	publishResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/publish", nil, "")
	if publishResp.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishResp.Code, publishResp.Body.String())
	}
	unpublishResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/unpublish", nil, "")
	if unpublishResp.Code != http.StatusOK {
		t.Fatalf("unpublish status = %d, body = %s", unpublishResp.Code, unpublishResp.Body.String())
	}

	for _, path := range []string{
		"/v1/status-pages",
		"/v1/status-pages/" + createdPage.Data.Page.ID + "/sections",
		"/v1/status-pages/" + createdPage.Data.Page.ID + "/components",
		"/v1/status-pages/" + createdPage.Data.Page.ID + "/components/" + createdComponent.Data.Component.ID + "/mappings",
		"/v1/status-pages/" + createdPage.Data.Page.ID + "/incidents",
	} {
		resp := performJSONRequest(t, server, http.MethodGet, path, nil, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("list %s status = %d, body = %s", path, resp.Code, resp.Body.String())
		}
	}
}

func setupStatusPageAuthTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return NewServer(database, logging.NewLogger(), &config.Config{
		FrontendAuthOn: true,
		AdminUsername:  "admin",
		AdminPassword:  "correct-password",
		JWTSecret:      "test-secret",
	})
}

func loginStatusPageTestAdmin(t *testing.T, server *Server) string {
	t.Helper()

	loginResp := performJSONRequest(t, server, http.MethodPost, "/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "correct-password",
	}, "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResp.Code, loginResp.Body.String())
	}
	var login struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	decodeResponse(t, loginResp, &login)
	if login.Data.Token == "" {
		t.Fatalf("login response missing token: %+v", login)
	}
	return login.Data.Token
}
