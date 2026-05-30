package api

import (
	"net/http"
	"testing"
	"time"

	"orion/core/internal/db"
)

func TestDataLifecycleSettingsFlow(t *testing.T) {
	server := setupTestServer(t)

	getResp := performJSONRequest(t, server, http.MethodGet, "/v1/settings/data-lifecycle", nil, "")
	if getResp.Code != http.StatusOK {
		t.Fatalf("get settings status = %d, body = %s", getResp.Code, getResp.Body.String())
	}

	var settingsResp struct {
		Success bool `json:"success"`
		Data    struct {
			Settings struct {
				RawReportHotDays  int    `json:"raw_report_hot_days"`
				ArchiveRawReports bool   `json:"archive_raw_reports"`
				ArchiveDir        string `json:"archive_dir"`
				RollupsEnabled    bool   `json:"rollups_enabled"`
				ArchiveSchedule   string `json:"archive_schedule"`
			} `json:"settings"`
		} `json:"data"`
	}
	decodeResponse(t, getResp, &settingsResp)
	if !settingsResp.Success || settingsResp.Data.Settings.RawReportHotDays != 90 {
		t.Fatalf("settings response = %+v, want default hot days", settingsResp)
	}

	updateResp := performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", map[string]interface{}{
		"raw_report_hot_days":   120,
		"archive_raw_reports":   true,
		"archive_dir":           "data/archive",
		"rollups_enabled":       true,
		"rollup_retention_days": nil,
		"archive_schedule":      "manual",
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update settings status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	decodeResponse(t, updateResp, &settingsResp)
	if settingsResp.Data.Settings.RawReportHotDays != 120 || settingsResp.Data.Settings.ArchiveSchedule != "manual" {
		t.Fatalf("updated settings = %+v, want updated values", settingsResp.Data.Settings)
	}
}

func TestDataLifecycleActionsFlow(t *testing.T) {
	server := setupTestServer(t)
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
	}, "")
	if rollupResp.Code != http.StatusOK {
		t.Fatalf("rollup status = %d, body = %s", rollupResp.Code, rollupResp.Body.String())
	}

	var rollup db.MonitorUptimeRollup
	if err := server.db.Where("monitor_id = ? AND date = ?", registeredMonitor.Data.MonitorID, "2026-05-12").First(&rollup).Error; err != nil {
		t.Fatalf("find rollup: %v", err)
	}
	if rollup.UpCount != 1 || rollup.DownCount != 1 || rollup.TotalCount != 2 {
		t.Fatalf("rollup = %+v, want one up, one down, two total", rollup)
	}

	updateResp := performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", map[string]interface{}{
		"raw_report_hot_days":   0,
		"archive_raw_reports":   true,
		"archive_dir":           t.TempDir(),
		"rollups_enabled":       true,
		"rollup_retention_days": nil,
		"archive_schedule":      "manual",
	}, "")
	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid settings status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	updateResp = performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", map[string]interface{}{
		"raw_report_hot_days":   1,
		"archive_raw_reports":   true,
		"archive_dir":           t.TempDir(),
		"rollups_enabled":       true,
		"rollup_retention_days": nil,
		"archive_schedule":      "manual",
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	archiveResp := performJSONRequest(t, server, http.MethodPost, "/v1/settings/data-lifecycle/actions/archive", nil, "")
	if archiveResp.Code != http.StatusOK {
		t.Fatalf("archive status = %d, body = %s", archiveResp.Code, archiveResp.Body.String())
	}
}
