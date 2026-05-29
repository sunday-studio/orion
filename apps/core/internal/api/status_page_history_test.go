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
				Status        string   `json:"status"`
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

func TestPublicStatusPageProjectionIncludesSafeUptimeHistory(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC().Add(-24 * time.Hour)

	page := db.StatusPage{
		ID:                        "status-page-public-uptime-projection",
		Slug:                      "public-uptime-projection",
		Title:                     "Projection Status",
		Visibility:                "public",
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: "draft",
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	section := db.StatusPageSection{
		ID:           "public-uptime-section",
		StatusPageID: page.ID,
		Name:         "Core",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	monitorComponent := db.StatusPageComponent{
		ID:           "public-component-monitor",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "API",
		DisplayMode:  "single_resource",
		SortOrder:    1,
		Visible:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	agentComponent := db.StatusPageComponent{
		ID:           "public-component-agent",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Workers",
		DisplayMode:  "single_resource",
		SortOrder:    2,
		Visible:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	noDataComponent := db.StatusPageComponent{
		ID:           "public-component-no-data",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Queue",
		DisplayMode:  "single_resource",
		SortOrder:    3,
		Visible:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	hiddenComponent := db.StatusPageComponent{
		ID:           "hidden-public-component",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Hidden database",
		DisplayMode:  "single_resource",
		SortOrder:    4,
		Visible:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	monitorAgent := db.Agent{ID: "internal-agent-monitor", MachineId: "internal-machine-monitor", Name: "private monitor host", OS: "linux", Arch: "arm64", Token: "private-monitor-agent-token", LastSeen: now, CreatedAt: now}
	workerAgent := db.Agent{ID: "internal-agent-workers", MachineId: "internal-machine-workers", Name: "private worker host", OS: "linux", Arch: "arm64", Token: "private-worker-agent-token", LastSeen: now, CreatedAt: now}
	monitor := db.Monitor{ID: "internal-monitor-api", AgentID: monitorAgent.ID, Name: "private api monitor", Type: "http", Lifecycle: "active", Health: "down", ComputedHealth: "down", CreatedAt: now, UpdatedAt: now}
	workerMonitor := db.Monitor{ID: "internal-monitor-workers", AgentID: workerAgent.ID, Name: "private worker monitor", Type: "http", Lifecycle: "active", Health: "up", ComputedHealth: "up", CreatedAt: now, UpdatedAt: now}
	mappings := []db.StatusPageComponentMapping{
		{ID: "public-uptime-monitor-mapping", ComponentID: monitorComponent.ID, ResourceType: "monitor", ResourceID: monitor.ID, HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: now, UpdatedAt: now},
		{ID: "public-uptime-agent-mapping", ComponentID: agentComponent.ID, ResourceType: "agent", ResourceID: workerAgent.ID, HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: now, UpdatedAt: now},
		{ID: "public-uptime-no-data-mapping", ComponentID: noDataComponent.ID, ResourceType: "monitor", ResourceID: "missing-private-resource", HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: now, UpdatedAt: now},
		{ID: "public-uptime-hidden-mapping", ComponentID: hiddenComponent.ID, ResourceType: "monitor", ResourceID: "hidden-private-resource", HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: now, UpdatedAt: now},
	}
	reports := []db.MonitorReport{
		{ID: "projection-api-up", MonitorID: monitor.ID, Payload: `{"secret":"private-api-up"}`, CollectedAt: now.Format(time.RFC3339), Health: "up", CreatedAt: now},
		{ID: "projection-api-down", MonitorID: monitor.ID, Payload: `{"secret":"private-api-down"}`, CollectedAt: now.Add(time.Minute).Format(time.RFC3339), Health: "down", CreatedAt: now.Add(time.Minute)},
		{ID: "projection-worker-up", MonitorID: workerMonitor.ID, Payload: `{"secret":"private-worker-up"}`, CollectedAt: now.Format(time.RFC3339), Health: "up", CreatedAt: now},
	}

	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create section: %v", err)
	}
	if err := server.db.Create(&[]db.StatusPageComponent{monitorComponent, agentComponent, noDataComponent, hiddenComponent}).Error; err != nil {
		t.Fatalf("create components: %v", err)
	}
	if err := server.db.Model(&db.StatusPageComponent{}).Where("id = ?", hiddenComponent.ID).Update("visible", false).Error; err != nil {
		t.Fatalf("hide component: %v", err)
	}
	if err := server.db.Create(&[]db.Agent{monitorAgent, workerAgent}).Error; err != nil {
		t.Fatalf("create agents: %v", err)
	}
	if err := server.db.Create(&[]db.Monitor{monitor, workerMonitor}).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}
	if err := server.db.Create(&mappings).Error; err != nil {
		t.Fatalf("create mappings: %v", err)
	}
	if err := server.db.Create(&reports).Error; err != nil {
		t.Fatalf("create reports: %v", err)
	}

	resp := performJSONRequest(t, server, http.MethodGet, "/status/public-uptime-projection?format=json", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("public projection status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	for _, privateValue := range []string{
		monitor.ID,
		workerMonitor.ID,
		monitorAgent.ID,
		workerAgent.ID,
		monitorAgent.Token,
		workerAgent.Token,
		"missing-private-resource",
		"hidden-private-resource",
		"private-api",
		"private-worker",
	} {
		if strings.Contains(body, privateValue) {
			t.Fatalf("public projection leaked %q in %s", privateValue, body)
		}
	}
	assertNotContains(t, body, hiddenComponent.ID)

	var parsed struct {
		Data struct {
			StatusPage struct {
				Sections []struct {
					Components []struct {
						ID     string `json:"id"`
						Name   string `json:"name"`
						Status string `json:"status"`
						Uptime struct {
							Window        string   `json:"window"`
							Status        string   `json:"status"`
							UptimeRatio   *float64 `json:"uptime_ratio"`
							UptimeDisplay string   `json:"uptime_display"`
						} `json:"uptime"`
						UptimeHistory []struct {
							Date          string   `json:"date"`
							Status        string   `json:"status"`
							UptimeRatio   *float64 `json:"uptime_ratio"`
							UptimeDisplay string   `json:"uptime_display"`
						} `json:"uptime_history"`
					} `json:"components"`
				} `json:"sections"`
			} `json:"status_page"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &parsed)
	components := map[string]struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
		Uptime struct {
			Window        string   `json:"window"`
			Status        string   `json:"status"`
			UptimeRatio   *float64 `json:"uptime_ratio"`
			UptimeDisplay string   `json:"uptime_display"`
		} `json:"uptime"`
		UptimeHistory []struct {
			Date          string   `json:"date"`
			Status        string   `json:"status"`
			UptimeRatio   *float64 `json:"uptime_ratio"`
			UptimeDisplay string   `json:"uptime_display"`
		} `json:"uptime_history"`
	}{}
	for _, section := range parsed.Data.StatusPage.Sections {
		for _, component := range section.Components {
			components[component.Name] = component
		}
	}
	if len(components) != 3 {
		t.Fatalf("public component count = %d, want 3 visible components", len(components))
	}
	if components["API"].Uptime.Window != "90d" || components["API"].Uptime.Status != "degraded" || components["API"].Uptime.UptimeRatio == nil || *components["API"].Uptime.UptimeRatio != 0.5 {
		t.Fatalf("API uptime = %+v, want degraded 50%% monitor uptime", components["API"].Uptime)
	}
	if len(components["API"].UptimeHistory) != 90 || components["API"].UptimeHistory[len(components["API"].UptimeHistory)-1].Status != "degraded" {
		t.Fatalf("API uptime history = %+v, want 90 buckets ending degraded", components["API"].UptimeHistory)
	}
	if components["Workers"].Uptime.Status != "operational" || components["Workers"].Uptime.UptimeRatio == nil || *components["Workers"].Uptime.UptimeRatio != 1 {
		t.Fatalf("Workers uptime = %+v, want operational agent-mapped uptime", components["Workers"].Uptime)
	}
	if components["Queue"].Uptime.Status != "no_data" || components["Queue"].Uptime.UptimeRatio != nil || components["Queue"].Uptime.UptimeDisplay != "No data" {
		t.Fatalf("Queue uptime = %+v, want no-data uptime", components["Queue"].Uptime)
	}
	if len(components["Queue"].UptimeHistory) != 90 || components["Queue"].UptimeHistory[0].Status != "no_data" {
		t.Fatalf("Queue uptime history = %+v, want distinct no-data buckets", components["Queue"].UptimeHistory)
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
