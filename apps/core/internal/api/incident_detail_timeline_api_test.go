package api

import (
	"net/http"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"
)

func TestIncidentDetailAndTimelineEndpoints(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	createIncidentImpactStatusPageMapping(t, server, registeredMonitor.Data.MonitorID, registered.Data.AgentID, "incident-detail-component", "Checkout", "agent")
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	downResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "down", "metrics": map[string]interface{}{"status_code": 500}}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	relatedIncident := db.Incident{ID: "incident-related-detail", Status: "resolved", Severity: "medium", Title: "Prior homepage failure", AgentID: registered.Data.AgentID, MonitorID: registeredMonitor.Data.MonitorID, OpenedAt: incident.OpenedAt.Add(-2 * time.Hour), ResolvedAt: &incident.OpenedAt, LastEventAt: incident.OpenedAt, LatestEvent: "Prior incident resolved", NotificationStatus: "suppressed", ResolutionKind: "recovered"}
	if err := server.db.Create(&relatedIncident).Error; err != nil {
		t.Fatalf("create related incident: %v", err)
	}
	latestResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "degraded", "metrics": map[string]interface{}{"status_code": 503, "message": "slow checkout"}}, registered.Data.Token)
	if latestResp.Code != http.StatusOK {
		t.Fatalf("latest report status = %d, body = %s", latestResp.Code, latestResp.Body.String())
	}
	now := time.Now().UTC()
	failedDelivery := db.AlertDelivery{ID: "delivery-failed-incident-detail", IncidentID: incident.ID, EventType: "incident_opened", Channel: "Primary webhook", Type: "webhook", Status: "failed", Error: "context deadline exceeded", AttemptCount: 1, MaxAttempts: 3, LastAttemptAt: &now, CreatedAt: now, UpdatedAt: now}
	if err := server.db.Create(&failedDelivery).Error; err != nil {
		t.Fatalf("create failed delivery: %v", err)
	}
	detailResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID, nil, "")
	if detailResp.Code != http.StatusOK {
		t.Fatalf("incident detail status = %d, body = %s", detailResp.Code, detailResp.Body.String())
	}
	var detail struct {
		Success bool `json:"success"`
		Data    struct {
			Incident struct {
				ID                 string `json:"id"`
				AgentName          string `json:"agent_name"`
				MonitorName        string `json:"monitor_name"`
				ImpactedComponents []struct {
					ComponentID   string `json:"component_id"`
					ComponentName string `json:"component_name"`
					Status        string `json:"status"`
					Impact        string `json:"impact"`
				} `json:"impacted_components"`
			} `json:"incident"`
			Evidence struct {
				TriggeringReport *struct {
					ID      string `json:"id"`
					Health  string `json:"health"`
					Payload string `json:"payload"`
				} `json:"triggering_report"`
				LatestReport *struct {
					ID      string `json:"id"`
					Health  string `json:"health"`
					Payload string `json:"payload"`
				} `json:"latest_report"`
			} `json:"evidence"`
			RelatedIncidents []struct {
				ID             string `json:"id"`
				ResolutionKind string `json:"resolution_kind"`
			} `json:"related_incidents"`
			NextActions []struct {
				ID           string `json:"id"`
				ActionType   string `json:"action_type"`
				TargetKind   string `json:"target_kind"`
				TargetID     string `json:"target_id"`
				TargetTab    string `json:"target_tab"`
				FilterStatus string `json:"filter_status"`
			} `json:"next_actions"`
			Timeline []struct {
				Type            string `json:"type"`
				Source          string `json:"source"`
				Evidence        string `json:"evidence"`
				MonitorReportID string `json:"monitor_report_id"`
				AlertDeliveryID string `json:"alert_delivery_id"`
			} `json:"timeline"`
			AlertDeliveries []struct {
				Status string `json:"status"`
			} `json:"alert_deliveries"`
			MonitorReports []struct {
				Health string `json:"health"`
			} `json:"monitor_reports"`
		} `json:"data"`
	}
	decodeResponse(t, detailResp, &detail)
	if !detail.Success || detail.Data.Incident.ID != incident.ID {
		t.Fatalf("incident detail response = %+v, want incident %s", detail, incident.ID)
	}
	if detail.Data.Incident.AgentName != "test-server" || detail.Data.Incident.MonitorName != "homepage" {
		t.Fatalf("incident names = %+v, want agent and monitor names", detail.Data.Incident)
	}
	if len(detail.Data.Incident.ImpactedComponents) != 1 || detail.Data.Incident.ImpactedComponents[0].ComponentID != "incident-detail-component" || detail.Data.Incident.ImpactedComponents[0].ComponentName != "Checkout" || detail.Data.Incident.ImpactedComponents[0].Status != "degraded" || detail.Data.Incident.ImpactedComponents[0].Impact != "degraded" {
		t.Fatalf("incident detail component impact = %+v, want Checkout degraded impact", detail.Data.Incident.ImpactedComponents)
	}
	if len(detail.Data.Timeline) < 2 || len(detail.Data.AlertDeliveries) == 0 || len(detail.Data.MonitorReports) == 0 {
		t.Fatalf("incident detail linked data missing: %+v", detail.Data)
	}
	if detail.Data.Evidence.TriggeringReport == nil || detail.Data.Evidence.TriggeringReport.Health != "down" || !strings.Contains(detail.Data.Evidence.TriggeringReport.Payload, `"status_code":500`) {
		t.Fatalf("triggering evidence = %+v, want down report with safe payload", detail.Data.Evidence.TriggeringReport)
	}
	if detail.Data.Evidence.LatestReport == nil || detail.Data.Evidence.LatestReport.Health != "degraded" || !strings.Contains(detail.Data.Evidence.LatestReport.Payload, "slow checkout") {
		t.Fatalf("latest evidence = %+v, want latest degraded report", detail.Data.Evidence.LatestReport)
	}
	if len(detail.Data.RelatedIncidents) != 1 || detail.Data.RelatedIncidents[0].ID != relatedIncident.ID || detail.Data.RelatedIncidents[0].ResolutionKind != "recovered" {
		t.Fatalf("related incidents = %+v, want prior same-monitor incident", detail.Data.RelatedIncidents)
	}
	var hasMonitorTuningAction bool
	var hasNotificationRecoveryAction bool
	for _, action := range detail.Data.NextActions {
		if action.ActionType == "review_monitor_tuning" && action.TargetKind == "monitor" && action.TargetID == registeredMonitor.Data.MonitorID && action.TargetTab == "config" {
			hasMonitorTuningAction = true
		}
		if action.ActionType == "review_failed_notifications" && action.TargetKind == "alert_deliveries" && action.TargetID == incident.ID && action.FilterStatus == "failed" {
			hasNotificationRecoveryAction = true
		}
	}
	if !hasMonitorTuningAction || !hasNotificationRecoveryAction {
		t.Fatalf("next actions = %+v, want monitor tuning and notification recovery actions", detail.Data.NextActions)
	}
	var reportLinked bool
	var deliveryLinked bool
	for _, item := range detail.Data.Timeline {
		if item.MonitorReportID != "" && item.Evidence != "" {
			reportLinked = true
		}
		if item.AlertDeliveryID != "" {
			deliveryLinked = true
		}
	}
	if !reportLinked || !deliveryLinked {
		t.Fatalf("timeline links = %+v, want report evidence and alert delivery links", detail.Data.Timeline)
	}
	timelineResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID+"/timeline", nil, "")
	if timelineResp.Code != http.StatusOK {
		t.Fatalf("incident timeline status = %d, body = %s", timelineResp.Code, timelineResp.Body.String())
	}
	var timeline struct {
		Success bool `json:"success"`
		Data    struct {
			Timeline []struct {
				Source string `json:"source"`
			} `json:"timeline"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, timelineResp, &timeline)
	if !timeline.Success || timeline.Data.Count < 2 {
		t.Fatalf("timeline response = %+v, want incident and alert events", timeline)
	}
}
func createIncidentImpactStatusPageMapping(t *testing.T, server *Server, monitorID string, agentID string, componentID string, componentName string, resourceType string) {
	t.Helper()
	now := time.Now().UTC()
	page := db.StatusPage{ID: componentID + "-page", Slug: componentID + "-page", Title: componentName + " Status", Visibility: statusPageVisibilityPublic, ThemeSettings: "{}", DefaultIncidentVisibility: statusPageIncidentVisibilityDraft, PublishedAt: &now}
	section := db.StatusPageSection{ID: componentID + "-section", StatusPageID: page.ID, Name: "Public services"}
	component := db.StatusPageComponent{ID: componentID, StatusPageID: page.ID, SectionID: section.ID, PublicName: componentName, DisplayMode: "single_resource", SortOrder: 1, Visible: true}
	resourceID := monitorID
	if resourceType == "agent" {
		resourceID = agentID
	}
	mapping := db.StatusPageComponentMapping{ID: componentID + "-mapping", ComponentID: component.ID, ResourceType: resourceType, ResourceID: resourceID, HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst"}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page for incident impact: %v", err)
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create status page section for incident impact: %v", err)
	}
	if err := server.db.Create(&component).Error; err != nil {
		t.Fatalf("create status page component for incident impact: %v", err)
	}
	if err := server.db.Create(&mapping).Error; err != nil {
		t.Fatalf("create status page component mapping for incident impact: %v", err)
	}
}
func TestMaintenanceSuppressesAutomaticIncidentOpen(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	maintenanceResp := performJSONRequest(t, server, http.MethodPut, "/v1/agents/"+registered.Data.AgentID+"/maintenance", map[string]bool{"maintenance_mode": true}, registered.Data.Token)
	if maintenanceResp.Code != http.StatusOK {
		t.Fatalf("maintenance status = %d, body = %s", maintenanceResp.Code, maintenanceResp.Body.String())
	}
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	downResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "down", "metrics": map[string]interface{}{}}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}
	var incidentCount int64
	if err := server.db.Model(&db.Incident{}).Where("monitor_id = ?", registeredMonitor.Data.MonitorID).Count(&incidentCount).Error; err != nil {
		t.Fatalf("count incidents: %v", err)
	}
	if incidentCount != 0 {
		t.Fatalf("incident count = %d, want 0", incidentCount)
	}
}
func TestTLSExpiryMetricOpensAndResolvesIncident(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	expiringResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "up", "metrics": map[string]interface{}{"tls_days_remaining": 3}}, registered.Data.Token)
	if expiringResp.Code != http.StatusOK {
		t.Fatalf("expiring TLS report status = %d, body = %s", expiringResp.Code, expiringResp.Body.String())
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	if incident.Status != "open" || incident.Severity != "medium" {
		t.Fatalf("incident = %+v, want open medium", incident)
	}
	healthyResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "up", "metrics": map[string]interface{}{"tls_days_remaining": 90}}, registered.Data.Token)
	if healthyResp.Code != http.StatusOK {
		t.Fatalf("healthy TLS report status = %d, body = %s", healthyResp.Code, healthyResp.Body.String())
	}
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload incident: %v", err)
	}
	if incident.Status != "resolved" {
		t.Fatalf("incident status = %q, want resolved", incident.Status)
	}
}
func TestAgentReportOpensStaleMonitorIncident(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	if err := server.db.Model(&db.Monitor{}).Where("id = ?", registeredMonitor.Data.MonitorID).Update("created_at", time.Now().Add(-20*time.Minute)).Error; err != nil {
		t.Fatalf("age monitor: %v", err)
	}
	reportBody := map[string]interface{}{"uptime_seconds": 120, "timestamp": time.Now().UTC().Format(time.RFC3339), "cpu": map[string]interface{}{}, "memory": map[string]interface{}{}, "disk": map[string]interface{}{}}
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/report"
	reportResp := performJSONRequest(t, server, http.MethodPost, reportPath, reportBody, registered.Data.Token)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("agent report status = %d, body = %s", reportResp.Code, reportResp.Body.String())
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	if incident.Status != "open" || incident.Severity != "high" {
		t.Fatalf("incident = %+v, want open high stale incident", incident)
	}
}
