package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"orion/core/internal/db"
)

func TestMaintenanceSuppressesIncidentCandidates(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)

	reportBody := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{},
		"error": map[string]string{
			"message": "connection refused",
		},
	}
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	reportResp := performJSONRequest(t, server, http.MethodPost, reportPath, reportBody, registered.Data.Token)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("monitor report status = %d, body = %s", reportResp.Code, reportResp.Body.String())
	}

	beforeMaintenance := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, "")
	assertIncidentCandidateCount(t, beforeMaintenance, 1)

	maintenanceResp := performJSONRequest(
		t,
		server,
		http.MethodPut,
		"/v1/agents/"+registered.Data.AgentID+"/maintenance",
		map[string]bool{"maintenance_mode": true},
		registered.Data.Token,
	)
	if maintenanceResp.Code != http.StatusOK {
		t.Fatalf("maintenance status = %d, body = %s", maintenanceResp.Code, maintenanceResp.Body.String())
	}

	afterMaintenance := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, "")
	assertIncidentCandidateCount(t, afterMaintenance, 0)

	disableMaintenanceResp := performJSONRequest(
		t,
		server,
		http.MethodPut,
		"/v1/agents/"+registered.Data.AgentID+"/maintenance",
		map[string]bool{"maintenance_mode": false},
		registered.Data.Token,
	)
	if disableMaintenanceResp.Code != http.StatusOK {
		t.Fatalf("disable maintenance status = %d, body = %s", disableMaintenanceResp.Code, disableMaintenanceResp.Body.String())
	}

	afterMaintenanceDisabled := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, "")
	assertIncidentCandidateCount(t, afterMaintenanceDisabled, 1)
}

func TestIncidentCandidatesIncludeStaleMonitors(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{
		ID:        "agent-stale-candidate",
		MachineId: "machine-stale-candidate",
		Name:      "stale candidate",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-stale-candidate",
		LastSeen:  time.Now(),
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitor := db.Monitor{
		ID:                       "monitor-stale-candidate",
		AgentID:                  agent.ID,
		Name:                     "stale monitor",
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "up",
		ComputedHealth:           "up",
		ReportingIntervalSeconds: 60,
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}

	oldTimestamp := time.Now().Add(-30 * time.Minute)
	report := db.MonitorReport{
		ID:          "monitor-report-stale-candidate",
		MonitorID:   monitor.ID,
		Payload:     "{}",
		CollectedAt: oldTimestamp.UTC().Format(time.RFC3339),
		Health:      "up",
		CreatedAt:   oldTimestamp,
	}
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

	downResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics": map[string]interface{}{
			"failure_reason": "connection refused",
		},
	}, registered.Data.Token)
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

	downAgainResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{},
	}, registered.Data.Token)
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

	upResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "up",
		"metrics": map[string]interface{}{
			"status_code": 200,
		},
	}, registered.Data.Token)
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

	downResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics": map[string]interface{}{
			"failure_reason": "manual action test failure",
		},
	}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	incidentID := incident.ID

	coveredUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	coverResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incidentID+"/cover", map[string]interface{}{
		"covered_until": coveredUntil.Format(time.RFC3339),
		"note":          "Known deploy window",
	}, "")
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
	assertIncidentEventMetadata(t, server, incident.ID, "incident_covered", "user", "console", "Known deploy window")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")

	reopenCoveredResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/reopen", map[string]interface{}{
		"note": "Deploy window complete",
	}, "")
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
	assertIncidentEventMetadata(t, server, incident.ID, "incident_reopened", "user", "console", "Deploy window complete")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")

	ackResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/acknowledge", map[string]interface{}{
		"note": "Investigating with on-call",
	}, "")
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
	assertIncidentEventMetadata(t, server, incident.ID, "incident_acknowledged", "user", "console", "Investigating with on-call")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")

	resolveResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/resolve", map[string]interface{}{
		"note": "Rollback restored service",
	}, "")
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
	assertIncidentEventMetadata(t, server, incident.ID, "incident_resolved", "user", "console", "Rollback restored service")
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
	coverResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/cover", map[string]interface{}{
		"covered_until": coveredUntil.Format(time.RFC3339),
		"note":          "Known upstream outage",
	}, "")
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
	expiredCoverResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+expiringIncident.ID+"/cover", map[string]interface{}{
		"covered_until": expiredUntil.Format(time.RFC3339),
		"note":          "Expired maintenance cover",
	}, "")
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

	downResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/"+registeredMonitor.Data.MonitorID+"/report", map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics": map[string]interface{}{
			"failure_reason": "removed monitor failure",
		},
	}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")

	unregisterResp := performJSONRequest(
		t,
		server,
		http.MethodPost,
		"/v1/agents/"+registered.Data.AgentID+"/unregister-monitor",
		map[string]interface{}{"monitor_id": registeredMonitor.Data.MonitorID},
		registered.Data.Token,
	)
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

