package api

import (
	"net/http"
	"orion/core/internal/db"
	"testing"
	"time"
)

func TestMaintenanceSuppressesIncidentCandidates(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	reportBody := map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "down", "metrics": map[string]interface{}{}, "error": map[string]string{"message": "connection refused"}}
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	reportResp := performJSONRequest(t, server, http.MethodPost, reportPath, reportBody, registered.Data.Token)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("monitor report status = %d, body = %s", reportResp.Code, reportResp.Body.String())
	}
	beforeMaintenance := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, "")
	assertIncidentCandidateCount(t, beforeMaintenance, 1)
	maintenanceResp := performJSONRequest(t, server, http.MethodPut, "/v1/agents/"+registered.Data.AgentID+"/maintenance", map[string]bool{"maintenance_mode": true}, registered.Data.Token)
	if maintenanceResp.Code != http.StatusOK {
		t.Fatalf("maintenance status = %d, body = %s", maintenanceResp.Code, maintenanceResp.Body.String())
	}
	afterMaintenance := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, "")
	assertIncidentCandidateCount(t, afterMaintenance, 0)
	disableMaintenanceResp := performJSONRequest(t, server, http.MethodPut, "/v1/agents/"+registered.Data.AgentID+"/maintenance", map[string]bool{"maintenance_mode": false}, registered.Data.Token)
	if disableMaintenanceResp.Code != http.StatusOK {
		t.Fatalf("disable maintenance status = %d, body = %s", disableMaintenanceResp.Code, disableMaintenanceResp.Body.String())
	}
	afterMaintenanceDisabled := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, "")
	assertIncidentCandidateCount(t, afterMaintenanceDisabled, 1)
}
func TestIncidentCandidatesIncludeStaleMonitors(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{ID: "agent-stale-candidate", MachineId: "machine-stale-candidate", Name: "stale candidate", OS: "linux", Arch: "arm64", Token: "token-stale-candidate", LastSeen: time.Now()}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	monitor := db.Monitor{ID: "monitor-stale-candidate", AgentID: agent.ID, Name: "stale monitor", Type: "http", Lifecycle: "active", Health: "up", ComputedHealth: "up", ReportingIntervalSeconds: 60}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	oldTimestamp := time.Now().Add(-30 * time.Minute)
	report := db.MonitorReport{ID: "monitor-report-stale-candidate", MonitorID: monitor.ID, Payload: "{}", CollectedAt: oldTimestamp.UTC().Format(time.RFC3339), Health: "up", CreatedAt: oldTimestamp}
	if err := server.db.Create(&report).Error; err != nil {
		t.Fatalf("create monitor report: %v", err)
	}
	candidatesResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, "")
	if candidatesResp.Code != http.StatusOK {
		t.Fatalf("incident candidates status = %d, body = %s", candidatesResp.Code, candidatesResp.Body.String())
	}
	var candidates struct {
		Success bool `json:"success"`
		Data    struct {
			Candidates []struct {
				MonitorID string `json:"monitor_id"`
				Health    string `json:"health"`
				Severity  string `json:"severity"`
				IssueType string `json:"issue_type"`
			} `json:"candidates"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, candidatesResp, &candidates)
	if !candidates.Success || candidates.Data.Count != 1 || len(candidates.Data.Candidates) != 1 {
		t.Fatalf("incident candidates = %+v, want one stale candidate", candidates)
	}
	candidate := candidates.Data.Candidates[0]
	if candidate.MonitorID != monitor.ID || candidate.Health != "stale" || candidate.Severity != "high" || candidate.IssueType != "stale_data" {
		t.Fatalf("stale candidate = %+v, want monitor stale high stale_data", candidate)
	}
}
func TestMonitorReportsOpenAndResolveIncident(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	downResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "down", "metrics": map[string]interface{}{"failure_reason": "connection refused"}}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	if incident.Status != "open" || incident.Severity != "high" || incident.NotificationStatus != "pending" {
		t.Fatalf("incident = %+v, want open high pending", incident)
	}
	assertAlertDelivery(t, server, incident.ID, "incident_opened", "suppressed")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")
	downAgainResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "down", "metrics": map[string]interface{}{}}, registered.Data.Token)
	if downAgainResp.Code != http.StatusOK {
		t.Fatalf("second down report status = %d, body = %s", downAgainResp.Code, downAgainResp.Body.String())
	}
	var incidentCount int64
	if err := server.db.Model(&db.Incident{}).Where("monitor_id = ?", registeredMonitor.Data.MonitorID).Count(&incidentCount).Error; err != nil {
		t.Fatalf("count incidents: %v", err)
	}
	if incidentCount != 1 {
		t.Fatalf("incident count = %d, want 1", incidentCount)
	}
	upResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "up", "metrics": map[string]interface{}{"status_code": 200}}, registered.Data.Token)
	if upResp.Code != http.StatusOK {
		t.Fatalf("up report status = %d, body = %s", upResp.Code, upResp.Body.String())
	}
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil {
		t.Fatalf("incident = %+v, want resolved with resolved_at", incident)
	}
	assertAlertDelivery(t, server, incident.ID, "incident_resolved", "suppressed")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, "", "up")
	var eventCount int64
	if err := server.db.Model(&db.IncidentEvent{}).Where("incident_id = ?", incident.ID).Count(&eventCount).Error; err != nil {
		t.Fatalf("count incident events: %v", err)
	}
	if eventCount != 3 {
		t.Fatalf("incident event count = %d, want 3", eventCount)
	}
}
func TestManualIncidentActionsAcknowledgeAndResolve(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	downResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "down", "metrics": map[string]interface{}{"failure_reason": "manual action test failure"}}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	incidentID := incident.ID
	coveredUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	coverResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incidentID+"/cover", map[string]interface{}{"covered_until": coveredUntil.Format(time.RFC3339), "note": "Known deploy window"}, "")
	if coverResp.Code != http.StatusOK {
		t.Fatalf("cover incident status = %d, body = %s", coverResp.Code, coverResp.Body.String())
	}
	incident = db.Incident{}
	if err := server.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		t.Fatalf("reload covered incident: %v", err)
	}
	if incident.Status != "covered" || incident.CoveredAt == nil || incident.CoveredUntil == nil || incident.CoverageNote != "Known deploy window" || incident.LatestEvent != "Incident marked covered" {
		t.Fatalf("covered incident = %+v, want covered lifecycle fields", incident)
	}
	assertIncidentEvent(t, server, incident.ID, "incident_covered", "Incident marked covered")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")
	reopenCoveredResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/reopen", nil, "")
	if reopenCoveredResp.Code != http.StatusOK {
		t.Fatalf("reopen covered incident status = %d, body = %s", reopenCoveredResp.Code, reopenCoveredResp.Body.String())
	}
	incident = db.Incident{}
	if err := server.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		t.Fatalf("reload reopened covered incident: %v", err)
	}
	if incident.Status != "open" || incident.CoveredAt != nil || incident.CoveredUntil != nil || incident.CoverageNote != "" || incident.ReopenedAt == nil || incident.ReopenCount != 1 || incident.LatestEvent != "Incident reopened" {
		t.Fatalf("reopened covered incident = %+v, want open with cleared coverage fields", incident)
	}
	assertIncidentEvent(t, server, incident.ID, "incident_reopened", "Incident reopened")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")
	ackResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/acknowledge", nil, "")
	if ackResp.Code != http.StatusOK {
		t.Fatalf("acknowledge incident status = %d, body = %s", ackResp.Code, ackResp.Body.String())
	}
	incident = db.Incident{}
	if err := server.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		t.Fatalf("reload acknowledged incident: %v", err)
	}
	if incident.Status != "acknowledged" || incident.LatestEvent != "Incident manually acknowledged" {
		t.Fatalf("acknowledged incident = %+v, want acknowledged manual latest event", incident)
	}
	assertIncidentEvent(t, server, incident.ID, "incident_acknowledged", "Incident manually acknowledged")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")
	resolveResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/resolve", nil, "")
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("resolve incident status = %d, body = %s", resolveResp.Code, resolveResp.Body.String())
	}
	incident = db.Incident{}
	if err := server.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		t.Fatalf("reload resolved incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil || incident.ResolutionKind != "manual" || incident.LatestEvent != "Incident manually resolved" {
		t.Fatalf("resolved incident = %+v, want resolved manual latest event", incident)
	}
	assertIncidentEvent(t, server, incident.ID, "incident_resolved", "Incident manually resolved")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, "", "up")
	reopenResolvedResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/reopen", nil, "")
	if reopenResolvedResp.Code != http.StatusOK {
		t.Fatalf("reopen resolved incident status = %d, body = %s", reopenResolvedResp.Code, reopenResolvedResp.Body.String())
	}
	incident = db.Incident{}
	if err := server.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		t.Fatalf("reload reopened resolved incident: %v", err)
	}
	if incident.Status != "open" || incident.ResolvedAt != nil || incident.ResolutionKind != "" || incident.ReopenCount != 2 {
		t.Fatalf("reopened resolved incident = %+v, want open with cleared resolution fields", incident)
	}
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")
	resolveAgainResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/resolve", nil, "")
	if resolveAgainResp.Code != http.StatusOK {
		t.Fatalf("resolve reopened incident status = %d, body = %s", resolveAgainResp.Code, resolveAgainResp.Body.String())
	}
	ackResolvedResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/acknowledge", nil, "")
	if ackResolvedResp.Code != http.StatusBadRequest {
		t.Fatalf("acknowledge resolved incident status = %d, want 400, body = %s", ackResolvedResp.Code, ackResolvedResp.Body.String())
	}
	coverResolvedResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/cover", map[string]interface{}{}, "")
	if coverResolvedResp.Code != http.StatusBadRequest {
		t.Fatalf("cover resolved incident status = %d, want 400, body = %s", coverResolvedResp.Code, coverResolvedResp.Body.String())
	}
}
func TestCoveredIncidentSuppressesFailuresUntilRecoveryOrExpiry(t *testing.T) {
	server := setupTestServer(t)
	startedAt := time.Date(2026, 5, 28, 3, 0, 0, 0, time.UTC)
	coveredMonitor := seedCoreNoiseMonitor(t, server, "monitor-core-covered-suppression", 0, 0, 0)
	storeCoreConfirmationReport(t, server, coveredMonitor.ID, "down", startedAt)
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", coveredMonitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find covered suppression incident: %v", err)
	}
	coveredUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	coverResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/cover", map[string]interface{}{"covered_until": coveredUntil.Format(time.RFC3339), "note": "Known upstream outage"}, "")
	if coverResp.Code != http.StatusOK {
		t.Fatalf("cover incident status = %d, body = %s", coverResp.Code, coverResp.Body.String())
	}
	storeCoreConfirmationReport(t, server, coveredMonitor.ID, "down", startedAt.Add(time.Minute))
	assertCoreIncidentCount(t, server, coveredMonitor.ID, 1)
	incident = db.Incident{}
	if err := server.db.Where("monitor_id = ?", coveredMonitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload suppressed incident: %v", err)
	}
	if incident.Status != "covered" || incident.CoveredAt == nil || incident.CoveredUntil == nil || incident.CoverageNote != "Known upstream outage" || incident.LatestEvent != "Incident coverage suppressed failing monitor report" {
		t.Fatalf("suppressed incident = %+v, want covered with suppression event", incident)
	}
	assertMonitorIncidentState(t, server, coveredMonitor.ID, incident.ID, "down")
	assertIncidentEventCount(t, server, incident.ID, "incident_coverage_suppressed", 1)
	assertIncidentEventCount(t, server, incident.ID, "monitor_failed", 0)
	storeCoreConfirmationReport(t, server, coveredMonitor.ID, "up", startedAt.Add(2*time.Minute))
	incident = db.Incident{}
	if err := server.db.Where("monitor_id = ?", coveredMonitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload recovered covered incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil || incident.CoveredAt != nil || incident.CoveredUntil != nil || incident.CoverageNote != "" || incident.ResolutionKind != "recovered" {
		t.Fatalf("recovered covered incident = %+v, want resolved with cleared coverage", incident)
	}
	assertMonitorIncidentState(t, server, coveredMonitor.ID, "", "up")
	assertIncidentEvent(t, server, incident.ID, "incident_resolved", "Monitor "+coveredMonitor.Name+" recovered")
	expiringMonitor := seedCoreNoiseMonitor(t, server, "monitor-core-covered-expiry", 0, 0, 0)
	storeCoreConfirmationReport(t, server, expiringMonitor.ID, "down", startedAt)
	var expiringIncident db.Incident
	if err := server.db.Where("monitor_id = ?", expiringMonitor.ID).First(&expiringIncident).Error; err != nil {
		t.Fatalf("find expiring coverage incident: %v", err)
	}
	expiredUntil := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	expiredCoverResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+expiringIncident.ID+"/cover", map[string]interface{}{"covered_until": expiredUntil.Format(time.RFC3339), "note": "Expired maintenance cover"}, "")
	if expiredCoverResp.Code != http.StatusOK {
		t.Fatalf("cover expiring incident status = %d, body = %s", expiredCoverResp.Code, expiredCoverResp.Body.String())
	}
	storeCoreConfirmationReport(t, server, expiringMonitor.ID, "down", startedAt.Add(time.Minute))
	assertCoreIncidentCount(t, server, expiringMonitor.ID, 1)
	expiringIncident = db.Incident{}
	if err := server.db.Where("monitor_id = ?", expiringMonitor.ID).First(&expiringIncident).Error; err != nil {
		t.Fatalf("reload expired coverage incident: %v", err)
	}
	if expiringIncident.Status != "open" || expiringIncident.CoveredAt != nil || expiringIncident.CoveredUntil != nil || expiringIncident.CoverageNote != "" || expiringIncident.LatestEvent == "Incident coverage expired" {
		t.Fatalf("expired coverage incident = %+v, want reopened failure with cleared coverage", expiringIncident)
	}
	assertMonitorIncidentState(t, server, expiringMonitor.ID, expiringIncident.ID, "down")
	assertIncidentEventCount(t, server, expiringIncident.ID, "incident_coverage_expired", 1)
	assertIncidentEventCount(t, server, expiringIncident.ID, "monitor_failed", 1)
}
func TestUnregisterMonitorResolvesActiveIncidentPath(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	downResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/"+registeredMonitor.Data.MonitorID+"/report", map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "down", "metrics": map[string]interface{}{"failure_reason": "removed monitor failure"}}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")
	unregisterResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/unregister-monitor", map[string]interface{}{"monitor_id": registeredMonitor.Data.MonitorID}, registered.Data.Token)
	if unregisterResp.Code != http.StatusOK {
		t.Fatalf("unregister monitor status = %d, body = %s", unregisterResp.Code, unregisterResp.Body.String())
	}
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil || incident.LatestEvent != "Monitor removed; active incident resolved" {
		t.Fatalf("incident after monitor unregister = %+v, want resolved monitor removed event", incident)
	}
	assertIncidentEvent(t, server, incident.ID, "incident_resolved", "Monitor removed; active incident resolved")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, "", "unknown")
}
