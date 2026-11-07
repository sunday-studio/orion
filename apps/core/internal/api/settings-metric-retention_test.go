package api

import (
	"net/http"
	"testing"
)

func TestDataLifecycleMetricRetentionSettings(t *testing.T) {
	server := setupTestServer(t)
	type settingsBody struct {
		Data struct {
			Settings struct {
				MetricSampleRetentionDays int `json:"metric_sample_retention_days"`
				MetricRollupRetentionDays int `json:"metric_rollup_retention_days"`
			} `json:"settings"`
		} `json:"data"`
	}
	var got settingsBody
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/settings/data-lifecycle", nil, ""), &got)
	if got.Data.Settings.MetricSampleRetentionDays != 7 || got.Data.Settings.MetricRollupRetentionDays != 90 {
		t.Fatalf("defaults = %+v, want 7 and 90", got.Data.Settings)
	}

	base := func(extra map[string]any) map[string]any {
		body := map[string]any{"raw_report_hot_days": 90, "archive_raw_reports": false, "archive_dir": "", "rollups_enabled": true,
			"rollup_retention_days": nil, "archive_schedule": "manual"}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}
	resp := performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", base(map[string]any{"metric_sample_retention_days": 3, "metric_rollup_retention_days": 30}), "")
	if resp.Code != http.StatusOK {
		t.Fatalf("update = %d %s", resp.Code, resp.Body.String())
	}
	decodeResponse(t, resp, &got)
	if got.Data.Settings.MetricSampleRetentionDays != 3 || got.Data.Settings.MetricRollupRetentionDays != 30 {
		t.Fatalf("updated = %+v", got.Data.Settings)
	}
	// Omitted fields keep their value, and out-of-range values are refused.
	decodeResponse(t, performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", base(nil), ""), &got)
	if got.Data.Settings.MetricSampleRetentionDays != 3 {
		t.Fatalf("omitted field changed: %+v", got.Data.Settings)
	}
	for _, bad := range []int{0, 3651} {
		if resp := performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", base(map[string]any{"metric_sample_retention_days": bad}), ""); resp.Code != http.StatusBadRequest {
			t.Fatalf("metric_sample_retention_days=%d status = %d", bad, resp.Code)
		}
	}
}
