package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/service"
)

func TestDataLifecycleSettingsAuditEventRecordsActorAndSafeMetadata(t *testing.T) {
	server := setupStatusPageAuthTestServer(t)
	token := loginStatusPageTestAdmin(t, server)
	archiveDir := "data/private-archive-path"

	updateResp := performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", map[string]interface{}{
		"raw_report_hot_days":   120,
		"archive_raw_reports":   true,
		"archive_dir":           archiveDir,
		"rollups_enabled":       true,
		"rollup_retention_days": nil,
		"archive_schedule":      "manual",
	}, token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update settings status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	var event db.AuditEvent
	if err := server.db.Where("action = ?", service.DataLifecycleAuditActionSettingsUpdated).First(&event).Error; err != nil {
		t.Fatalf("load settings audit event: %v", err)
	}
	if event.AffectedObjectType != "data_lifecycle" || event.AffectedObjectID != "settings" {
		t.Fatalf("settings audit object = %s/%s, want data_lifecycle/settings", event.AffectedObjectType, event.AffectedObjectID)
	}
	if event.ActorType != "user" || event.ActorID != "admin" {
		t.Fatalf("settings audit actor = %s/%s, want user/admin", event.ActorType, event.ActorID)
	}
	if strings.Contains(event.MetadataJSON, archiveDir) {
		t.Fatalf("settings audit metadata leaked archive path: %s", event.MetadataJSON)
	}

	var metadata struct {
		ActionType        string   `json:"action_type"`
		ResultStatus      string   `json:"result_status"`
		ChangedFields     []string `json:"changed_fields"`
		ArchiveConfigured bool     `json:"archive_configured"`
	}
	if err := json.Unmarshal([]byte(event.MetadataJSON), &metadata); err != nil {
		t.Fatalf("decode settings audit metadata: %v", err)
	}
	if metadata.ActionType != "settings_update" || metadata.ResultStatus != "success" || !metadata.ArchiveConfigured {
		t.Fatalf("settings audit metadata = %+v, want successful settings update", metadata)
	}
	for _, field := range []string{"raw_report_hot_days", "archive_dir", "archive_schedule"} {
		if !containsString(metadata.ChangedFields, field) {
			t.Fatalf("changed fields = %#v, want %q", metadata.ChangedFields, field)
		}
	}
}

