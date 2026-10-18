package api

import (
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"
)

// A fresh install must read as empty even though Core watches itself: its worker monitor and
// owner agent are system items, hidden from counts and default lists, but a dead worker still
// raises an incident and turns the health chip.
func TestCoreSelfMonitorIsSystemAndKeepsFirstRunEmpty(t *testing.T) {
	cfg := &config.Config{CoreWorkerStaleSeconds: 60, AlertRecoveryNotifications: true}
	server := setupTestServerWithConfig(t, cfg)
	now := time.Now().UTC()
	self := service.NewSelfMonitorService(server.db, utils.NewLogger(), cfg, service.NewRuntimeDiagnosticsService(server.db, utils.NewLogger()))
	server.db.Save(&db.CoreWorkerStatus{WorkerID: "w1", ProcessKind: service.CoreMonitorWorkerProcessKind, Status: "running", StartedAt: now, LastHeartbeatAt: now.Add(-time.Second)})
	if err := self.Check(t.Context(), now); err != nil {
		t.Fatal(err)
	}

	var ov struct{ Data OverviewResponse }
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/overview", nil, ""), &ov)
	o := ov.Data
	if o.State != "empty" || o.Stats.MonitorsTotal != 0 || o.Stats.MonitorsUp != 0 || o.Stats.ServersTotal != 0 || o.Stats.ServersReporting != 0 ||
		o.FirstRun.HasServers || o.FirstRun.HasMonitors || len(o.Services) != 0 {
		t.Fatalf("fresh install overview = %+v", o)
	}

	listed := func(path string) string {
		return performJSONRequest(t, server, http.MethodGet, path, nil, "").Body.String()
	}
	for _, path := range []string{"/v1/agents", "/v1/monitors"} {
		if body := listed(path); !strings.Contains(body, `"count":0`) {
			t.Fatalf("%s lists system items by default: %s", path, body)
		}
		if body := listed(path + "?include_system=true"); !strings.Contains(body, `"system":true`) || !strings.Contains(body, `"count":1`) {
			t.Fatalf("%s?include_system=true = %s", path, body)
		}
	}
	for _, path := range []string{"/v1/agents/summary", "/v1/monitors/summary"} {
		if body := listed(path); !strings.Contains(body, `"total":0`) {
			t.Fatalf("%s counts system items: %s", path, body)
		}
	}
	var health struct {
		Data struct {
			Monitors struct{ Total int } `json:"monitors"`
			Agents   struct{ Total int } `json:"agents"`
		}
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/health/summary", nil, ""), &health)
	if health.Data.Monitors.Total != 0 || health.Data.Agents.Total != 0 {
		t.Fatalf("health summary counts system items: %+v", health.Data)
	}
	if body := listed("/v1/search?q=core"); !strings.Contains(body, `"count":0`) {
		t.Fatalf("search returns system items: %s", body)
	}

	// The worker dies: the incident and the header chip must still appear.
	if err := self.Check(t.Context(), now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/overview", nil, ""), &ov)
	o = ov.Data
	if o.State != "issues" || o.OngoingIncidentCount != 1 || o.NeedsYou == nil || o.Stats.MonitorsTotal != 0 || o.Stats.ServersTotal != 0 {
		t.Fatalf("dead worker overview = %+v", o)
	}
	if body := listed("/v1/health/summary"); !strings.Contains(body, `"overall_health":"down"`) || !strings.Contains(body, `"total":0`) {
		t.Fatalf("health summary with a dead worker = %s", body)
	}
}