func TestUnregisterMonitorClearsStaleIncidentPathWithoutDuplicateEvent(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	now := time.Now().UTC()
	incident := db.Incident{
		ID:                 "incident-stale-active-path",
		Status:             "resolved",
		Severity:           "high",
		Title:              "Resolved stale active path",
		AgentID:            registered.Data.AgentID,
		MonitorID:          registeredMonitor.Data.MonitorID,
		OpenedAt:           now.Add(-30 * time.Minute),
		ResolvedAt:         &now,
		LastEventAt:        now,
		LatestEvent:        "Already resolved",
		NotificationStatus: "suppressed",
	}
	if err := server.db.Create(&incident).Error; err != nil {
		t.Fatalf("create resolved incident: %v", err)
	}
	if err := server.db.Model(&db.Monitor{}).Where("id = ?", registeredMonitor.Data.MonitorID).Updates(map[string]interface{}{
		"active_incident_id": incident.ID,
		"incident_state":     "down",
	}).Error; err != nil {
		t.Fatalf("set stale monitor incident path: %v", err)
	}

	unregisterResp := performJSONRequest(
		t,
		server,
		http.MethodPost,
		"/v1/agents/"+registered.Data.AgentID+"/unregister-monitor",
		map[string]interface{}{"monitor_id": registeredMonitor.Data.MonitorID},
		registered.Data.Token,
	)
	if unregisterResp.Code != http.StatusOK {
		t.Fatalf("unregister monitor status = %d, body = %s", unregisterResp.Code, unregisterResp.Body.String())
	}

	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, "", "unknown")

	var resolvedEventCount int64
	if err := server.db.Model(&db.IncidentEvent{}).
		Where("incident_id = ? AND type = ?", incident.ID, "incident_resolved").
		Count(&resolvedEventCount).Error; err != nil {
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
	reportResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{"status_code": 500},
	}, registered.Data.Token)
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
	if listed.Data.Incidents[0].Status != "open" ||
		listed.Data.Incidents[0].AgentName != "test-server" ||
		listed.Data.Incidents[0].MonitorName != "homepage" {
		t.Fatalf("incident row = %+v, want open homepage on test-server", listed.Data.Incidents[0])
	}
	if len(listed.Data.Incidents[0].ImpactedComponents) != 1 ||
		listed.Data.Incidents[0].ImpactedComponents[0].ComponentID != "incident-list-component" ||
		listed.Data.Incidents[0].ImpactedComponents[0].ComponentName != "Public API" ||
		listed.Data.Incidents[0].ImpactedComponents[0].Status != "major_outage" ||
		listed.Data.Incidents[0].ImpactedComponents[0].Impact != "down" {
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

	if err := server.db.Model(&db.Incident{}).
		Where("monitor_id = ?", registeredMonitor.Data.MonitorID).
		Update("notification_status", "failed").Error; err != nil {
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

	secondReportResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{"status_code": 503},
	}, registered.Data.Token)
	if secondReportResp.Code != http.StatusOK {
		t.Fatalf("second monitor report status = %d, body = %s", secondReportResp.Code, secondReportResp.Body.String())
	}
	var coveredIncident db.Incident
	if err := server.db.Where("monitor_id = ? AND status = ?", registeredMonitor.Data.MonitorID, "open").First(&coveredIncident).Error; err != nil {
		t.Fatalf("find second list incident: %v", err)
	}
	coverResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+coveredIncident.ID+"/cover", map[string]interface{}{
		"note": "Known recurring outage",
	}, "")
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
	if len(insights.Data.Insights.RecurringFailures) != 1 ||
		insights.Data.Insights.RecurringFailures[0].MonitorID != registeredMonitor.Data.MonitorID ||
		insights.Data.Insights.RecurringFailures[0].MonitorName != "homepage" ||
		insights.Data.Insights.RecurringFailures[0].IncidentCount != 2 {
		t.Fatalf("recurring failure insights = %+v, want homepage with two incidents", insights.Data.Insights.RecurringFailures)
	}
	if insights.Data.Insights.LifecycleTiming.AcknowledgedCount != 1 ||
		insights.Data.Insights.LifecycleTiming.ResolvedCount != 1 ||
		insights.Data.Insights.LifecycleTiming.MeanTimeToAcknowledgeSeconds < 0 ||
		insights.Data.Insights.LifecycleTiming.MeanTimeToResolveSeconds < 0 {
		t.Fatalf("lifecycle insights = %+v, want acknowledgement and resolution timing", insights.Data.Insights.LifecycleTiming)
	}
	if insights.Data.Insights.NotificationReliability.TotalDeliveries == 0 ||
		insights.Data.Insights.NotificationReliability.SuppressedDeliveries == 0 ||
		insights.Data.Insights.NotificationReliability.SuccessRatePercent != 0 {
		t.Fatalf("notification insights = %+v, want suppressed delivery reliability stats", insights.Data.Insights.NotificationReliability)
	}
}

