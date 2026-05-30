package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/service"
)

func TestCoreWorkerReportsOpenAndResolveIncident(t *testing.T) {
	server := setupTestServer(t)
	monitor := db.Monitor{
		ID:                       "monitor-core-worker-report",
		AgentID:                  "agent-core-worker",
		Name:                     "Core API health",
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		ReportingIntervalSeconds: 30,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create core monitor: %v", err)
	}

	downReportID, err := server.reportService.StoreMonitorReport(monitor.ID, service.MonitorReportPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Health:    "down",
		Metrics: map[string]interface{}{
			"runner":       "core",
			"status_code":  500,
			"duration_ms":  125,
			"failure_step": "http",
		},
	})
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

	upReportID, err := server.reportService.StoreMonitorReport(monitor.ID, service.MonitorReportPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Health:    "up",
		Metrics: map[string]interface{}{
			"runner":      "core",
			"status_code": 200,
			"duration_ms": 48,
		},
	})
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
	server := setupTestServerWithConfig(t, &config.Config{
		AlertRecoveryNotifications:     true,
		AlertTLSExpiryDays:             14,
		CoreMonitorAllowPrivateTargets: true,
	})
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ready"))
	}))
	defer target.Close()

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name":                        "Core API probe",
		"description":                 "managed by core",
		"kind":                        "http",
		"interval_seconds":            30,
		"timeout_seconds":             2,
		"confirmation_period_seconds": 60,
		"confirmation_check_count":    3,
		"recovery_period_seconds":     45,
		"config": map[string]interface{}{
			"url":               target.URL + "/health",
			"expected_status":   http.StatusAccepted,
			"authorization":     "Bearer should-not-leak",
			"required_contains": []string{"ready"},
		},
		"secret_refs": map[string]interface{}{
			"header": "secret-ref",
		},
	}, "")
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

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+monitorID, map[string]interface{}{
		"name":                        "Core API probe updated",
		"interval_seconds":            45,
		"confirmation_period_seconds": 30,
		"confirmation_check_count":    2,
		"recovery_period_seconds":     20,
		"config": map[string]interface{}{
			"url":             target.URL + "/health",
			"expected_status": http.StatusAccepted,
			"token":           "patch-secret",
		},
	}, "")
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
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name":   "Unsupported",
		"kind":   "coffee",
		"config": map[string]interface{}{},
	}, "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("unsupported kind status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestCoreMonitorManagementRejectsInvalidConfig(t *testing.T) {
	server := setupTestServer(t)

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name":   "Missing URL",
		"kind":   "http",
		"config": map[string]interface{}{},
	}, "")
	if createResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}

	validResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name": "Valid URL",
		"kind": "http",
		"config": map[string]interface{}{
			"url": "https://example.com/health",
		},
	}, "")
	if validResp.Code != http.StatusCreated {
		t.Fatalf("valid create status = %d, body = %s", validResp.Code, validResp.Body.String())
	}
	var created struct {
		Data struct {
			Monitor struct {
				ID string `json:"id"`
			} `json:"monitor"`
		} `json:"data"`
	}
	decodeResponse(t, validResp, &created)

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+created.Data.Monitor.ID, map[string]interface{}{
		"config": map[string]interface{}{
			"url": "ftp://example.com/health",
		},
	}, "")
	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid update status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	if err := server.db.Model(&db.CoreMonitorConfig{}).
		Where("monitor_id = ?", created.Data.Monitor.ID).
		Update("config_json", `{"url":"ftp://example.com/health"}`).Error; err != nil {
		t.Fatalf("poison core monitor config: %v", err)
	}
	testResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+created.Data.Monitor.ID+"/test", nil, "")
	if testResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid test-now status = %d, body = %s", testResp.Code, testResp.Body.String())
	}
}

