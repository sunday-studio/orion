package api

import (
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"
)

func createStatusPageSuggestionFixtures(t *testing.T, server *Server, now time.Time) statusPageSuggestionFixtures {
	t.Helper()
	page := db.StatusPage{ID: "status-page-suggestions", Slug: "suggestions", Title: "Suggestions", Visibility: "draft", ThemeSettings: "{}", DefaultIncidentVisibility: "draft", CreatedAt: now, UpdatedAt: now}
	section := db.StatusPageSection{ID: "status-page-suggestions-section", StatusPageID: page.ID, Name: "Customer components", CreatedAt: now, UpdatedAt: now}
	agent := db.Agent{ID: "agent-private-suggestions", MachineId: "machine-private-suggestions", Name: "private-prod-agent-name", OS: "linux", Arch: "arm64", Token: "private-agent-token-value", LastSeen: now, CreatedAt: now}
	monitor := db.Monitor{ID: "monitor-private-suggestions", AgentID: agent.ID, Name: "private-checkout-monitor", Type: "http", Lifecycle: "active", Health: "down", ComputedHealth: "down", CreatedAt: now, UpdatedAt: now}
	monitorComponent := db.StatusPageComponent{ID: "status-page-public-monitor-component", StatusPageID: page.ID, SectionID: section.ID, PublicName: "Checkout API", DisplayMode: "single_resource", SortOrder: 1, Visible: true, CreatedAt: now, UpdatedAt: now}
	agentComponent := db.StatusPageComponent{ID: "status-page-public-agent-component", StatusPageID: page.ID, SectionID: section.ID, PublicName: "Core Platform", DisplayMode: "single_resource", SortOrder: 2, Visible: true, CreatedAt: now, UpdatedAt: now}
	hiddenComponent := db.StatusPageComponent{ID: "status-page-hidden-private-component", StatusPageID: page.ID, SectionID: section.ID, PublicName: "Hidden private database", DisplayMode: "single_resource", SortOrder: 3, Visible: false, CreatedAt: now, UpdatedAt: now}
	mappings := []db.StatusPageComponentMapping{{ID: "status-page-monitor-suggestion-mapping", ComponentID: monitorComponent.ID, ResourceType: "monitor", ResourceID: monitor.ID, HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: now, UpdatedAt: now}, {ID: "status-page-agent-suggestion-mapping", ComponentID: agentComponent.ID, ResourceType: "agent", ResourceID: agent.ID, HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: now, UpdatedAt: now}, {ID: "status-page-hidden-suggestion-mapping", ComponentID: hiddenComponent.ID, ResourceType: "monitor", ResourceID: monitor.ID, HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: now, UpdatedAt: now}}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create status page section: %v", err)
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	if err := server.db.Create(&[]db.StatusPageComponent{monitorComponent, agentComponent, hiddenComponent}).Error; err != nil {
		t.Fatalf("create status page components: %v", err)
	}
	if err := server.db.Model(&db.StatusPageComponent{}).Where("id = ?", hiddenComponent.ID).Update("visible", false).Error; err != nil {
		t.Fatalf("hide status page component: %v", err)
	}
	if err := server.db.Create(&mappings).Error; err != nil {
		t.Fatalf("create status page component mappings: %v", err)
	}
	return statusPageSuggestionFixtures{Page: page, Agent: agent, Monitor: monitor, MonitorComponent: monitorComponent, AgentComponent: agentComponent, HiddenComponent: hiddenComponent}
}
func assertSafePublicDraftCopy(t *testing.T, draft StatusPageIncidentDraftResponse, privateValues []string) {
	t.Helper()
	copyText := strings.Join([]string{draft.Title, draft.ImpactSummary, draft.InitialUpdateMessage}, " ")
	if !strings.Contains(copyText, "Checkout API") {
		t.Fatalf("draft copy = %q, want public component name", copyText)
	}
	for _, privateValue := range privateValues {
		if privateValue == "" {
			continue
		}
		if strings.Contains(copyText, privateValue) {
			t.Fatalf("draft copy leaked %q in %q", privateValue, copyText)
		}
	}
}
