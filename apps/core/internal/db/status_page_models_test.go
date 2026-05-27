package db

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestStatusPagePublicationModelsCanBeCreatedWithoutChangingAgentReporting(t *testing.T) {
	database := openStatusPageModelTestDatabase(t)
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)

	agent := Agent{
		ID:                       "agent-publication-test",
		MachineId:                "machine-publication-test",
		Name:                     "Internal API Node",
		OS:                       "darwin",
		Arch:                     "arm64",
		Token:                    "token-publication-test",
		ReportingIntervalSeconds: 60,
		CreatedAt:                now,
		LastSeen:                 now,
	}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitor := Monitor{
		ID:                       "monitor-publication-test",
		Type:                     "http",
		Name:                     "Internal HTTP Check",
		AgentID:                  agent.ID,
		ReportingIntervalSeconds: 60,
		ComputedHealth:           "up",
		Lifecycle:                "active",
		Health:                   "up",
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := database.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}

	beforeReport := AgentReport{
		ID:            "agent-report-before-status-page",
		AgentID:       agent.ID,
		CreatedAt:     now,
		AgentVersion:  "test",
		ConfigSummary: "{}",
		Timestamp:     now.Format(time.RFC3339),
	}
	if err := database.Create(&beforeReport).Error; err != nil {
		t.Fatalf("create baseline agent report: %v", err)
	}

	page := StatusPage{
		ID:                        "status-page-publication-test",
		Slug:                      "public-status",
		Title:                     "Public Status",
		Description:               "Public-facing service health",
		Visibility:                "draft",
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: "draft",
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := database.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}

	section := StatusPageSection{
		ID:           "status-page-section-api",
		StatusPageID: page.ID,
		Name:         "API",
		SortOrder:    10,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := database.Create(&section).Error; err != nil {
		t.Fatalf("create status page section: %v", err)
	}

	component := StatusPageComponent{
		ID:                "status-page-component-api",
		StatusPageID:      page.ID,
		SectionID:         section.ID,
		PublicName:        "API",
		PublicDescription: "Public API availability",
		DisplayMode:       "single_resource",
		SortOrder:         10,
		Visible:           true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := database.Create(&component).Error; err != nil {
		t.Fatalf("create status page component: %v", err)
	}

	mapping := StatusPageComponentMapping{
		ID:                   "status-page-mapping-api-monitor",
		ComponentID:          component.ID,
		ResourceType:         "monitor",
		ResourceID:           monitor.ID,
		HealthRollupStrategy: "worst",
		UptimeRollupStrategy: "worst",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := database.Create(&mapping).Error; err != nil {
		t.Fatalf("create status page component mapping: %v", err)
	}

	internalIncident := Incident{
		ID:                 "incident-publication-test",
		Status:             "open",
		Severity:           "critical",
		Title:              "Internal incident title",
		AgentID:            agent.ID,
		MonitorID:          monitor.ID,
		OpenedAt:           now,
		LastEventAt:        now,
		NotificationStatus: "pending",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := database.Create(&internalIncident).Error; err != nil {
		t.Fatalf("create internal incident: %v", err)
	}

	publishedAt := now.Add(5 * time.Minute)
	publicIncident := StatusPageIncident{
		ID:                   "status-page-incident-api",
		StatusPageID:         page.ID,
		InternalIncidentID:   internalIncident.ID,
		Title:                "API availability issue",
		PublicStatus:         "investigating",
		Severity:             "critical",
		ImpactSummary:        "Some API requests are failing.",
		Visibility:           "published",
		AffectedComponentIDs: `["status-page-component-api"]`,
		PublishedAt:          &publishedAt,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := database.Create(&publicIncident).Error; err != nil {
		t.Fatalf("create public incident: %v", err)
	}

	update := StatusPageIncidentUpdate{
		ID:          "status-page-incident-update-api",
		IncidentID:  publicIncident.ID,
		Status:      "investigating",
		Message:     "We are investigating API errors.",
		CreatedBy:   "admin",
		PublishedAt: &publishedAt,
		CreatedAt:   now,
	}
	if err := database.Create(&update).Error; err != nil {
		t.Fatalf("create public incident update: %v", err)
	}

	afterReport := AgentReport{
		ID:            "agent-report-after-status-page",
		AgentID:       agent.ID,
		CreatedAt:     now.Add(10 * time.Minute),
		AgentVersion:  "test",
		ConfigSummary: "{}",
		Timestamp:     now.Add(10 * time.Minute).Format(time.RFC3339),
	}
	if err := database.Create(&afterReport).Error; err != nil {
		t.Fatalf("create agent report after status page publication records: %v", err)
	}

	assertModelCount(t, database, &StatusPage{}, 1)
	assertModelCount(t, database, &StatusPageSection{}, 1)
	assertModelCount(t, database, &StatusPageComponent{}, 1)
	assertModelCount(t, database, &StatusPageComponentMapping{}, 1)
	assertModelCount(t, database, &StatusPageIncident{}, 1)
	assertModelCount(t, database, &StatusPageIncidentUpdate{}, 1)
	assertModelCount(t, database, &AgentReport{}, 2)

	var loadedMapping StatusPageComponentMapping
	if err := database.Where("component_id = ? AND resource_type = ? AND resource_id = ?", component.ID, "monitor", monitor.ID).First(&loadedMapping).Error; err != nil {
		t.Fatalf("load component mapping: %v", err)
	}
	if loadedMapping.HealthRollupStrategy != "worst" {
		t.Fatalf("health rollup strategy = %q, want worst", loadedMapping.HealthRollupStrategy)
	}
}

func openStatusPageModelTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return database
}

func assertModelCount(t *testing.T, database *gorm.DB, model any, want int64) {
	t.Helper()

	var count int64
	if err := database.Model(model).Count(&count).Error; err != nil {
		t.Fatalf("count model %T: %v", model, err)
	}
	if count != want {
		t.Fatalf("count model %T = %d, want %d", model, count, want)
	}
}