func TestDataLifecycleActionAuditEventsAppearInLogs(t *testing.T) {
	server := setupStatusPageAuthTestServer(t)
	token := loginStatusPageTestAdmin(t, server)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)

	reportTime := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	for _, health := range []string{"up", "down"} {
		resp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
			"timestamp": reportTime.Format(time.RFC3339),
			"health":    health,
			"metrics":   map[string]interface{}{},
		}, registered.Data.Token)
		if resp.Code != http.StatusOK {
			t.Fatalf("monitor report status = %d, body = %s", resp.Code, resp.Body.String())
		}
	}
	if err := server.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", registeredMonitor.Data.MonitorID).Update("created_at", reportTime).Error; err != nil {
		t.Fatalf("set monitor report created_at: %v", err)
	}

	rollupResp := performJSONRequest(t, server, http.MethodPost, "/v1/settings/data-lifecycle/actions/rollup", map[string]string{
		"date": "2026-05-12",
	}, token)
	if rollupResp.Code != http.StatusOK {
		t.Fatalf("rollup status = %d, body = %s", rollupResp.Code, rollupResp.Body.String())
	}

	archiveDir := t.TempDir()
	settingsResp := performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", map[string]interface{}{
		"raw_report_hot_days":   1,
		"archive_raw_reports":   true,
		"archive_dir":           archiveDir,
		"rollups_enabled":       true,
		"rollup_retention_days": nil,
		"archive_schedule":      "manual",
	}, token)
	if settingsResp.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, body = %s", settingsResp.Code, settingsResp.Body.String())
	}

	archiveResp := performJSONRequest(t, server, http.MethodPost, "/v1/settings/data-lifecycle/actions/archive", nil, token)
	if archiveResp.Code != http.StatusOK {
		t.Fatalf("archive status = %d, body = %s", archiveResp.Code, archiveResp.Body.String())
	}

	rollupEvent := loadDataLifecycleAuditEvent(t, server, service.DataLifecycleAuditActionRollupRun)
	if rollupEvent.ActorType != "user" || rollupEvent.ActorID != "admin" {
		t.Fatalf("rollup audit actor = %s/%s, want user/admin", rollupEvent.ActorType, rollupEvent.ActorID)
	}
	var rollupMetadata struct {
		ActionType   string `json:"action_type"`
		ResultStatus string `json:"result_status"`
		Date         string `json:"date"`
		MonitorDays  int    `json:"monitor_days"`
		ReportCount  int    `json:"report_count"`
	}
	if err := json.Unmarshal([]byte(rollupEvent.MetadataJSON), &rollupMetadata); err != nil {
		t.Fatalf("decode rollup audit metadata: %v", err)
	}
	if rollupMetadata.ActionType != "manual_rollup" || rollupMetadata.ResultStatus != "success" || rollupMetadata.Date != "2026-05-12" || rollupMetadata.MonitorDays != 1 || rollupMetadata.ReportCount != 2 {
		t.Fatalf("rollup metadata = %+v, want successful rollup counts", rollupMetadata)
	}

	archiveEvent := loadDataLifecycleAuditEvent(t, server, service.DataLifecycleAuditActionArchiveRun)
	if strings.Contains(archiveEvent.MetadataJSON, archiveDir) {
		t.Fatalf("archive audit metadata leaked archive path: %s", archiveEvent.MetadataJSON)
	}
	var archiveMetadata struct {
		ActionType              string `json:"action_type"`
		ResultStatus            string `json:"result_status"`
		AgentReportsArchived    int    `json:"agent_reports_archived"`
		MonitorReportsArchived  int    `json:"monitor_reports_archived"`
		SkippedBecauseNoReports bool   `json:"skipped_because_no_reports"`
	}
	if err := json.Unmarshal([]byte(archiveEvent.MetadataJSON), &archiveMetadata); err != nil {
		t.Fatalf("decode archive audit metadata: %v", err)
	}
	if archiveMetadata.ActionType != "manual_archive" || archiveMetadata.ResultStatus == "" {
		t.Fatalf("archive metadata = %+v, want manual archive result", archiveMetadata)
	}

	assertLifecycleEventLogFilter(t, server, token, service.DataLifecycleAuditActionRollupRun, "2 reports")
	assertLifecycleEventLogFilter(t, server, token, service.DataLifecycleAuditActionArchiveRun, "monitor reports")
}

func loadDataLifecycleAuditEvent(t *testing.T, server *Server, action string) db.AuditEvent {
	t.Helper()
	var event db.AuditEvent
	if err := server.db.Where("action = ? AND affected_object_type = ?", action, "data_lifecycle").First(&event).Error; err != nil {
		t.Fatalf("load lifecycle audit event %q: %v", action, err)
	}
	return event
}

func assertLifecycleEventLogFilter(t *testing.T, server *Server, token string, eventType string, wantMessageFragment string) {
	t.Helper()
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/events?source=data_lifecycle&type="+eventType, nil, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("events status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var listed struct {
		Data struct {
			Events []struct {
				Type    string `json:"type"`
				Source  string `json:"source"`
				Message string `json:"message"`
			} `json:"events"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &listed)
	if listed.Data.Count != 1 || len(listed.Data.Events) != 1 {
		t.Fatalf("events response = %+v, want one filtered event", listed.Data)
	}
	event := listed.Data.Events[0]
	if event.Type != eventType || event.Source != "data_lifecycle" || !strings.Contains(event.Message, wantMessageFragment) {
		t.Fatalf("filtered event = %+v, want %s data_lifecycle event containing %q", event, eventType, wantMessageFragment)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