func TestIncidentDetailAndTimelineEndpoints(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	createIncidentImpactStatusPageMapping(t, server, registeredMonitor.Data.MonitorID, registered.Data.AgentID, "incident-detail-component", "Checkout", "agent")

	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	downResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{"status_code": 500},
	}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	relatedIncident := db.Incident{
		ID:                 "incident-related-detail",
		Status:             "resolved",
		Severity:           "medium",
		Title:              "Prior homepage failure",
		AgentID:            registered.Data.AgentID,
		MonitorID:          registeredMonitor.Data.MonitorID,
		OpenedAt:           incident.OpenedAt.Add(-2 * time.Hour),
		ResolvedAt:         &incident.OpenedAt,
		LastEventAt:        incident.OpenedAt,
		LatestEvent:        "Prior incident resolved",
		NotificationStatus: "suppressed",
		ResolutionKind:     "recovered",
	}
	if err := server.db.Create(&relatedIncident).Error; err != nil {
		t.Fatalf("create related incident: %v", err)
	}

	latestResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "degraded",
		"metrics":   map[string]interface{}{"status_code": 503, "message": "slow checkout"},
	}, registered.Data.Token)
	if latestResp.Code != http.StatusOK {
		t.Fatalf("latest report status = %d, body = %s", latestResp.Code, latestResp.Body.String())
	}

	ackResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/acknowledge", map[string]interface{}{
		"note": "On-call is investigating",
	}, "")
	if ackResp.Code != http.StatusOK {
		t.Fatalf("acknowledge incident status = %d, body = %s", ackResp.Code, ackResp.Body.String())
	}

	detailResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID, nil, "")
	if detailResp.Code != http.StatusOK {
		t.Fatalf("incident detail status = %d, body = %s", detailResp.Code, detailResp.Body.String())
	}
	var detail struct {
		Success bool `json:"success"`
		Data    struct {
			Incident struct {
				ID             string `json:"id"`
				AgentName      string `json:"agent_name"`
				MonitorName    string `json:"monitor_name"`
				AllowedActions struct {
					Acknowledge struct {
						Allowed bool   `json:"allowed"`
						Reason  string `json:"reason"`
					} `json:"acknowledge"`
					Cover struct {
						Allowed bool `json:"allowed"`
					} `json:"cover"`
					Resolve struct {
						Allowed bool `json:"allowed"`
					} `json:"resolve"`
					Reopen struct {
						Allowed bool   `json:"allowed"`
						Reason  string `json:"reason"`
					} `json:"reopen"`
				} `json:"allowed_actions"`
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
			Timeline []struct {
				Type            string `json:"type"`
				Source          string `json:"source"`
				Evidence        string `json:"evidence"`
				MonitorReportID string `json:"monitor_report_id"`
				AlertDeliveryID string `json:"alert_delivery_id"`
				ActorType       string `json:"actor_type"`
				ActorID         string `json:"actor_id"`
				Note            string `json:"note"`
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
	if detail.Data.Incident.AllowedActions.Acknowledge.Allowed ||
		detail.Data.Incident.AllowedActions.Acknowledge.Reason == "" ||
		!detail.Data.Incident.AllowedActions.Cover.Allowed ||
		!detail.Data.Incident.AllowedActions.Resolve.Allowed ||
		detail.Data.Incident.AllowedActions.Reopen.Allowed ||
		detail.Data.Incident.AllowedActions.Reopen.Reason == "" {
		t.Fatalf("allowed actions = %+v, want acknowledged incident action state", detail.Data.Incident.AllowedActions)
	}
	if len(detail.Data.Incident.ImpactedComponents) != 1 ||
		detail.Data.Incident.ImpactedComponents[0].ComponentID != "incident-detail-component" ||
		detail.Data.Incident.ImpactedComponents[0].ComponentName != "Checkout" ||
		detail.Data.Incident.ImpactedComponents[0].Status != "degraded" ||
		detail.Data.Incident.ImpactedComponents[0].Impact != "degraded" {
		t.Fatalf("incident detail component impact = %+v, want Checkout degraded impact", detail.Data.Incident.ImpactedComponents)
	}
	if len(detail.Data.Timeline) < 2 || len(detail.Data.AlertDeliveries) == 0 || len(detail.Data.MonitorReports) == 0 {
		t.Fatalf("incident detail linked data missing: %+v", detail.Data)
	}
	if detail.Data.Evidence.TriggeringReport == nil ||
		detail.Data.Evidence.TriggeringReport.Health != "down" ||
		!strings.Contains(detail.Data.Evidence.TriggeringReport.Payload, `"status_code":500`) {
		t.Fatalf("triggering evidence = %+v, want down report with safe payload", detail.Data.Evidence.TriggeringReport)
	}
	if detail.Data.Evidence.LatestReport == nil ||
		detail.Data.Evidence.LatestReport.Health != "degraded" ||
		!strings.Contains(detail.Data.Evidence.LatestReport.Payload, "slow checkout") {
		t.Fatalf("latest evidence = %+v, want latest degraded report", detail.Data.Evidence.LatestReport)
	}
	if len(detail.Data.RelatedIncidents) != 1 ||
		detail.Data.RelatedIncidents[0].ID != relatedIncident.ID ||
		detail.Data.RelatedIncidents[0].ResolutionKind != "recovered" {
		t.Fatalf("related incidents = %+v, want prior same-monitor incident", detail.Data.RelatedIncidents)
	}
	var reportLinked bool
	var deliveryLinked bool
	var acknowledgedWithNote bool
	for _, item := range detail.Data.Timeline {
		if item.MonitorReportID != "" && item.Evidence != "" {
			reportLinked = true
		}
		if item.AlertDeliveryID != "" {
			deliveryLinked = true
		}
		if item.Type == "incident_acknowledged" &&
			item.ActorType == "user" &&
			item.ActorID == "console" &&
			item.Note == "On-call is investigating" {
			acknowledgedWithNote = true
		}
	}
	if !reportLinked || !deliveryLinked {
		t.Fatalf("timeline links = %+v, want report evidence and alert delivery links", detail.Data.Timeline)
	}
	if !acknowledgedWithNote {
		t.Fatalf("timeline = %+v, want acknowledged action actor and note", detail.Data.Timeline)
	}

	timelineResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID+"/timeline", nil, "")
	if timelineResp.Code != http.StatusOK {
		t.Fatalf("incident timeline status = %d, body = %s", timelineResp.Code, timelineResp.Body.String())
	}
	var timeline struct {
		Success bool `json:"success"`
		Data    struct {
			Timeline []struct {
				Source    string `json:"source"`
				ActorType string `json:"actor_type"`
				ActorID   string `json:"actor_id"`
				Note      string `json:"note"`
			} `json:"timeline"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, timelineResp, &timeline)
	if !timeline.Success || timeline.Data.Count < 2 {
		t.Fatalf("timeline response = %+v, want incident and alert events", timeline)
	}
	acknowledgedWithNote = false
	for _, item := range timeline.Data.Timeline {
		if item.ActorType == "user" && item.ActorID == "console" && item.Note == "On-call is investigating" {
			acknowledgedWithNote = true
		}
	}
	if !acknowledgedWithNote {
		t.Fatalf("timeline response = %+v, want action actor and note", timeline.Data.Timeline)
	}
}

func createIncidentImpactStatusPageMapping(t *testing.T, server *Server, monitorID string, agentID string, componentID string, componentName string, resourceType string) {
	t.Helper()

	now := time.Now().UTC()
	page := db.StatusPage{
		ID:                        componentID + "-page",
		Slug:                      componentID + "-page",
		Title:                     componentName + " Status",
		Visibility:                statusPageVisibilityPublic,
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
		PublishedAt:               &now,
	}
	section := db.StatusPageSection{
		ID:           componentID + "-section",
		StatusPageID: page.ID,
		Name:         "Public services",
	}
	component := db.StatusPageComponent{
		ID:           componentID,
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   componentName,
		DisplayMode:  "single_resource",
		SortOrder:    1,
		Visible:      true,
	}
	resourceID := monitorID
	if resourceType == "agent" {
		resourceID = agentID
	}
	mapping := db.StatusPageComponentMapping{
		ID:                   componentID + "-mapping",
		ComponentID:          component.ID,
		ResourceType:         resourceType,
		ResourceID:           resourceID,
		HealthRollupStrategy: "worst",
		UptimeRollupStrategy: "worst",
	}
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

	maintenanceResp := performJSONRequest(
		t,
		server,
		http.MethodPut,
		"/v1/agents/"+registered.Data.AgentID+"/maintenance",
		map[string]bool{"maintenance_mode": true},
		registered.Data.Token,
	)
	if maintenanceResp.Code != http.StatusOK {
		t.Fatalf("maintenance status = %d, body = %s", maintenanceResp.Code, maintenanceResp.Body.String())
	}

	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	downResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{},
	}, registered.Data.Token)
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

	expiringResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "up",
		"metrics": map[string]interface{}{
			"tls_days_remaining": 3,
		},
	}, registered.Data.Token)
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

	healthyResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "up",
		"metrics": map[string]interface{}{
			"tls_days_remaining": 90,
		},
	}, registered.Data.Token)
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

	if err := server.db.Model(&db.Monitor{}).
		Where("id = ?", registeredMonitor.Data.MonitorID).
		Update("created_at", time.Now().Add(-20*time.Minute)).Error; err != nil {
		t.Fatalf("age monitor: %v", err)
	}

	reportBody := map[string]interface{}{
		"uptime_seconds": 120,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"cpu":            map[string]interface{}{},
		"memory":         map[string]interface{}{},
		"disk":           map[string]interface{}{},
	}
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
