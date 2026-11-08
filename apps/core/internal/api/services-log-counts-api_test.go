package api

import (
	"net/http"
	"testing"
	"time"

	"orion/core/internal/logstore"

	"github.com/gin-gonic/gin"
)

func TestServiceLogCountsBucketsTheRollupBySeverity(t *testing.T) {
	server := setupLogsServer(t, nil)
	seedServiceTargets(t, server)
	id := createServiceVia(t, server, gin.H{"name": "Checkout"})
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+id+"/links", gin.H{"kind": "app", "id": "app-1"}, "")

	at := time.Now().UTC().Truncate(time.Minute).Add(-10 * time.Minute)
	mk := func(i int, ts time.Time, sev string) logstore.Entry {
		return logstore.Entry{ID: logstore.NewEntryID(ts.Add(time.Duration(i))), AppID: "app-1", Timestamp: ts, IngestedAt: ts, Severity: sev, SeverityNumber: 9, Body: "x", Size: 1}
	}
	server.logs.store.Enqueue([]logstore.Entry{mk(0, at.Add(5*time.Second), "error"), mk(1, at.Add(20*time.Second), "error"), mk(2, at.Add(30*time.Second), "info"), mk(3, at.Add(2*time.Minute), "error")})
	server.logs.store.Flush(5 * time.Second)

	var got struct {
		Data struct {
			StepSeconds int      `json:"step_seconds"`
			AppIDs      []string `json:"app_ids"`
			Buckets     []struct {
				Total  int64            `json:"total"`
				Counts map[string]int64 `json:"counts"`
			} `json:"buckets"`
		} `json:"data"`
	}
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/services/"+id+"/log-counts?from=-1h", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d %s", resp.Code, resp.Body.String())
	}
	decodeResponse(t, resp, &got)
	if got.Data.StepSeconds != 60 || len(got.Data.AppIDs) != 1 || len(got.Data.Buckets) != 2 ||
		got.Data.Buckets[0].Total != 3 || got.Data.Buckets[0].Counts["error"] != 2 || got.Data.Buckets[1].Counts["error"] != 1 {
		t.Fatalf("buckets = %+v", got.Data)
	}

	// A coarser step merges the minutes.
	var coarse struct {
		Data struct {
			Buckets []struct{ Total int64 } `json:"buckets"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/services/"+id+"/log-counts?from=-1h&step_seconds=3600", nil, ""), &coarse)
	var total int64
	for _, b := range coarse.Data.Buckets {
		total += b.Total
	}
	if total != 4 {
		t.Fatalf("coarse buckets = %+v", coarse.Data.Buckets)
	}

	for name, tc := range map[string]struct {
		path string
		want int
	}{
		"missing service": {"/v1/services/svc-missing/log-counts", http.StatusNotFound},
		"bad step":        {"/v1/services/" + id + "/log-counts?step_seconds=90", http.StatusBadRequest},
		"too many":        {"/v1/services/" + id + "/log-counts?from=-7d", http.StatusBadRequest},
		"too long":        {"/v1/services/" + id + "/log-counts?from=-30d", http.StatusBadRequest},
		"from after to":   {"/v1/services/" + id + "/log-counts?from=-1h&to=-2h", http.StatusBadRequest},
	} {
		if resp := performJSONRequest(t, server, http.MethodGet, tc.path, nil, ""); resp.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, resp.Code, tc.want)
		}
	}
}
