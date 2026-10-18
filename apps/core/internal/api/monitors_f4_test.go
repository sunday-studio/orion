package api

import (
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type createdMonitor struct {
	Data struct {
		Monitor MonitorResponse `json:"monitor"`
		Config  struct {
			HeartbeatToken string `json:"heartbeat_token"`
		} `json:"config"`
	} `json:"data"`
}

func createHTTPMonitor(t *testing.T, server *Server, name string, extra gin.H) createdMonitor {
	t.Helper()
	body := gin.H{"name": name, "kind": "http", "interval_seconds": 30, "timeout_seconds": 10, "config": gin.H{"url": "https://example.com/health", "method": "GET"}}
	for k, v := range extra {
		body[k] = v
	}
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", body, "")
	var out createdMonitor
	if resp.Code != http.StatusCreated {
		t.Fatalf("create %s = %d %s", name, resp.Code, resp.Body.String())
	}
	decodeResponse(t, resp, &out)
	return out
}

func addChecks(server *Server, monitorID string, healths ...string) {
	now := time.Now().UTC()
	for i, h := range healths {
		at := now.Add(-time.Duration(len(healths)-i) * time.Minute)
		payload := `{"runner":"core","type":"http","status_code":200,"duration_ms":201}`
		if h != "up" {
			payload = `{"runner":"core","type":"http","error":"context deadline exceeded","duration_ms":10000}`
		}
		server.db.Create(&db.MonitorReport{ID: fmt.Sprintf("r-%s-%02d", monitorID, i), MonitorID: monitorID, Payload: payload, CollectedAt: at.Format(time.RFC3339), Health: h, CreatedAt: at})
	}
}

func TestMonitorListRowsFiltersAndExtras(t *testing.T) {
	server := setupTestServer(t)
	registerTestAgent(t, server)
	down := createHTTPMonitor(t, server, "api-https", gin.H{"runbook_url": "https://wiki.example.com/runbooks/api"})
	up := createHTTPMonitor(t, server, "web-https", nil)
	paused := createHTTPMonitor(t, server, "old-check", nil)
	tcp := createHTTPMonitor(t, server, "pg-tcp", gin.H{"kind": "tcp", "config": gin.H{"host": "db-02.example.com", "port": 5432}})
	addChecks(server, down.Data.Monitor.ID, "up", "up", "down", "down", "down")
	addChecks(server, up.Data.Monitor.ID, "up", "up", "up")
	addChecks(server, tcp.Data.Monitor.ID, "up")
	for id, h := range map[string]string{down.Data.Monitor.ID: "down", up.Data.Monitor.ID: "up", tcp.Data.Monitor.ID: "up"} {
		server.db.Model(&db.Monitor{}).Where("id = ?", id).Updates(map[string]any{"computed_health": h, "health": h, "reporting_interval_seconds": 3600})
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+paused.Data.Monitor.ID+"/pause", nil, ""); r.Code != http.StatusOK {
		t.Fatalf("pause = %d", r.Code)
	}

	type listResp struct {
		Data struct {
			Monitors    []MonitorResponse `json:"monitors"`
			Count       int64             `json:"count"`
			Total       int64             `json:"total"`
			HiddenCount int64             `json:"hidden_count"`
		} `json:"data"`
	}
	get := func(q string) listResp {
		var l listResp
		decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/monitors"+q, nil, ""), &l)
		return l
	}
	all := get("")
	if all.Data.Count != 4 || all.Data.HiddenCount != 0 {
		t.Fatalf("all = %+v", all.Data)
	}
	byName := map[string]MonitorResponse{}
	for _, m := range all.Data.Monitors {
		byName[m.Name] = m
	}
	a := byName["api-https"]
	if a.Target != "GET example.com/health" || a.Kind != "http" || a.IntervalSeconds != 30 || a.RunbookURL != "https://wiki.example.com/runbooks/api" ||
		len(a.RecentChecks) != 5 || a.RecentChecks[0] != "up" || a.RecentChecks[4] != "down" || !strings.HasPrefix(a.StatusText, "Down ") || a.StatusDetail != "Timeout, 10,000 ms" {
		t.Fatalf("api-https row = %+v", a)
	}
	if byName["pg-tcp"].Target != "TCP db-02.example.com:5432" || !strings.HasPrefix(byName["web-https"].StatusText, "Up ") {
		t.Fatalf("tcp/web rows = %+v / %+v", byName["pg-tcp"], byName["web-https"])
	}
	if p := byName["old-check"]; !p.Paused || p.StatusText != "Paused" || p.PausedAt == nil {
		t.Fatalf("paused row = %+v", p)
	}
	if l := get("?status=down,degraded"); l.Data.Count != 1 || l.Data.Total != 4 || l.Data.HiddenCount != 3 || l.Data.Monitors[0].Name != "api-https" {
		t.Fatalf("status filter = %+v", l.Data)
	}
	if l := get("?paused=true"); l.Data.Count != 1 || l.Data.Monitors[0].Name != "old-check" {
		t.Fatalf("paused filter = %+v", l.Data)
	}
	if l := get("?paused=false"); l.Data.Count != 3 {
		t.Fatalf("running filter = %d", l.Data.Count)
	}
	if l := get("?q=pg-"); l.Data.Count != 1 || l.Data.Monitors[0].Name != "pg-tcp" {
		t.Fatalf("q filter = %+v", l.Data)
	}
	if l := get("?agent_id=nope"); l.Data.Count != 0 {
		t.Fatalf("agent filter = %d", l.Data.Count)
	}
	var sum struct {
		Data service.MonitorSummary `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/monitors/summary", nil, ""), &sum)
	if sum.Data.Total != 4 || sum.Data.Paused != 1 {
		t.Fatalf("summary = %+v", sum.Data)
	}
	if r := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+down.Data.Monitor.ID, gin.H{"runbook_url": "javascript:alert(1)"}, ""); r.Code != http.StatusBadRequest {
		t.Fatalf("bad runbook = %d", r.Code)
	}
}

func TestMonitorDetailSummaryResponseTimesAvailabilityAndHistoryRows(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	hb := createHTTPMonitor(t, server, "nightly-backup", gin.H{"kind": "heartbeat", "interval_seconds": 86400, "config": gin.H{"grace_seconds": 600}})
	if hb.Data.Config.HeartbeatToken == "" {
		t.Fatal("no heartbeat token")
	}
	m := createHTTPMonitor(t, server, "api-https", nil)
	id := m.Data.Monitor.ID
	server.db.Model(&db.Monitor{}).Where("id = ?", id).Updates(map[string]any{"reporting_interval_seconds": 3600})
	addChecks(server, id, "up", "up", "down", "down")
	server.db.Create(&db.Incident{ID: "i1", Number: 1, Status: "open", Severity: "high", Title: "api down", AgentID: agent.Data.AgentID, MonitorID: id, OpenedAt: time.Now().UTC().Add(-30 * time.Minute), LastEventAt: time.Now().UTC(), Source: "monitor"})

	var detail struct {
		Data struct {
			Summary MonitorDetailSummary    `json:"summary"`
			Reports []MonitorReportResponse `json:"recent_reports"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+id, nil, ""), &detail)
	s := detail.Data.Summary
	if s.Target != "GET example.com/health" || s.Incidents30d != 1 || s.LastResult != "fail" || s.CurrentStateSince == nil || s.LastCheckedAt == nil || s.IntervalSeconds != 30 || s.TimeoutSeconds != 10 {
		t.Fatalf("summary = %+v", s)
	}
	var hbDetail struct {
		Data struct {
			Summary MonitorDetailSummary `json:"summary"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+hb.Data.Monitor.ID, nil, ""), &hbDetail)
	h := hbDetail.Data.Summary.Heartbeat
	if h == nil || h.GraceSeconds != 600 || h.ExpectedEverySecond != 86400 || !strings.Contains(h.PingURLSuccess, "/v1/heartbeats/") || !strings.Contains(h.PingURLSuccess, "...") || strings.Contains(h.PingURLSuccess, hb.Data.Config.HeartbeatToken) {
		t.Fatalf("heartbeat info = %+v", h)
	}
	foundFail := false
	for _, r := range detail.Data.Reports {
		if !r.Passed && r.Summary == "Timeout, 10,000 ms" && r.ReportedBy == "Core worker" && r.DurationMs != nil && *r.DurationMs == 10000 {
			foundFail = true
		}
	}
	if !foundFail {
		t.Fatalf("report rows lack derived fields: %+v", detail.Data.Reports)
	}

	var rt struct {
		Data ResponseTimesResponse `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+id+"/response-times?range=week", nil, ""), &rt)
	if rt.Data.Range != "week" || rt.Data.BucketSeconds != 3600 || rt.Data.TimeoutMs != 10000 || len(rt.Data.Points) == 0 || rt.Data.Points[0].MaxMs == nil {
		t.Fatalf("response times = %+v", rt.Data)
	}
	var av struct {
		Data struct {
			Periods []AvailabilityPeriod `json:"periods"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+id+"/availability", nil, ""), &av)
	if len(av.Data.Periods) != 4 || av.Data.Periods[0].Period != "today" || av.Data.Periods[1].Incidents != 1 || !av.Data.Periods[1].LongestOngoing || av.Data.Periods[1].DowntimeSeconds < 1700 {
		t.Fatalf("availability = %+v", av.Data.Periods)
	}
	if av.Data.Periods[3].AvailabilityPct == nil || *av.Data.Periods[3].AvailabilityPct != 50 {
		t.Fatalf("365d availability = %+v (want 2 of 4 checks up)", av.Data.Periods[3])
	}
	if r := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/nope/availability", nil, ""); r.Code != http.StatusNotFound {
		t.Fatalf("unknown monitor = %d", r.Code)
	}
}

func TestMonitorCloneAndDeleteWithOrWithoutHistory(t *testing.T) {
	server := setupTestServer(t)
	registerTestAgent(t, server)
	m := createHTTPMonitor(t, server, "api-https", gin.H{"runbook_url": "https://wiki.example.com/r"})
	id := m.Data.Monitor.ID
	addChecks(server, id, "up", "down")

	var cloned createdMonitor
	r := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+id+"/clone", nil, "")
	decodeResponse(t, r, &cloned)
	if r.Code != http.StatusCreated || cloned.Data.Monitor.Name != "api-https copy" || cloned.Data.Monitor.RunbookURL != "https://wiki.example.com/r" || cloned.Data.Monitor.ID == id {
		t.Fatalf("clone = %d %+v", r.Code, cloned.Data.Monitor)
	}
	var again createdMonitor
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+id+"/clone", nil, ""), &again)
	if again.Data.Monitor.Name != "api-https copy 2" {
		t.Fatalf("second clone name = %q", again.Data.Monitor.Name)
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/nope/clone", nil, ""); r.Code != http.StatusNotFound {
		t.Fatalf("clone unknown = %d", r.Code)
	}

	// Default delete removes history and keeps incidents.
	server.db.Create(&db.Incident{ID: "i1", Number: 1, Status: "open", Severity: "low", Title: "x", AgentID: "a", MonitorID: id, OpenedAt: time.Now().UTC(), LastEventAt: time.Now().UTC(), Source: "monitor"})
	if r := performJSONRequest(t, server, http.MethodDelete, "/v1/monitors/"+id+"?keep_history=false", nil, ""); r.Code != http.StatusOK {
		t.Fatalf("delete = %d", r.Code)
	}
	var reports, incidents, monitors int64
	server.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", id).Count(&reports)
	server.db.Model(&db.Incident{}).Where("id = ?", "i1").Count(&incidents)
	server.db.Model(&db.Monitor{}).Where("id = ?", id).Count(&monitors)
	if reports != 0 || incidents != 1 || monitors != 0 {
		t.Fatalf("after default delete reports=%d incidents=%d monitors=%d", reports, incidents, monitors)
	}
	var closed db.Incident
	server.db.First(&closed, "id = ?", "i1")
	if closed.Status != "resolved" || closed.ResolutionKind != "monitor_removed" {
		t.Fatalf("incident of a deleted monitor = %+v", closed)
	}

	// keep_history=true soft-deletes; retention purges it after 90 days.
	cid := cloned.Data.Monitor.ID
	addChecks(server, cid, "up")
	if r := performJSONRequest(t, server, http.MethodDelete, "/v1/monitors/"+cid+"?keep_history=true", nil, ""); r.Code != http.StatusOK {
		t.Fatalf("delete keep = %d", r.Code)
	}
	server.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", cid).Count(&reports)
	if reports != 1 {
		t.Fatalf("kept reports = %d", reports)
	}
	server.db.Model(&db.Monitor{}).Where("id = ?", cid).Update("deleted_at", time.Now().UTC().AddDate(0, 0, -91))
	if n, err := service.PurgeDeletedMonitors(server.db, server.logger, time.Now().UTC()); err != nil || n != 1 {
		t.Fatalf("purge = %d, %v", n, err)
	}
	server.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", cid).Count(&reports)
	if reports != 0 {
		t.Fatalf("reports after purge = %d", reports)
	}
}

// Review P1: list enrichment must not scan a monitor's whole history.
func TestMonitorListStaysFastWithLargeHistory(t *testing.T) {
	server := setupTestServer(t)
	registerTestAgent(t, server)
	ids := []string{}
	for i := 0; i < 5; i++ {
		m := createHTTPMonitor(t, server, fmt.Sprintf("big-%d", i), nil)
		ids = append(ids, m.Data.Monitor.ID)
	}
	for _, id := range ids {
		// 100k always-up reports per monitor via a recursive CTE (fast, no per-row Go work).
		if err := server.db.Exec(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 100000)
			INSERT INTO monitor_reports (id, monitor_id, payload, collected_at, health, created_at)
			SELECT ? || '-' || i, ?, '{"runner":"core","duration_ms":50}', '', 'up', datetime('now', '-' || (i * 30) || ' seconds') FROM n`, id, id).Error; err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("list = %d", resp.Code)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("list over 500k reports took %v, want index-bounded work", elapsed)
	}
}
