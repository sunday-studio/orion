package api

import (
	"net/http"
	"testing"
)

func TestDataLifecycleTraceRetentionSetting(t *testing.T) {
	server := setupTestServer(t)
	type settingsBody struct {
		Data struct {
			Settings struct {
				TraceRetentionDays int `json:"trace_retention_days"`
			} `json:"settings"`
		} `json:"data"`
	}
	var got settingsBody
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/settings/data-lifecycle", nil, ""), &got)
	if got.Data.Settings.TraceRetentionDays != 7 {
		t.Fatalf("default = %d, want 7", got.Data.Settings.TraceRetentionDays)
	}

	base := func(extra map[string]any) map[string]any {
		body := map[string]any{"raw_report_hot_days": 90, "archive_raw_reports": false, "archive_dir": "", "rollups_enabled": true,
			"rollup_retention_days": nil, "archive_schedule": "manual"}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}
	resp := performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", base(map[string]any{"trace_retention_days": 3}), "")
	if resp.Code != http.StatusOK {
		t.Fatalf("update = %d %s", resp.Code, resp.Body.String())
	}
	decodeResponse(t, resp, &got)
	if got.Data.Settings.TraceRetentionDays != 3 {
		t.Fatalf("updated = %d, want 3", got.Data.Settings.TraceRetentionDays)
	}
	// An omitted field keeps its value, and out-of-range values are refused.
	decodeResponse(t, performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", base(nil), ""), &got)
	if got.Data.Settings.TraceRetentionDays != 3 {
		t.Fatalf("omitted field changed to %d", got.Data.Settings.TraceRetentionDays)
	}
	for _, bad := range []int{0, 3651} {
		if resp := performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", base(map[string]any{"trace_retention_days": bad}), ""); resp.Code != http.StatusBadRequest {
			t.Fatalf("trace_retention_days=%d status = %d, want 400", bad, resp.Code)
		}
	}
}
