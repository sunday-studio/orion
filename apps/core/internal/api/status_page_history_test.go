package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"orion/core/internal/db"
)

func TestPublicStatusPageComponentHistoryAggregatesAndRedactsInternals(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC().Add(-1 * time.Hour)

	page := db.StatusPage{
		ID:                        "status-page-history-test",
		Slug:                      "uptime-status",
		Title:                     "Uptime Status",
		Visibility:                "public",
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: "draft",
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	section := db.StatusPageSection{
		ID:           "status-page-history-section",
		StatusPageID: page.ID,
		Name:         "Core",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	component := db.StatusPageComponent{
		ID:           "status-page-history-component",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Public API",
		DisplayMode:  "single_resource",
		ManualStatus: "",
		SortOrder:    1,
		Visible:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	hiddenComponent := db.StatusPageComponent{
		ID:           "status-page-hidden-component",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Hidden database",
		DisplayMode:  "single_resource",
		SortOrder:    2,
		Visible:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	agent := db.Agent{
		ID:        "agent-private-history",
		MachineId: "machine-private-history",
		Name:      "private host",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "secret-agent-token",
		LastSeen:  now,
		CreatedAt: now,
	}
	monitors := []db.Monitor{
		{ID: "monitor-history-perfect", AgentID: agent.ID, Name: "private perfect", Type: "http", Lifecycle: "active", Health: "up", ComputedHealth: "up", CreatedAt: now, UpdatedAt: now},
		{ID: "monitor-history-degraded", AgentID: agent.ID, Name: "private degraded", Type: "http", Lifecycle: "active", Health: "down", ComputedHealth: "down", CreatedAt: now, UpdatedAt: now},
	}
	mappings := []db.StatusPageComponentMapping{
		{ID: "mapping-perfect", ComponentID: component.ID, ResourceType: "monitor", ResourceID: monitors[0].ID, HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: now, UpdatedAt: now},
		{ID: "mapping-degraded", ComponentID: component.ID, ResourceType: "monitor", ResourceID: monitors[1].ID, HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: now, UpdatedAt: now},
		{ID: "mapping-hidden", ComponentID: hiddenComponent.ID, ResourceType: "agent", ResourceID: agent.ID, HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: now, UpdatedAt: now},
	}
	reports := []db.MonitorReport{
		{ID: "report-perfect-up-1", MonitorID: monitors[0].ID, Payload: `{"secret":"private-perfect"}`, CollectedAt: now.Format(time.RFC3339), Health: "up", CreatedAt: now},
		{ID: "report-perfect-up-2", MonitorID: monitors[0].ID, Payload: `{"secret":"private-perfect"}`, CollectedAt: now.Add(-time.Minute).Format(time.RFC3339), Health: "up", CreatedAt: now.Add(-time.Minute)},
		{ID: "report-degraded-up", MonitorID: monitors[1].ID, Payload: `{"secret":"private-degraded"}`, CollectedAt: now.Format(time.RFC3339), Health: "up", CreatedAt: now},
		{ID: "report-degraded-down", MonitorID: monitors[1].ID, Payload: `{"secret":"private-degraded"}`, CollectedAt: now.Add(-time.Minute).Format(time.RFC3339), Health: "down", CreatedAt: now.Add(-time.Minute)},
	}

	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create section: %v", err)
	}
	if err := server.db.Create(&[]db.StatusPageComponent{component, hiddenComponent}).Error; err != nil {
		t.Fatalf("create components: %v", err)
	}
	if err := server.db.Model(&db.StatusPageComponent{}).Where("id = ?", hiddenComponent.ID).Update("visible", false).Error; err != nil {
		t.Fatalf("hide component: %v", err)
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}
	if err := server.db.Create(&mappings).Error; err != nil {
		t.Fatalf("create mappings: %v", err)
	}
	if err := server.db.Create(&reports).Error; err != nil {
		t.Fatalf("create reports: %v", err)
	}

	resp := performJSONRequest(t, server, http.MethodGet, "/status/uptime-status/components/"+component.ID+"/history?window=7d", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("component history status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	for _, privateValue := range []string{
		monitors[0].ID,
		monitors[1].ID,
		agent.ID,
		"secret-agent-token",
		"private-perfect",
		"private-degraded",
		"total",
		"up_count",
		"sample",
	} {
		if strings.Contains(body, privateValue) {
			t.Fatalf("public component history leaked %q in %s", privateValue, body)
		}
	}

	var parsed struct {
		Data struct {
			Component struct {
				ID            string `json:"id"`
				Status        string `json:"status"`
				StatusDisplay string `json:"status_display"`
			} `json:"component"`
			Uptime struct {
				Window        string   `json:"window"`
				UptimeRatio   *float64 `json:"uptime_ratio"`
				UptimeDisplay string   `json:"uptime_display"`
			} `json:"uptime"`
			History []struct {
				Date          string   `json:"date"`
				UptimeRatio   *float64 `json:"uptime_ratio"`
				UptimeDisplay string   `json:"uptime_display"`
			} `json:"history"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &parsed)
	if parsed.Data.Component.ID != component.ID || parsed.Data.Component.Status != "major_outage" || parsed.Data.Component.StatusDisplay != "Major outage" {
		t.Fatalf("component = %+v, want public major outage component", parsed.Data.Component)
	}
	if parsed.Data.Uptime.Window != "7d" || parsed.Data.Uptime.UptimeRatio == nil || *parsed.Data.Uptime.UptimeRatio != 0.5 || parsed.Data.Uptime.UptimeDisplay != "50.0%" {
		t.Fatalf("uptime = %+v, want worst mapped monitor uptime of 50.0%%", parsed.Data.Uptime)
	}
	if len(parsed.Data.History) != 7 {
		t.Fatalf("history bucket count = %d, want 7", len(parsed.Data.History))
	}

	hiddenResp := performJSONRequest(t, server, http.MethodGet, "/status/uptime-status/components/"+hiddenComponent.ID+"/history?window=7d", nil, "")
	if hiddenResp.Code != http.StatusNotFound {
		t.Fatalf("hidden component history status = %d, body = %s, want 404", hiddenResp.Code, hiddenResp.Body.String())
	}
}

func TestPublicStatusPageUptimeShowsUnknownAndNoData(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC()

	page := db.StatusPage{
		ID:                        "status-page-unknown-history-test",
		Slug:                      "unknown-status",
		Title:                     "Unknown Status",
		Visibility:                "unlisted",
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: "draft",
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	section := db.StatusPageSection{ID: "status-page-unknown-section", StatusPageID: page.ID, Name: "Core", CreatedAt: now, UpdatedAt: now}
	component := db.StatusPageComponent{
		ID:           "status-page-unknown-component",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Queue",
		DisplayMode:  "single_resource",
		Visible:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	mapping := db.StatusPageComponentMapping{
		ID:                   "mapping-unknown-monitor",
		ComponentID:          component.ID,
		ResourceType:         "monitor",
		ResourceID:           "missing-private-monitor",
		HealthRollupStrategy: "worst",
		UptimeRollupStrategy: "worst",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create section: %v", err)
	}
	if err := server.db.Create(&component).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := server.db.Create(&mapping).Error; err != nil {
		t.Fatalf("create mapping: %v", err)
	}

	resp := performJSONRequest(t, server, http.MethodGet, "/status/unknown-status/components/"+component.ID+"/uptime", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("component uptime status = %d, body = %s", resp.Code, resp.Body.String())
	}
	assertNotContains(t, resp.Body.String(), "missing-private-monitor")

	var parsed struct {
		Data struct {
			Component struct {
				Status        string `json:"status"`
				StatusDisplay string `json:"status_display"`
			} `json:"component"`
			Uptime struct {
				Window        string   `json:"window"`
				UptimeRatio   *float64 `json:"uptime_ratio"`
				UptimeDisplay string   `json:"uptime_display"`
			} `json:"uptime"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &parsed)
	if parsed.Data.Component.Status != "unknown" || parsed.Data.Component.StatusDisplay != "Unknown" {
		t.Fatalf("component = %+v, want public Unknown state", parsed.Data.Component)
	}
	if parsed.Data.Uptime.Window != "90d" || parsed.Data.Uptime.UptimeRatio != nil || parsed.Data.Uptime.UptimeDisplay != "No data" {
		t.Fatalf("uptime = %+v, want default 90d No data", parsed.Data.Uptime)
	}
}

func TestPublicStatusPageIncidentHistoryRoundsAndRedactsUpdates(t *testing.T) {
	server := setupTestServer(t)
	now := time.Date(2026, 5, 27, 12, 0, 40, 0, time.UTC)

	page := db.StatusPage{
		ID:                        "status-page-incident-history-test",
		Slug:                      "incident-history",
		Title:                     "Incident History",
		Visibility:                "public",
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: "draft",
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	incident := db.StatusPageIncident{
		ID:            "public-history-incident",
		StatusPageID:  page.ID,
		Title:         "API availability issue",
		PublicStatus:  "investigating",
		Severity:      "major",
		ImpactSummary: "Customer-safe summary.",
		Visibility:    "published",
		PublishedAt:   &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	publishedUpdate := db.StatusPageIncidentUpdate{
		ID:          "published-public-history-update",
		IncidentID:  incident.ID,
		Status:      "identified",
		Message:     "A public mitigation is in progress.",
		CreatedBy:   "internal-operator",
		PublishedAt: &now,
		CreatedAt:   now,
	}
	draftUpdate := db.StatusPageIncidentUpdate{
		ID:         "draft-public-history-update",
		IncidentID: incident.ID,
		Status:     "monitoring",
		Message:    "Internal-only secret update.",
		CreatedBy:  "internal-operator",
		CreatedAt:  now,
	}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	if err := server.db.Create(&incident).Error; err != nil {
		t.Fatalf("create incident: %v", err)
	}
	if err := server.db.Create(&[]db.StatusPageIncidentUpdate{publishedUpdate, draftUpdate}).Error; err != nil {
		t.Fatalf("create incident updates: %v", err)
	}

	resp := performJSONRequest(t, server, http.MethodGet, "/status/incident-history/incidents/"+incident.ID+"/history", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("incident history status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	assertContains(t, body, "A public mitigation is in progress.")
	assertContains(t, body, "2026-05-27T12:01:00Z")
	assertNotContains(t, body, "Internal-only secret update")
	assertNotContains(t, body, "internal-operator")
}