func TestCoreMonitorManagementEnforcesTargetPolicy(t *testing.T) {
	server := setupTestServer(t)

	localhostResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name": "Localhost Target",
		"kind": "http",
		"config": map[string]interface{}{
			"url": "http://localhost:8080/health?token=secret",
		},
	}, "")
	if localhostResp.Code != http.StatusBadRequest {
		t.Fatalf("localhost create status = %d, body = %s", localhostResp.Code, localhostResp.Body.String())
	}
	if strings.Contains(localhostResp.Body.String(), "token=secret") {
		t.Fatalf("localhost create leaked query secret: %s", localhostResp.Body.String())
	}

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name": "Metadata Target",
		"kind": "http",
		"config": map[string]interface{}{
			"url": "http://169.254.169.254/latest/meta-data?token=secret",
		},
	}, "")
	if createResp.Code != http.StatusBadRequest {
		t.Fatalf("blocked create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	if strings.Contains(createResp.Body.String(), "token=secret") {
		t.Fatalf("blocked create leaked query secret: %s", createResp.Body.String())
	}

	validResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name": "Public Target",
		"kind": "http",
		"config": map[string]interface{}{
			"url": "https://example.com/health?token=secret",
		},
	}, "")
	if validResp.Code != http.StatusCreated {
		t.Fatalf("valid create status = %d, body = %s", validResp.Code, validResp.Body.String())
	}
	var created struct {
		Data struct {
			Monitor struct {
				ID string `json:"id"`
			} `json:"monitor"`
		} `json:"data"`
	}
	decodeResponse(t, validResp, &created)

	configResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+created.Data.Monitor.ID+"/config", nil, "")
	if configResp.Code != http.StatusOK {
		t.Fatalf("config status = %d, body = %s", configResp.Code, configResp.Body.String())
	}
	if strings.Contains(configResp.Body.String(), "token=secret") {
		t.Fatalf("config response leaked query secret: %s", configResp.Body.String())
	}

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+created.Data.Monitor.ID, map[string]interface{}{
		"config": map[string]interface{}{
			"url": "http://127.0.0.1:8080/health",
		},
	}, "")
	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf("blocked update status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	if err := server.db.Model(&db.CoreMonitorConfig{}).
		Where("monitor_id = ?", created.Data.Monitor.ID).
		Update("config_json", `{"url":"http://169.254.169.254/latest/meta-data"}`).Error; err != nil {
		t.Fatalf("poison core monitor config: %v", err)
	}
	testResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+created.Data.Monitor.ID+"/test", nil, "")
	if testResp.Code != http.StatusBadRequest {
		t.Fatalf("blocked test-now status = %d, body = %s", testResp.Code, testResp.Body.String())
	}
}

func TestCoreMonitorManagementAllowsPrivateTargetsWhenConfigured(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{CoreMonitorAllowPrivateTargets: true})

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name": "Private Target",
		"kind": "tcp",
		"config": map[string]interface{}{
			"host": "10.0.0.5",
			"port": 443,
		},
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("private create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
}

