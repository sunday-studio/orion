package api

import (
	"net/http"
	"orion/core/internal/db"
	"testing"
	"time"
)

func TestUnregisterMonitorClearsStaleIncidentPathWithoutDuplicateEvent(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	now := time.Now().UTC()
	incident := db.Incident{ID: "incident-stale-active-path", Status: "resolved", Severity: "high", Title: "Resolved stale active path", AgentID: registered.Data.AgentID, MonitorID: registeredMonitor.Data.MonitorID, OpenedAt: now.Add(-30 * time.Minute), ResolvedAt: &now, LastEventAt: now, LatestEvent: "Already resolved", NotificationStatus: "suppressed"}
	if err := server.db.Create(&incident).Error; err != nil {
		t.Fatalf("create resolved incident: %v", err)
	}
	if err := server.db.Model(&db.Monitor{}).Where("id = ?", registeredMonitor.Data.MonitorID).Updates(map[string]interface{}{"active_incident_id": incident.ID, "incident_state": "down"}).Error; err != nil {
		t.Fatalf("set stale monitor incident path: %v", err)
	}
	unregisterResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/unregister-monitor", map[string]interface{}{"monitor_id": registeredMonitor.Data.MonitorID}, registered.Data.Token)
	if unregisterResp.Code != http.StatusOK {
		t.Fatalf("unregister monitor status = %d, body = %s", unregisterResp.Code, unregisterResp.Body.String())
	}
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, "", "unknown")
	var resolvedEventCount int64
	if err := server.db.Model(&db.IncidentEvent{}).Where("incident_id = ? AND type = ?", incident.ID, "incident_resolved").Count(&resolvedEventCount).Error; err != nil {
		t.Fatalf("count resolved events: %v", err)
	}
	if resolvedEventCount != 0 {
		t.Fatalf("resolved event count = %d, want 0 duplicate events", resolvedEventCount)
	}
}
func TestListIncidentsReturnsPersistedActiveIncidents(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	createIncidentImpactStatusPageMapping(t, server, registeredMonitor.Data.MonitorID, registered.Data.AgentID, "incident-list-component", "Public API", "monitor")
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	reportResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "down", "metrics": map[string]interface{}{"status_code": 500}}, registered.Data.Token)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("monitor report status = %d, body = %s", reportResp.Code, reportResp.Body.String())
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find list incident: %v", err)
	}
	incidentsResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents", nil, "")
	if incidentsResp.Code != http.StatusOK {
		t.Fatalf("incidents status = %d, body = %s", incidentsResp.Code, incidentsResp.Body.String())
	}
	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Incidents []struct {
				Status             string `json:"status"`
				AgentID            string `json:"agent_id"`
				AgentName          string `json:"agent_name"`
				MonitorName        string `json:"monitor_name"`
				ImpactedComponents []struct {
					ComponentID   string `json:"component_id"`
					ComponentName string `json:"component_name"`
					Status        string `json:"status"`
					Impact        string `json:"impact"`
				} `json:"impacted_components"`
			} `json:"incidents"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, incidentsResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || len(listed.Data.Incidents) != 1 {
		t.Fatalf("incidents response = %+v, want one active incident", listed)
	}
	if listed.Data.Incidents[0].Status != "open" || listed.Data.Incidents[0].AgentName != "test-server" || listed.Data.Incidents[0].MonitorName != "homepage" {
		t.Fatalf("incident row = %+v, want open homepage on test-server", listed.Data.Incidents[0])
	}
	if len(listed.Data.Incidents[0].ImpactedComponents) != 1 || listed.Data.Incidents[0].ImpactedComponents[0].ComponentID != "incident-list-component" || listed.Data.Incidents[0].ImpactedComponents[0].ComponentName != "Public API" || listed.Data.Incidents[0].ImpactedComponents[0].Status != "major_outage" || listed.Data.Incidents[0].ImpactedComponents[0].Impact != "down" {
		t.Fatalf("incident component impact = %+v, want Public API down impact", listed.Data.Incidents[0].ImpactedComponents)
	}
	filteredResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?agent_id="+registered.Data.AgentID, nil, "")
	if filteredResp.Code != http.StatusOK {
		t.Fatalf("filtered incidents status = %d, body = %s", filteredResp.Code, filteredResp.Body.String())
	}
	var filtered struct {
		Success bool `json:"success"`
		Data    struct {
			Incidents []struct {
				AgentID string `json:"agent_id"`
			} `json:"incidents"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, filteredResp, &filtered)
	if !filtered.Success || filtered.Data.Count != 1 || filtered.Data.Incidents[0].AgentID != registered.Data.AgentID {
		t.Fatalf("filtered incidents response = %+v, want one incident for agent %s", filtered, registered.Data.AgentID)
	}
	monitorFilteredResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?monitor_id="+registeredMonitor.Data.MonitorID, nil, "")
	if monitorFilteredResp.Code != http.StatusOK {
		t.Fatalf("monitor filtered incidents status = %d, body = %s", monitorFilteredResp.Code, monitorFilteredResp.Body.String())
	}
	var monitorFiltered struct {
		Data struct {
			Incidents []struct {
				MonitorName string `json:"monitor_name"`
			} `json:"incidents"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, monitorFilteredResp, &monitorFiltered)
	if monitorFiltered.Data.Count != 1 || monitorFiltered.Data.Incidents[0].MonitorName != "homepage" {
		t.Fatalf("monitor filtered incidents = %+v, want homepage incident", monitorFiltered)
	}
	noMatchResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?agent_id=agent-no-match", nil, "")
	if noMatchResp.Code != http.StatusOK {
		t.Fatalf("no match incidents status = %d, body = %s", noMatchResp.Code, noMatchResp.Body.String())
	}
	var noMatch struct {
		Data struct {
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, noMatchResp, &noMatch)
	if noMatch.Data.Count != 0 {
		t.Fatalf("no match incident count = %d, want 0", noMatch.Data.Count)
	}
	highSeverityReviewResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?needs_review=true", nil, "")
	if highSeverityReviewResp.Code != http.StatusOK {
		t.Fatalf("high severity review incidents status = %d, body = %s", highSeverityReviewResp.Code, highSeverityReviewResp.Body.String())
	}
	var highSeverityReview struct {
		Data struct {
			Count     int64 `json:"count"`
			Incidents []struct {
				Severity string `json:"severity"`
			} `json:"incidents"`
		} `json:"data"`
	}
	decodeResponse(t, highSeverityReviewResp, &highSeverityReview)
	if highSeverityReview.Data.Count != 1 || highSeverityReview.Data.Incidents[0].Severity != "high" {
		t.Fatalf("high severity review incidents = %+v, want high severity incident", highSeverityReview)
	}
	if err := server.db.Model(&db.Incident{}).Where("monitor_id = ?", registeredMonitor.Data.MonitorID).Update("notification_status", "failed").Error; err != nil {
		t.Fatalf("mark incident notification failed: %v", err)
	}
	needsReviewResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?needs_review=true", nil, "")
	if needsReviewResp.Code != http.StatusOK {
		t.Fatalf("needs review incidents status = %d, body = %s", needsReviewResp.Code, needsReviewResp.Body.String())
	}
	var needsReview struct {
		Data struct {
			Count     int64 `json:"count"`
			Incidents []struct {
				NotificationStatus string `json:"notification_status"`
			} `json:"incidents"`
		} `json:"data"`
	}
	decodeResponse(t, needsReviewResp, &needsReview)
	if needsReview.Data.Count != 1 || needsReview.Data.Incidents[0].NotificationStatus != "failed" {
		t.Fatalf("needs review incidents = %+v, want failed notification incident", needsReview)
	}
	ackResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/acknowledge", nil, "")
	if ackResp.Code != http.StatusOK {
		t.Fatalf("acknowledge list incident status = %d, body = %s", ackResp.Code, ackResp.Body.String())
	}
	resolveResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/resolve", nil, "")
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("resolve list incident status = %d, body = %s", resolveResp.Code, resolveResp.Body.String())
	}
	secondReportResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "down", "metrics": map[string]interface{}{"status_code": 503}}, registered.Data.Token)
	if secondReportResp.Code != http.StatusOK {
		t.Fatalf("second monitor report status = %d, body = %s", secondReportResp.Code, secondReportResp.Body.String())
	}
	var coveredIncident db.Incident
	if err := server.db.Where("monitor_id = ? AND status = ?", registeredMonitor.Data.MonitorID, "open").First(&coveredIncident).Error; err != nil {
		t.Fatalf("find second list incident: %v", err)
	}
	coverResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+coveredIncident.ID+"/cover", map[string]interface{}{"note": "Known recurring outage"}, "")
	if coverResp.Code != http.StatusOK {
		t.Fatalf("cover list incident status = %d, body = %s", coverResp.Code, coverResp.Body.String())
	}
	manualFilterResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?status=open,acknowledged,covered,resolved&resolution_kind=manual", nil, "")
	if manualFilterResp.Code != http.StatusOK {
		t.Fatalf("manual resolution filter status = %d, body = %s", manualFilterResp.Code, manualFilterResp.Body.String())
	}
	var manualFilter struct {
		Data struct {
			Count     int64 `json:"count"`
			Incidents []struct {
				ResolutionKind string `json:"resolution_kind"`
			} `json:"incidents"`
		} `json:"data"`
	}
	decodeResponse(t, manualFilterResp, &manualFilter)
	if manualFilter.Data.Count != 1 || manualFilter.Data.Incidents[0].ResolutionKind != "manual" {
		t.Fatalf("manual resolution filter = %+v, want one manual incident", manualFilter)
	}
	actorFilterResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?status=open,acknowledged,covered,resolved&actor=manual", nil, "")
	if actorFilterResp.Code != http.StatusOK {
		t.Fatalf("manual actor filter status = %d, body = %s", actorFilterResp.Code, actorFilterResp.Body.String())
	}
	var actorFilter struct {
		Data struct {
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, actorFilterResp, &actorFilter)
	if actorFilter.Data.Count != 2 {
		t.Fatalf("manual actor filter count = %d, want 2 acknowledged/covered incidents", actorFilter.Data.Count)
	}
	coveredFilterResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?covered=true", nil, "")
	if coveredFilterResp.Code != http.StatusOK {
		t.Fatalf("covered filter status = %d, body = %s", coveredFilterResp.Code, coveredFilterResp.Body.String())
	}
	var coveredFilter struct {
		Data struct {
			Count     int64 `json:"count"`
			Incidents []struct {
				Status string `json:"status"`
			} `json:"incidents"`
		} `json:"data"`
	}
	decodeResponse(t, coveredFilterResp, &coveredFilter)
	if coveredFilter.Data.Count != 1 || coveredFilter.Data.Incidents[0].Status != "covered" {
		t.Fatalf("covered filter = %+v, want one covered incident", coveredFilter)
	}
	insightsResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?status=open,acknowledged,covered,resolved", nil, "")
	if insightsResp.Code != http.StatusOK {
		t.Fatalf("incident insights status = %d, body = %s", insightsResp.Code, insightsResp.Body.String())
	}
	var insights struct {
		Data struct {
			Insights struct {
				RecurringFailures []struct {
					MonitorID     string `json:"monitor_id"`
					MonitorName   string `json:"monitor_name"`
					IncidentCount int64  `json:"incident_count"`
				} `json:"recurring_failures"`
				LifecycleTiming struct {
					AcknowledgedCount            int64 `json:"acknowledged_count"`
					ResolvedCount                int64 `json:"resolved_count"`
					MeanTimeToAcknowledgeSeconds int64 `json:"mean_time_to_acknowledge_seconds"`
					MeanTimeToResolveSeconds     int64 `json:"mean_time_to_resolve_seconds"`
				} `json:"lifecycle_timing"`
				NotificationReliability struct {
					TotalDeliveries      int64   `json:"total_deliveries"`
					SuppressedDeliveries int64   `json:"suppressed_deliveries"`
					SuccessRatePercent   float64 `json:"success_rate_percent"`
				} `json:"notification_reliability"`
			} `json:"insights"`
		} `json:"data"`
	}
	decodeResponse(t, insightsResp, &insights)
	if len(insights.Data.Insights.RecurringFailures) != 1 || insights.Data.Insights.RecurringFailures[0].MonitorID != registeredMonitor.Data.MonitorID || insights.Data.Insights.RecurringFailures[0].MonitorName != "homepage" || insights.Data.Insights.RecurringFailures[0].IncidentCount != 2 {
		t.Fatalf("recurring failure insights = %+v, want homepage with two incidents", insights.Data.Insights.RecurringFailures)
	}
	if insights.Data.Insights.LifecycleTiming.AcknowledgedCount != 1 || insights.Data.Insights.LifecycleTiming.ResolvedCount != 1 || insights.Data.Insights.LifecycleTiming.MeanTimeToAcknowledgeSeconds < 0 || insights.Data.Insights.LifecycleTiming.MeanTimeToResolveSeconds < 0 {
		t.Fatalf("lifecycle insights = %+v, want acknowledgement and resolution timing", insights.Data.Insights.LifecycleTiming)
	}
	if insights.Data.Insights.NotificationReliability.TotalDeliveries == 0 || insights.Data.Insights.NotificationReliability.SuppressedDeliveries == 0 || insights.Data.Insights.NotificationReliability.SuccessRatePercent != 0 {
		t.Fatalf("notification insights = %+v, want suppressed delivery reliability stats", insights.Data.Insights.NotificationReliability)
	}
}
