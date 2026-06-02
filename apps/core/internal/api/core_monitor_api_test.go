package api

import (
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"testing"
	"time"
)

func TestCoreWorkerReportsOpenAndResolveIncident(t *testing.T) {
	server := setupTestServer(t)
	monitor := db.Monitor{ID: "monitor-core-worker-report", AgentID: "agent-core-worker", Name: "Core API health", Type: "http", Lifecycle: "active", Health: "unknown", ComputedHealth: "unknown", ReportingIntervalSeconds: 30, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create core monitor: %v", err)
	}
	downReportID, err := server.reportService.StoreMonitorReport(monitor.ID, service.MonitorReportPayload{Timestamp: time.Now().UTC().Format(time.RFC3339), Health: "down", Metrics: map[string]interface{}{"runner": "core", "status_code": 500, "duration_ms": 125, "failure_step": "http"}})
	if err != nil {
		t.Fatalf("store core down report: %v", err)
	}
	if downReportID == nil || *downReportID == "" {
		t.Fatalf("down report id = %v, want generated id", downReportID)
	}
	var coreOwner db.Agent
	if err := server.db.Where("id = ?", "agent-core-worker").First(&coreOwner).Error; err != nil {
		t.Fatalf("find generated core owner: %v", err)
	}
	if coreOwner.MachineId != "core" || coreOwner.Name != "Orion Core" {
		t.Fatalf("core owner = %+v, want Orion Core owner row", coreOwner)
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find core incident: %v", err)
	}
	if incident.Status != "open" || incident.Severity != "high" || incident.AgentID != coreOwner.ID {
		t.Fatalf("core incident = %+v, want open high incident owned by core", incident)
	}
	if incident.Title != "Core monitor down: Core API health" {
		t.Fatalf("core incident title = %q, want Core monitor down title", incident.Title)
	}
	assertAlertDelivery(t, server, incident.ID, "incident_opened", "suppressed")
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "down")
	var storedMonitor db.Monitor
	if err := server.db.Where("id = ?", monitor.ID).First(&storedMonitor).Error; err != nil {
		t.Fatalf("reload core monitor: %v", err)
	}
	if storedMonitor.Health != "down" || storedMonitor.ComputedHealth != "down" {
		t.Fatalf("core monitor health = stored %q computed %q, want down/down", storedMonitor.Health, storedMonitor.ComputedHealth)
	}
	listResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?monitor_id="+monitor.ID, nil, "")
	if listResp.Code != http.StatusOK {
		t.Fatalf("list core incidents status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Incidents []struct {
				AgentName   string `json:"agent_name"`
				MonitorName string `json:"monitor_name"`
				Severity    string `json:"severity"`
			} `json:"incidents"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, listResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || listed.Data.Incidents[0].AgentName != "Orion Core" || listed.Data.Incidents[0].MonitorName != "Core API health" || listed.Data.Incidents[0].Severity != "high" {
		t.Fatalf("listed core incidents = %+v, want Orion Core incident", listed)
	}
	upReportID, err := server.reportService.StoreMonitorReport(monitor.ID, service.MonitorReportPayload{Timestamp: time.Now().UTC().Format(time.RFC3339), Health: "up", Metrics: map[string]interface{}{"runner": "core", "status_code": 200, "duration_ms": 48}})
	if err != nil {
		t.Fatalf("store core up report: %v", err)
	}
	if upReportID == nil || *upReportID == "" {
		t.Fatalf("up report id = %v, want generated id", upReportID)
	}
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload core incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil {
		t.Fatalf("core incident = %+v, want resolved with resolved_at", incident)
	}
	assertAlertDelivery(t, server, incident.ID, "incident_resolved", "suppressed")
	assertMonitorIncidentState(t, server, monitor.ID, "", "up")
	if err := server.db.Where("id = ?", monitor.ID).First(&storedMonitor).Error; err != nil {
		t.Fatalf("reload recovered core monitor: %v", err)
	}
	if storedMonitor.Health != "up" || storedMonitor.LastSuccessfulReportAt == nil {
		t.Fatalf("recovered core monitor = %+v, want up with last_successful_report_at", storedMonitor)
	}
}
func TestCoreMonitorConfirmationPeriodDefersTransientFailure(t *testing.T) {
	server := setupTestServer(t)
	monitor := seedCoreConfirmationMonitor(t, server, "monitor-core-confirm-transient", 60, 0)
	startedAt := time.Date(2026, 5, 28, 1, 0, 0, 0, time.UTC)
	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt)
	assertCoreIncidentCount(t, server, monitor.ID, 0)
	assertMonitorIncidentState(t, server, monitor.ID, "", "down")
	storeCoreConfirmationReport(t, server, monitor.ID, "up", startedAt.Add(30*time.Second))
	assertCoreIncidentCount(t, server, monitor.ID, 0)
	assertMonitorIncidentState(t, server, monitor.ID, "", "up")
}
func TestCoreMonitorConfirmationPeriodOpensSustainedFailure(t *testing.T) {
	server := setupTestServer(t)
	monitor := seedCoreConfirmationMonitor(t, server, "monitor-core-confirm-sustained", 60, 0)
	startedAt := time.Date(2026, 5, 28, 1, 10, 0, 0, time.UTC)
	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt)
	assertCoreIncidentCount(t, server, monitor.ID, 0)
	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt.Add(61*time.Second))
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find sustained core incident: %v", err)
	}
	if incident.Status != "open" {
		t.Fatalf("sustained incident = %+v, want open", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "down")
}
func TestCoreMonitorConfirmationCheckCountOpensAfterConsecutiveFailures(t *testing.T) {
	server := setupTestServer(t)
	monitor := seedCoreConfirmationMonitor(t, server, "monitor-core-confirm-count", 0, 2)
	startedAt := time.Date(2026, 5, 28, 1, 20, 0, 0, time.UTC)
	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt)
	assertCoreIncidentCount(t, server, monitor.ID, 0)
	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt.Add(10*time.Second))
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find count-confirmed core incident: %v", err)
	}
	if incident.Status != "open" {
		t.Fatalf("count-confirmed incident = %+v, want open", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "down")
}
func TestCoreMonitorRecoveryPeriodDefersBlipsAndResolvesSustainedRecovery(t *testing.T) {
	server := setupTestServer(t)
	monitor := seedCoreNoiseMonitor(t, server, "monitor-core-recovery-period", 0, 0, 60)
	startedAt := time.Date(2026, 5, 28, 1, 30, 0, 0, time.UTC)
	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt)
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find recovery incident: %v", err)
	}
	if incident.Status != "open" {
		t.Fatalf("recovery incident = %+v, want open", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "down")
	storeCoreConfirmationReport(t, server, monitor.ID, "up", startedAt.Add(30*time.Second))
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload recovering incident: %v", err)
	}
	if incident.Status != "open" || incident.ResolvedAt != nil {
		t.Fatalf("short recovery incident = %+v, want still open", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "recovering")
	assertIncidentEventCount(t, server, incident.ID, "incident_resolved", 0)
	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt.Add(40*time.Second))
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload relapse incident: %v", err)
	}
	if incident.Status != "open" || incident.ResolvedAt != nil {
		t.Fatalf("relapse incident = %+v, want still open", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "down")
	storeCoreConfirmationReport(t, server, monitor.ID, "up", startedAt.Add(50*time.Second))
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "recovering")
	storeCoreConfirmationReport(t, server, monitor.ID, "up", startedAt.Add(111*time.Second))
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload resolved recovery incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil {
		t.Fatalf("sustained recovery incident = %+v, want resolved", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, "", "up")
}
func TestCoreMonitorMaintenanceWindowsSuppressIncidents(t *testing.T) {
	server := setupTestServer(t)
	windowStart := time.Date(2026, 5, 28, 2, 0, 0, 0, time.UTC)
	windowEnd := windowStart.Add(30 * time.Minute)
	monitor := seedCoreMaintenanceMonitor(t, server, "monitor-core-maintenance-window", windowStart, windowEnd)
	storeCoreConfirmationReport(t, server, monitor.ID, "down", windowStart.Add(-time.Minute))
	assertCoreIncidentCount(t, server, monitor.ID, 1)
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find maintenance incident: %v", err)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "down")
	var deliveryCountBeforeMaintenance int64
	if err := server.db.Model(&db.AlertDelivery{}).Joins("JOIN incidents ON incidents.id = alert_deliveries.incident_id").Where("incidents.monitor_id = ?", monitor.ID).Count(&deliveryCountBeforeMaintenance).Error; err != nil {
		t.Fatalf("count pre-maintenance alert deliveries: %v", err)
	}
	storeCoreConfirmationReport(t, server, monitor.ID, "up", windowStart.Add(time.Minute))
	assertCoreIncidentCount(t, server, monitor.ID, 1)
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload maintenance incident: %v", err)
	}
	if incident.Status != "open" || incident.ResolvedAt != nil {
		t.Fatalf("maintenance recovery incident = %+v, want still open", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "maintenance")
	assertIncidentEventCount(t, server, incident.ID, "incident_resolved", 0)
	storeCoreConfirmationReport(t, server, monitor.ID, "down", windowStart.Add(2*time.Minute))
	assertCoreIncidentCount(t, server, monitor.ID, 1)
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "maintenance")
	var activeReportCount int64
	if err := server.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", monitor.ID).Count(&activeReportCount).Error; err != nil {
		t.Fatalf("count maintenance reports: %v", err)
	}
	if activeReportCount != 3 {
		t.Fatalf("maintenance report count = %d, want preserved report", activeReportCount)
	}
	var activeDeliveryCount int64
	if err := server.db.Model(&db.AlertDelivery{}).Joins("JOIN incidents ON incidents.id = alert_deliveries.incident_id").Where("incidents.monitor_id = ?", monitor.ID).Count(&activeDeliveryCount).Error; err != nil {
		t.Fatalf("count maintenance alert deliveries: %v", err)
	}
	if activeDeliveryCount != deliveryCountBeforeMaintenance {
		t.Fatalf("maintenance alert delivery count = %d, want unchanged %d", activeDeliveryCount, deliveryCountBeforeMaintenance)
	}
	storeCoreConfirmationReport(t, server, monitor.ID, "up", windowEnd.Add(time.Second))
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload resolved maintenance incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil {
		t.Fatalf("exited maintenance incident = %+v, want resolved", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, "", "up")
	storeCoreConfirmationReport(t, server, monitor.ID, "down", windowEnd.Add(2*time.Second))
	assertCoreIncidentCount(t, server, monitor.ID, 2)
}
func TestCoreMonitorManagementLifecycle(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{AlertRecoveryNotifications: true, AlertTLSExpiryDays: 14, CoreMonitorAllowPrivateTargets: true})
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ready"))
	}))
	defer target.Close()
	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Core API probe", "description": "managed by core", "kind": "http", "interval_seconds": 30, "timeout_seconds": 2, "confirmation_period_seconds": 60, "confirmation_check_count": 3, "recovery_period_seconds": 45, "config": map[string]interface{}{"url": target.URL + "/health", "expected_status": http.StatusAccepted, "authorization": "Bearer should-not-leak", "required_contains": []string{"ready"}}, "secret_refs": map[string]interface{}{"header": "secret-ref"}}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create core monitor status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	assertNotContains(t, createResp.Body.String(), "should-not-leak")
	assertNotContains(t, createResp.Body.String(), "secret-ref")
	var created struct {
		Success bool `json:"success"`
		Data    struct {
			Monitor struct {
				ID        string `json:"id"`
				OwnerKind string `json:"owner_kind"`
				Source    string `json:"source"`
			} `json:"monitor"`
			Config struct {
				Kind                      string                 `json:"kind"`
				ConfirmationPeriodSeconds int                    `json:"confirmation_period_seconds"`
				ConfirmationCheckCount    int                    `json:"confirmation_check_count"`
				RecoveryPeriodSeconds     int                    `json:"recovery_period_seconds"`
				Config                    map[string]interface{} `json:"config"`
				SecretRefs                map[string]interface{} `json:"secret_refs"`
			} `json:"config"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	if !created.Success || created.Data.Monitor.ID == "" {
		t.Fatalf("create response = %+v, want monitor id", created)
	}
	monitorID := created.Data.Monitor.ID
	if created.Data.Monitor.OwnerKind != "core" || created.Data.Monitor.Source != "core" {
		t.Fatalf("owner fields = %+v, want core/core", created.Data.Monitor)
	}
	if created.Data.Config.Kind != "http" || created.Data.Config.Config["authorization"] != "[redacted]" || created.Data.Config.SecretRefs["header"] != "[redacted]" {
		t.Fatalf("redacted config = %+v, secret refs = %+v", created.Data.Config.Config, created.Data.Config.SecretRefs)
	}
	if created.Data.Config.ConfirmationPeriodSeconds != 60 || created.Data.Config.ConfirmationCheckCount != 3 || created.Data.Config.RecoveryPeriodSeconds != 45 {
		t.Fatalf("noise config = %+v, want 60s confirmation, 3 checks, and 45s recovery", created.Data.Config)
	}
	configResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+monitorID+"/config", nil, "")
	if configResp.Code != http.StatusOK {
		t.Fatalf("get config status = %d, body = %s", configResp.Code, configResp.Body.String())
	}
	assertNotContains(t, configResp.Body.String(), "should-not-leak")
	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+monitorID, map[string]interface{}{"name": "Core API probe updated", "interval_seconds": 45, "confirmation_period_seconds": 30, "confirmation_check_count": 2, "recovery_period_seconds": 20, "config": map[string]interface{}{"url": target.URL + "/health", "expected_status": http.StatusAccepted, "token": "patch-secret"}}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	assertNotContains(t, updateResp.Body.String(), "patch-secret")
	var updated db.Monitor
	if err := server.db.Where("id = ?", monitorID).First(&updated).Error; err != nil {
		t.Fatalf("find updated monitor: %v", err)
	}
	if updated.Name != "Core API probe updated" || updated.ReportingIntervalSeconds != 45 {
		t.Fatalf("updated monitor = %+v, want updated name and interval", updated)
	}
	var updatedConfig db.CoreMonitorConfig
	if err := server.db.Where("monitor_id = ?", monitorID).First(&updatedConfig).Error; err != nil {
		t.Fatalf("find updated core monitor config: %v", err)
	}
	if updatedConfig.ConfirmationPeriodSeconds != 30 || updatedConfig.ConfirmationCheckCount != 2 || updatedConfig.RecoveryPeriodSeconds != 20 {
		t.Fatalf("updated noise config = %+v, want 30s confirmation, 2 checks, and 20s recovery", updatedConfig)
	}
	pauseResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+monitorID+"/pause", nil, "")
	if pauseResp.Code != http.StatusOK {
		t.Fatalf("pause status = %d, body = %s", pauseResp.Code, pauseResp.Body.String())
	}
	var paused db.CoreMonitorConfig
	if err := server.db.Where("monitor_id = ?", monitorID).First(&paused).Error; err != nil {
		t.Fatalf("find paused core monitor config: %v", err)
	}
	if !paused.Paused {
		t.Fatalf("paused = false, want true")
	}
	resumeResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+monitorID+"/resume", nil, "")
	if resumeResp.Code != http.StatusOK {
		t.Fatalf("resume status = %d, body = %s", resumeResp.Code, resumeResp.Body.String())
	}
	var resumed db.CoreMonitorConfig
	if err := server.db.Where("monitor_id = ?", monitorID).First(&resumed).Error; err != nil {
		t.Fatalf("find resumed core monitor config: %v", err)
	}
	if resumed.Paused || resumed.NextRunAt.After(time.Now().UTC().Add(2*time.Second)) {
		t.Fatalf("resumed config = %+v, want unpaused and due", resumed)
	}
	testResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+monitorID+"/test", nil, "")
	if testResp.Code != http.StatusOK {
		t.Fatalf("test-now status = %d, body = %s", testResp.Code, testResp.Body.String())
	}
	var reportCount int64
	if err := server.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", monitorID).Count(&reportCount).Error; err != nil {
		t.Fatalf("count test-now reports: %v", err)
	}
	if reportCount != 1 {
		t.Fatalf("test-now report count = %d, want 1", reportCount)
	}
	var tested db.CoreMonitorConfig
	if err := server.db.Where("monitor_id = ?", monitorID).First(&tested).Error; err != nil {
		t.Fatalf("find tested core monitor config: %v", err)
	}
	if tested.LastRunAt == nil || tested.LastSuccessAt == nil {
		t.Fatalf("tested config = %+v, want last run and success", tested)
	}
	deleteResp := performJSONRequest(t, server, http.MethodDelete, "/v1/monitors/"+monitorID, nil, "")
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	var deleted db.Monitor
	if err := server.db.Where("id = ?", monitorID).First(&deleted).Error; err != nil {
		t.Fatalf("find deleted monitor: %v", err)
	}
	if deleted.Lifecycle != "deleted" {
		t.Fatalf("deleted lifecycle = %q, want deleted", deleted.Lifecycle)
	}
}
func TestCoreMonitorManagementRejectsUnsupportedKind(t *testing.T) {
	server := setupTestServer(t)
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Unsupported", "kind": "coffee", "config": map[string]interface{}{}}, "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("unsupported kind status = %d, body = %s", resp.Code, resp.Body.String())
	}
}