func TestCoreMonitorManagementValidatesCatalogConfigTypes(t *testing.T) {
	server := setupTestServer(t)

	cases := []struct {
		kind          string
		validConfig   map[string]interface{}
		invalidConfig map[string]interface{}
	}{
		{
			kind:          "heartbeat",
			validConfig:   map[string]interface{}{"grace_seconds": 30},
			invalidConfig: map[string]interface{}{"grace_seconds": -1},
		},
		{
			kind:          "http",
			validConfig:   map[string]interface{}{"url": "https://example.com/health", "method": "HEAD", "expected_status": 204},
			invalidConfig: map[string]interface{}{"url": "ftp://example.com/health"},
		},
		{
			kind:          "http_keyword",
			validConfig:   map[string]interface{}{"url": "https://example.com/health", "required_contains": []string{"ok"}},
			invalidConfig: map[string]interface{}{"url": "https://example.com/health", "method": "POST"},
		},
		{
			kind:          "expected_status",
			validConfig:   map[string]interface{}{"url": "https://example.com/health", "expected_statuses": []int{200, 204}},
			invalidConfig: map[string]interface{}{"url": "https://example.com/health", "expected_status": 99},
		},
		{
			kind:          "tcp",
			validConfig:   map[string]interface{}{"host": "example.com", "port": 443},
			invalidConfig: map[string]interface{}{"host": "example.com", "port": 70000},
		},
		{
			kind:          "dns",
			validConfig:   map[string]interface{}{"host": "example.com", "record_type": "AAAA"},
			invalidConfig: map[string]interface{}{"host": "example.com", "record_type": "SOA"},
		},
		{
			kind:          "tls",
			validConfig:   map[string]interface{}{"host": "example.com", "warning_days": 14},
			invalidConfig: map[string]interface{}{"host": "", "port": 443},
		},
		{
			kind:          "udp",
			validConfig:   map[string]interface{}{"host": "example.com", "port": 53, "payload": "ping", "expected_response": "pong"},
			invalidConfig: map[string]interface{}{"host": "example.com", "port": 53, "payload": "ping"},
		},
		{
			kind:          "api_request",
			validConfig:   map[string]interface{}{"url": "https://example.com/api", "method": "POST", "expected_status": 201},
			invalidConfig: map[string]interface{}{"url": "https://example.com/api", "method": "TRACE"},
		},
		{
			kind:          "domain_expiration",
			validConfig:   map[string]interface{}{"domain": "example.com", "warning_days": 30},
			invalidConfig: map[string]interface{}{"domain": "https://example.com"},
		},
		{
			kind:          "ping",
			validConfig:   map[string]interface{}{"host": "example.com", "method": "tcp", "port": 443},
			invalidConfig: map[string]interface{}{"host": "example.com", "method": "icmp", "port": 443},
		},
		{
			kind:          "smtp",
			validConfig:   map[string]interface{}{"protocol": "smtp", "host": "mail.example.com", "port": 25, "tls_mode": "starttls"},
			invalidConfig: map[string]interface{}{"protocol": "smtp", "host": "mail.example.com", "auth_enabled": true},
		},
		{
			kind:          "imap",
			validConfig:   map[string]interface{}{"protocol": "imap", "host": "mail.example.com", "port": 993, "tls_mode": "implicit"},
			invalidConfig: map[string]interface{}{"protocol": "imap", "host": "mail.example.com", "tls_mode": "bad"},
		},
		{
			kind:          "pop3",
			validConfig:   map[string]interface{}{"protocol": "pop", "host": "mail.example.com", "port": 995, "tls_mode": "implicit"},
			invalidConfig: map[string]interface{}{"protocol": "pop", "host": "", "port": 995},
		},
		{
			kind:          "synthetic",
			validConfig:   map[string]interface{}{"steps": []map[string]interface{}{{"type": "api", "url": "https://example.com/api", "method": "GET"}}},
			invalidConfig: map[string]interface{}{"steps": []map[string]interface{}{{"type": "api", "url": "https://example.com/api", "method": "TRACE"}}},
		},
		{
			kind:          "playwright",
			validConfig:   map[string]interface{}{"url": "https://example.com", "browser": "chromium", "steps": []map[string]interface{}{{"action": "goto", "url": "https://example.com"}}},
			invalidConfig: map[string]interface{}{"steps": []map[string]interface{}{{"action": "click"}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
				"name":   "Valid " + tc.kind,
				"kind":   tc.kind,
				"type":   tc.kind,
				"config": tc.validConfig,
			}, "")
			if createResp.Code != http.StatusCreated {
				t.Fatalf("valid create %s status = %d, body = %s", tc.kind, createResp.Code, createResp.Body.String())
			}

			invalidResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
				"name":   "Invalid " + tc.kind,
				"kind":   tc.kind,
				"type":   tc.kind,
				"config": tc.invalidConfig,
			}, "")
			if invalidResp.Code != http.StatusBadRequest {
				t.Fatalf("invalid create %s status = %d, body = %s", tc.kind, invalidResp.Code, invalidResp.Body.String())
			}
		})
	}
}

func TestHeartbeatMonitorTokenAndIngestRoutes(t *testing.T) {
	server := setupTestServer(t)

	httpResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name": "Convertible HTTP monitor",
		"kind": "http",
		"config": map[string]interface{}{
			"url": "https://example.com/health",
		},
	}, "")
	if httpResp.Code != http.StatusCreated {
		t.Fatalf("create convertible monitor status = %d, body = %s", httpResp.Code, httpResp.Body.String())
	}
	var convertible struct {
		Data struct {
			Monitor struct {
				ID string `json:"id"`
			} `json:"monitor"`
		} `json:"data"`
	}
	decodeResponse(t, httpResp, &convertible)
	convertResp := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+convertible.Data.Monitor.ID, map[string]interface{}{
		"kind": "heartbeat",
		"type": "heartbeat",
		"config": map[string]interface{}{
			"grace_seconds": 30,
		},
	}, "")
	if convertResp.Code != http.StatusOK {
		t.Fatalf("convert heartbeat monitor status = %d, body = %s", convertResp.Code, convertResp.Body.String())
	}
	var converted struct {
		Data struct {
			Config struct {
				HeartbeatToken string `json:"heartbeat_token"`
			} `json:"config"`
		} `json:"data"`
	}
	decodeResponse(t, convertResp, &converted)
	if converted.Data.Config.HeartbeatToken == "" {
		t.Fatalf("converted heartbeat token is empty")
	}

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name":             "Nightly backup heartbeat",
		"kind":             "heartbeat",
		"interval_seconds": 300,
		"config": map[string]interface{}{
			"grace_seconds": 60,
			"token":         "caller-supplied-token-must-not-be-used",
		},
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create heartbeat monitor status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	assertNotContains(t, createResp.Body.String(), "caller-supplied-token-must-not-be-used")

	var created struct {
		Success bool `json:"success"`
		Data    struct {
			Monitor struct {
				ID string `json:"id"`
			} `json:"monitor"`
			Config struct {
				Kind           string                 `json:"kind"`
				Config         map[string]interface{} `json:"config"`
				HeartbeatToken string                 `json:"heartbeat_token"`
			} `json:"config"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	if !created.Success || created.Data.Monitor.ID == "" || created.Data.Config.Kind != "heartbeat" {
		t.Fatalf("created heartbeat = %+v, want heartbeat monitor", created)
	}
	token := created.Data.Config.HeartbeatToken
	if token == "" || token == "caller-supplied-token-must-not-be-used" {
		t.Fatalf("heartbeat token = %q, want generated token", token)
	}

	configResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+created.Data.Monitor.ID+"/config", nil, "")
	if configResp.Code != http.StatusOK {
		t.Fatalf("get heartbeat config status = %d, body = %s", configResp.Code, configResp.Body.String())
	}
	assertNotContains(t, configResp.Body.String(), token)

	successResp := performRawRequest(t, server, http.MethodPost, "/v1/heartbeats/"+token+"/success", strings.NewReader(`{"job":"backup","output":"ok"}`), "")
	if successResp.Code != http.StatusOK {
		t.Fatalf("heartbeat success status = %d, body = %s", successResp.Code, successResp.Body.String())
	}
	var successSignal struct {
		Success bool `json:"success"`
		Data    struct {
			MonitorID        string    `json:"monitor_id"`
			Health           string    `json:"health"`
			ReportID         string    `json:"report_id"`
			ReceivedAt       time.Time `json:"received_at"`
			PayloadTruncated bool      `json:"payload_truncated"`
		} `json:"data"`
	}
	decodeResponse(t, successResp, &successSignal)
	if !successSignal.Success || successSignal.Data.MonitorID != created.Data.Monitor.ID || successSignal.Data.Health != "up" || successSignal.Data.ReportID == "" || successSignal.Data.PayloadTruncated {
		t.Fatalf("success signal = %+v, want up report without truncation", successSignal)
	}

	var config db.CoreMonitorConfig
	if err := server.db.Where("monitor_id = ?", created.Data.Monitor.ID).First(&config).Error; err != nil {
		t.Fatalf("find heartbeat config: %v", err)
	}
	if config.HeartbeatTokenHash == "" || strings.Contains(config.ConfigJSON, token) || config.LastSignalAt == nil || config.LastSuccessAt == nil {
		t.Fatalf("heartbeat config after success = %+v, want token hash and signal/success timestamps", config)
	}

	failurePayload := "password=super-secret token=raw-token-value " + strings.Repeat("x", heartbeatPayloadMaxBytes+200)
	failureResp := performRawRequest(t, server, http.MethodPost, "/v1/heartbeats/"+token+"/failure", strings.NewReader(failurePayload), "")
	if failureResp.Code != http.StatusOK {
		t.Fatalf("heartbeat failure status = %d, body = %s", failureResp.Code, failureResp.Body.String())
	}
	var failureSignal struct {
		Data struct {
			Health           string `json:"health"`
			PayloadTruncated bool   `json:"payload_truncated"`
		} `json:"data"`
	}
	decodeResponse(t, failureResp, &failureSignal)
	if failureSignal.Data.Health != "down" || !failureSignal.Data.PayloadTruncated {
		t.Fatalf("failure signal = %+v, want down truncated signal", failureSignal)
	}

	var failureReport db.MonitorReport
	if err := server.db.Where("monitor_id = ? AND health = ?", created.Data.Monitor.ID, "down").Order("created_at DESC").First(&failureReport).Error; err != nil {
		t.Fatalf("find failure report: %v", err)
	}
	if strings.Contains(failureReport.Payload, strings.Repeat("x", heartbeatPayloadMaxBytes+1)) || !strings.Contains(failureReport.Payload, `"payload_truncated":true`) {
		t.Fatalf("failure payload length = %d body = %.80s, want truncated payload marker", len(failureReport.Payload), failureReport.Payload)
	}
	assertNotContains(t, failureReport.Payload, "super-secret")
	assertNotContains(t, failureReport.Payload, "raw-token-value")
	assertContains(t, failureReport.Payload, "[redacted]")
	historyResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+created.Data.Monitor.ID+"/history", nil, "")
	if historyResp.Code != http.StatusOK {
		t.Fatalf("heartbeat history status = %d, body = %s", historyResp.Code, historyResp.Body.String())
	}
	assertNotContains(t, historyResp.Body.String(), "super-secret")
	assertNotContains(t, historyResp.Body.String(), "raw-token-value")
	assertContains(t, historyResp.Body.String(), "[redacted]")
	if err := server.db.Where("monitor_id = ?", created.Data.Monitor.ID).First(&config).Error; err != nil {
		t.Fatalf("reload heartbeat config: %v", err)
	}
	if config.LastFailureAt == nil || config.LastSignalAt == nil {
		t.Fatalf("heartbeat config after failure = %+v, want signal/failure timestamps", config)
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", created.Data.Monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find heartbeat incident: %v", err)
	}
	if incident.Status != "open" {
		t.Fatalf("heartbeat incident = %+v, want open incident from failure report", incident)
	}
	incidentResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID, nil, "")
	if incidentResp.Code != http.StatusOK {
		t.Fatalf("heartbeat incident detail status = %d, body = %s", incidentResp.Code, incidentResp.Body.String())
	}
	assertNotContains(t, incidentResp.Body.String(), "super-secret")
	assertNotContains(t, incidentResp.Body.String(), "raw-token-value")
	assertContains(t, incidentResp.Body.String(), "[redacted]")
	assertContains(t, incidentResp.Body.String(), failureReport.ID)
	timelineResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID+"/timeline", nil, "")
	if timelineResp.Code != http.StatusOK {
		t.Fatalf("heartbeat incident timeline status = %d, body = %s", timelineResp.Code, timelineResp.Body.String())
	}
	assertNotContains(t, timelineResp.Body.String(), "super-secret")
	assertNotContains(t, timelineResp.Body.String(), "raw-token-value")
	assertContains(t, timelineResp.Body.String(), "[redacted]")
	assertContains(t, timelineResp.Body.String(), failureReport.ID)

	invalidResp := performRawRequest(t, server, http.MethodPost, "/v1/heartbeats/not-"+token+"/success", strings.NewReader("ok"), "")
	if invalidResp.Code != http.StatusUnauthorized {
		t.Fatalf("invalid heartbeat token status = %d, body = %s", invalidResp.Code, invalidResp.Body.String())
	}

	pauseResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+created.Data.Monitor.ID+"/pause", nil, "")
	if pauseResp.Code != http.StatusOK {
		t.Fatalf("pause heartbeat status = %d, body = %s", pauseResp.Code, pauseResp.Body.String())
	}
	pausedResp := performRawRequest(t, server, http.MethodPost, "/v1/heartbeats/"+token+"/success", strings.NewReader("ok"), "")
	if pausedResp.Code != http.StatusUnauthorized {
		t.Fatalf("paused heartbeat token status = %d, body = %s", pausedResp.Code, pausedResp.Body.String())
	}

	resumeResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+created.Data.Monitor.ID+"/resume", nil, "")
	if resumeResp.Code != http.StatusOK {
		t.Fatalf("resume heartbeat status = %d, body = %s", resumeResp.Code, resumeResp.Body.String())
	}
	deleteResp := performJSONRequest(t, server, http.MethodDelete, "/v1/monitors/"+created.Data.Monitor.ID, nil, "")
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete heartbeat status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	deletedResp := performRawRequest(t, server, http.MethodPost, "/v1/heartbeats/"+token+"/success", strings.NewReader("ok"), "")
	if deletedResp.Code != http.StatusUnauthorized {
		t.Fatalf("deleted heartbeat token status = %d, body = %s", deletedResp.Code, deletedResp.Body.String())
	}
}

func TestCoreMonitorIncidentSeverityOverrideAppearsInResponses(t *testing.T) {
	server := setupTestServer(t)
	monitor := db.Monitor{
		ID:                       "monitor-core-severity-override",
		AgentID:                  "agent-core-severity-override",
		Name:                     "Checkout journey",
		Type:                     "synthetic",
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		ReportingIntervalSeconds: 60,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create core monitor: %v", err)
	}
	if err := server.db.Create(&db.CoreMonitorConfig{
		MonitorID:       monitor.ID,
		Kind:            "synthetic_multi_step",
		ConfigJSON:      `{"steps":[],"incident_severity":" Critical "}`,
		SecretRefJSON:   `{}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		NextRunAt:       time.Now().UTC(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}).Error; err != nil {
		t.Fatalf("create core monitor config: %v", err)
	}

	_, err := server.reportService.StoreMonitorReport(monitor.ID, service.MonitorReportPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Health:    "down",
		Metrics: map[string]interface{}{
			"runner":       "core",
			"failure_step": "checkout",
		},
	})
	if err != nil {
		t.Fatalf("store core down report: %v", err)
	}

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find core incident: %v", err)
	}
	if incident.Severity != "critical" {
		t.Fatalf("core incident severity = %q, want critical", incident.Severity)
	}

	listResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?monitor_id="+monitor.ID, nil, "")
	if listResp.Code != http.StatusOK {
		t.Fatalf("list core incidents status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Incidents []struct {
				Severity string `json:"severity"`
			} `json:"incidents"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, listResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || len(listed.Data.Incidents) != 1 || listed.Data.Incidents[0].Severity != "critical" {
		t.Fatalf("listed core incident severity = %+v, want critical", listed)
	}

	detailResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID, nil, "")
	if detailResp.Code != http.StatusOK {
		t.Fatalf("detail core incident status = %d, body = %s", detailResp.Code, detailResp.Body.String())
	}
	var detail struct {
		Success bool `json:"success"`
		Data    struct {
			Incident struct {
				Severity string `json:"severity"`
			} `json:"incident"`
		} `json:"data"`
	}
	decodeResponse(t, detailResp, &detail)
	if !detail.Success || detail.Data.Incident.Severity != "critical" {
		t.Fatalf("detail core incident severity = %+v, want critical", detail)
	}
}

func TestCoreMonitorIncidentSeverityInvalidOverrideFallsBack(t *testing.T) {
	server := setupTestServer(t)
	monitor := db.Monitor{
		ID:                       "monitor-core-severity-invalid",
		AgentID:                  "agent-core-severity-invalid",
		Name:                     "API health",
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		ReportingIntervalSeconds: 60,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create core monitor: %v", err)
	}
	if err := server.db.Create(&db.CoreMonitorConfig{
		MonitorID:       monitor.ID,
		Kind:            "http",
		ConfigJSON:      `{"url":"https://api.example.com/health","incident_severity":"urgent"}`,
		SecretRefJSON:   `{}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		NextRunAt:       time.Now().UTC(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}).Error; err != nil {
		t.Fatalf("create core monitor config: %v", err)
	}

	_, err := server.reportService.StoreMonitorReport(monitor.ID, service.MonitorReportPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Health:    "down",
		Metrics:   map[string]interface{}{"runner": "core", "status_code": 500},
	})
	if err != nil {
		t.Fatalf("store core down report: %v", err)
	}

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find core incident: %v", err)
	}
	if incident.Severity != "high" {
		t.Fatalf("core incident severity = %q, want high fallback", incident.Severity)
	}
}
