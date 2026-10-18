package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func seedStatusFixtures(t *testing.T, server *Server) (now time.Time) {
	t.Helper()
	now = time.Now().UTC().Truncate(time.Second)
	min := func(n int) time.Time { return now.Add(-time.Duration(n) * time.Minute) }
	check := func(s ...any) {
		for _, row := range s {
			if err := server.db.Create(row).Error; err != nil {
				t.Fatalf("create %T: %v", row, err)
			}
		}
	}
	check(&db.Agent{ID: "ag1", MachineId: "m1", Name: "edge-01", OS: "linux", Arch: "amd64", Token: "t1", CreatedAt: min(600), LastSeen: now},
		&db.Agent{ID: "ag2", MachineId: "m2", Name: "db-02", OS: "linux", Arch: "amd64", Token: "t2", CreatedAt: min(600), LastSeen: now})
	check(&db.Monitor{ID: "mon1", Name: "api-https", Type: "http", AgentID: "ag1", Lifecycle: "active", Health: "down", CreatedAt: min(500)},
		&db.Monitor{ID: "mon2", Name: "pg-tcp", Type: "tcp", AgentID: "ag2", Lifecycle: "active", Health: "up", CreatedAt: min(500)})
	report := func(id, monitor, health, payload string, ago int) *db.MonitorReport {
		return &db.MonitorReport{ID: id, MonitorID: monitor, Health: health, Payload: payload, CollectedAt: min(ago).Format(time.RFC3339), CreatedAt: min(ago)}
	}
	check(report("r1", "mon1", "up", `{"duration_ms":201,"status_code":200}`, 60), report("r2", "mon1", "down", `{"error":"context deadline exceeded","duration_ms":10000}`, 30),
		report("r3", "mon1", "down", `{"error":"context deadline exceeded","duration_ms":10000}`, 20), report("r4", "mon2", "up", `{"duration_ms":12}`, 25), report("r5", "mon1", "up", `{"duration_ms":180,"status_code":200}`, 5))
	check(&db.Incident{ID: "inc1", Number: 231, Status: "open", Severity: "high", Title: "api-https unreachable", AgentID: "ag1", MonitorID: "mon1", OpenedAt: min(29), LastEventAt: min(29), CreatedAt: min(29)})
	check(&db.IncidentEvent{ID: "ie1", IncidentID: "inc1", Type: "incident_opened", Message: "3 failed checks in a row", CreatedAt: min(29)},
		&db.IncidentEvent{ID: "ie2", IncidentID: "inc1", Type: "incident_note", Message: "Note added", Note: "Restarting the pool", ActorID: "admin", ActorType: "user", CreatedAt: min(10)})
	check(&db.AlertDelivery{ID: "dl1", IncidentID: "inc1", EventType: "incident_opened", Channel: "oncall webhook", Type: "webhook", Status: "failed", AttemptCount: 3, MaxAttempts: 3, CreatedAt: min(28)},
		&db.AlertDelivery{ID: "dl2", IncidentID: "inc1", EventType: "incident_opened", Channel: "ops-team", Type: "email", Status: "sent", AttemptCount: 1, MaxAttempts: 8, CreatedAt: min(27)})
	check(&db.AlertDeliveryAttempt{ID: "at1", AlertDeliveryID: "dl1", AttemptNumber: 3, Status: "failed", Stage: "http_response", HTTPStatus: 502, StartedAt: min(28), CompletedAt: ptrTime(min(28).Add(2 * time.Second))},
		&db.AlertDeliveryAttempt{ID: "at2", AlertDeliveryID: "dl2", AttemptNumber: 1, Status: "sent", Stage: "transport", StartedAt: min(27), CompletedAt: ptrTime(min(27).Add(900 * time.Millisecond))})
	check(&db.AuditEvent{ID: "au1", Action: "agent_token_rotated", AffectedObjectType: "agent", AffectedObjectID: "ag1", ActorType: "user", ActorID: "admin", CreatedAt: min(5)})
	check(&db.AgentReport{ID: "ar1", AgentID: "ag1", CreatedAt: min(2), AgentVersion: "0.9"})
	return now
}

func statusList(t *testing.T, server *Server, query string) StatusLogListResponse {
	t.Helper()
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/status-logs?"+query, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("status-logs %s = %d %s", query, resp.Code, resp.Body.String())
	}
	var out struct {
		Data StatusLogListResponse `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Data
}

func TestStatusLogsListFiltersTitlesAndPaging(t *testing.T) {
	server := setupTestServer(t)
	seedStatusFixtures(t, server)
	all := statusList(t, server, "limit=200")
	titles := map[string]StatusLogEntry{}
	for _, e := range all.Entries {
		titles[e.Title] = e
	}
	want := map[string]string{
		"Check passed":          "200 OK, 201 ms",
		"Went down":             "Timeout, 10,000 ms",
		"Recovered":             "200 OK, 180 ms",
		"Delivery failed":       "oncall webhook returned 502, attempt 3 of 3",
		"Delivered to ops-team": "email, 0.9 s",
		"INC-0231 opened":       "api-https unreachable",
		"Note on INC-0231":      "Restarting the pool",
		"Report accepted":       "",
		"Agent token rotated":   "admin",
		"Server registered":     "edge-01",
		"Monitor created":       "api-https",
		"Check failed":          "Timeout, 10,000 ms",
	}
	for title, detail := range want {
		e, ok := titles[title]
		if !ok {
			t.Errorf("missing %q in %v", title, keysOf(titles))
			continue
		}
		if title != "Report accepted" && e.Detail != detail {
			t.Errorf("%s detail = %q, want %q", title, e.Detail, detail)
		}
	}
	if e := titles["Went down"]; e.Level != "error" || e.Type != "monitor" || e.ServerName != "edge-01" || e.MonitorName != "api-https" {
		t.Errorf("went down = %+v", e)
	}
	if e := titles["Delivery failed"]; e.Level != "error" || e.Type != "alert" || e.IncidentDisplayID != "INC-0231" || e.MonitorName != "api-https" {
		t.Errorf("delivery = %+v", e)
	}
	if all.Total != int64(len(all.Entries)) {
		t.Errorf("total %d != entries %d", all.Total, len(all.Entries))
	}
	for i := 1; i < len(all.Entries); i++ {
		if all.Entries[i].At.After(all.Entries[i-1].At) {
			t.Fatalf("not newest first at %d", i)
		}
	}

	if got := statusList(t, server, "level=error&type=monitor"); len(got.Entries) == 0 || got.Total != int64(len(got.Entries)) {
		t.Fatalf("level+type = %+v", got)
	} else {
		for _, e := range got.Entries {
			if e.Level != "error" || e.Type != "monitor" {
				t.Errorf("filter leaked %+v", e)
			}
		}
	}
	if got := statusList(t, server, "server_id=ag2&limit=200"); got.Total == 0 {
		t.Fatal("server filter returned nothing")
	} else {
		for _, e := range got.Entries {
			if e.ServerID != "ag2" {
				t.Errorf("server filter leaked %+v", e)
			}
		}
	}
	if got := statusList(t, server, "incident_id=inc1&limit=200"); got.Total != 4 {
		t.Errorf("incident filter total = %d, want 4 (2 events, 2 deliveries)", got.Total)
	}
	if got := statusList(t, server, "q=oncall"); got.Total != 1 || got.Entries[0].Title != "Delivery failed" {
		t.Errorf("text search = %+v", got)
	}
	if got := statusList(t, server, "monitor_id=mon1&from=-45m&to=-15m"); got.Total == 0 {
		t.Error("time range returned nothing")
	}

	// Older, then newer: the cursor round trip lands on the same rows.
	page1 := statusList(t, server, "limit=5")
	if len(page1.Entries) != 5 || page1.NextCursor == "" || page1.PrevCursor != "" {
		t.Fatalf("page1 = %d next=%q prev=%q", len(page1.Entries), page1.NextCursor, page1.PrevCursor)
	}
	page2 := statusList(t, server, "limit=5&cursor="+page1.NextCursor)
	if len(page2.Entries) != 5 || page2.PrevCursor == "" || page2.Entries[0].ID == page1.Entries[0].ID {
		t.Fatalf("page2 = %+v", page2)
	}
	back := statusList(t, server, "limit=5&direction=newer&cursor="+page2.PrevCursor)
	if len(back.Entries) != 5 || back.Entries[0].ID != page1.Entries[0].ID || back.Entries[4].ID != page1.Entries[4].ID {
		t.Fatalf("newer page differs from page1: %v vs %v", ids(back.Entries), ids(page1.Entries))
	}
	for name, q := range map[string]string{"level": "level=fatal", "type": "type=nope", "time": "from=yesterday"} {
		if r := performJSONRequest(t, server, http.MethodGet, "/v1/status-logs?"+q, nil, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", name, r.Code)
		}
	}
}

func ids(entries []StatusLogEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}

func keysOf(m map[string]StatusLogEntry) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestStatusLogsFacetsHistogramPatternsAndExport(t *testing.T) {
	server := setupTestServer(t)
	server.cfg.DataDir = t.TempDir()
	seedStatusFixtures(t, server)

	var facets struct {
		Data StatusLogFacetsResponse `json:"data"`
	}
	json.Unmarshal(performJSONRequest(t, server, http.MethodGet, "/v1/status-logs/facets?top=1", nil, "").Body.Bytes(), &facets)
	f := facets.Data
	if f.Levels["error"] == 0 || f.Levels["info"] == 0 || f.Types["monitor"] == 0 || f.Types["alert"] != 2 || f.Types["incident"] != 2 || f.Total == 0 {
		t.Fatalf("facets = %+v", f)
	}
	if len(f.Servers) != 1 || f.ServersDistinct != 2 || len(f.Monitors) != 1 || f.MonitorsDistinct != 2 || f.Servers[0].Label == "" {
		t.Fatalf("server/monitor facets = %+v", f)
	}

	var scoped struct {
		Data StatusLogFacetsResponse `json:"data"`
	}
	json.Unmarshal(performJSONRequest(t, server, http.MethodGet, "/v1/status-logs/facets?from=-2h", nil, "").Body.Bytes(), &scoped)
	var hist struct {
		Data LogHistogramResponse `json:"data"`
	}
	json.Unmarshal(performJSONRequest(t, server, http.MethodGet, "/v1/status-logs/histogram?from=-2h&buckets=24", nil, "").Body.Bytes(), &hist)
	var total int64
	for _, b := range hist.Data.Buckets {
		total += b.Total
	}
	if hist.Data.BucketSeconds < 60 || len(hist.Data.Buckets) < 12 || total != scoped.Data.Total {
		t.Fatalf("histogram width=%d buckets=%d total=%d want %d", hist.Data.BucketSeconds, len(hist.Data.Buckets), total, scoped.Data.Total)
	}

	var patterns struct {
		Data LogPatternsResponse `json:"data"`
	}
	json.Unmarshal(performJSONRequest(t, server, http.MethodGet, "/v1/status-logs/patterns", nil, "").Body.Bytes(), &patterns)
	if len(patterns.Data.Patterns) == 0 || patterns.Data.Patterns[0].Count < 1 || len(patterns.Data.Patterns[0].Trend) != 12 {
		t.Fatalf("patterns = %+v", patterns.Data)
	}

	r := performJSONRequest(t, server, http.MethodPost, "/v1/logs/export", gin.H{"kind": "status", "format": "csv", "query": "level=error"}, "")
	var created struct {
		Data struct {
			Export ExportResponse `json:"export"`
		} `json:"data"`
	}
	json.Unmarshal(r.Body.Bytes(), &created)
	if r.Code != http.StatusAccepted {
		t.Fatalf("export = %d %s", r.Code, r.Body.String())
	}
	var done ExportResponse
	for i := 0; i < 100; i++ {
		var got struct {
			Data struct {
				Export ExportResponse `json:"export"`
			} `json:"data"`
		}
		json.Unmarshal(performJSONRequest(t, server, http.MethodGet, "/v1/exports/"+created.Data.Export.ID, nil, "").Body.Bytes(), &got)
		if done = got.Data.Export; done.Status == "done" || done.Status == "failed" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	body := performJSONRequest(t, server, http.MethodGet, done.DownloadURL, nil, "").Body.String()
	if done.Status != "done" || !strings.HasPrefix(body, "timestamp,level,type,server,monitor,title,detail\n") || !strings.Contains(body, "Delivery failed") || strings.Contains(body, "Delivered to") {
		t.Fatalf("status export = %+v\n%s", done, body)
	}
}

func TestStatusLogsStreamEmitsNewEvents(t *testing.T) {
	server := setupTestServer(t)
	seedStatusFixtures(t, server)
	srv := httptest.NewServer(server.router)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/status-logs/stream?level=error", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	time.Sleep(300 * time.Millisecond)
	server.db.Create(&db.IncidentEvent{ID: "ie-live", IncidentID: "inc1", Type: "incident_reopened", Message: "back again", CreatedAt: time.Now().UTC().Add(time.Second)})
	buf := make([]byte, 4096)
	var seen string
	for !strings.Contains(seen, "INC-0231 reopened") {
		n, err := resp.Body.Read(buf)
		seen += string(buf[:n])
		if err != nil {
			break
		}
	}
	if !strings.Contains(seen, "INC-0231 reopened") || !strings.Contains(seen, "data:") {
		t.Fatalf("stream = %q", seen)
	}
}

func TestInternalLogsAreCapturedSeparately(t *testing.T) {
	server := setupLogsServer(t, nil)
	logger := utils.NewLoggerWithWriter(&strings.Builder{})
	logger.AttachSink(logstore.NewCoreLogSink(server.logs.store, "api"))
	logger.Info("Alert delivery retry failed", "channel", "oncall", "token", "abc123")
	logger.Warn("Core worker flagged silent monitors as stale", "count", 3)
	logger.Info("HTTP request", "method", "GET", "route", "/v1/overview", "status", 200, "duration_ms", 12)
	logger.Error("HTTP request", "method", "POST", "route", "/v1/agents/x/report", "status", 500, "duration_ms", 30)
	logger.Info("Dropped a batch of log entries")
	appID, key := createAppWithKey(t, server, "web-store")
	ingest(server, key, "application/json", `{"body":"product line"}`)
	server.logs.store.Flush(5 * time.Second)

	internal := getData[LogListResponse](t, server, "/v1/internal-logs?limit=50")
	got := map[string]LogEntry{}
	for _, e := range internal.Entries {
		got[e.Body+"|"+string(e.Severity)] = e
		if e.Source != "internal" || e.AppName != "Orion Core" {
			t.Errorf("internal entry shape = %+v", e)
		}
	}
	retry := got["Alert delivery retry failed|info"]
	if retry.Attributes["component"] != "worker" || retry.Attributes["token"] != "[redacted]" || retry.Attributes["channel"] != "oncall" {
		t.Errorf("retry line = %+v", retry)
	}
	if got["Core worker flagged silent monitors as stale|warn"].Attributes["component"] != "worker" {
		t.Errorf("worker line = %+v", got)
	}
	if e := got["HTTP request|error"]; e.Attributes["component"] != "agent gateway" {
		t.Errorf("gateway line = %+v", e)
	}
	if len(internal.Entries) != 3 {
		t.Errorf("expected 3 internal lines (no routine request, no store noise, no product log), got %d: %v", len(internal.Entries), internal.Entries)
	}
	facets := getData[LogFacetsResponse](t, server, "/v1/internal-logs/facets")
	if len(facets.Values) < 2 || facets.Total != 3 {
		t.Errorf("internal facets = %+v", facets)
	}
	// Product searches never show Core's own lines.
	product := searchLogsAPI(t, server, "from=-1h")
	if len(product.Data.Entries) != 1 || product.Data.Entries[0].AppID != appID {
		t.Errorf("product search leaked internal lines: %+v", product.Data.Entries)
	}
	if hist := getData[LogHistogramResponse](t, server, "/v1/internal-logs/histogram?from=-1h"); len(hist.Buckets) == 0 {
		t.Error("internal histogram is empty")
	}
}

func TestStatusLogStreamPollsAreTimeBoundedAndRangeIsCapped(t *testing.T) {
	now := time.Now().UTC()
	last := now.Add(-3 * time.Second).UnixMilli()
	f := streamFilter(statusFilter{Levels: []string{"error"}}, last)
	if f.From.IsZero() || !f.To.IsZero() || now.Sub(f.From) > time.Minute {
		t.Fatalf("stream filter = %+v, want a From close to the resume point", f)
	}
	union, args := f.union()
	// Every time-based branch carries the lower bound: seven range clauses plus the state-change window.
	if got := strings.Count(union, "created_at >= ?"); got < 7 || len(args) < 8 {
		t.Fatalf("stream union has %d bounded branches and %d args:\n%s", got, len(args), union)
	}
	unbounded, _ := statusFilter{}.union()
	if strings.Contains(unbounded, "created_at >= ?") {
		t.Fatal("sanity check: an empty filter should not carry bounds")
	}
	if _, err := parseStatusFilter(url.Values{"from": {"-45d"}}, now); err == nil {
		t.Error("a 45 day range must be refused")
	}
	if _, err := parseStatusFilter(url.Values{"from": {"-29d"}}, now); err != nil {
		t.Errorf("a 29 day range is fine: %v", err)
	}
}

func TestInternalSinkScrubsCredentialsInText(t *testing.T) {
	server := setupLogsServer(t, nil)
	logger := utils.NewLoggerWithWriter(&strings.Builder{})
	logger.AttachSink(logstore.NewCoreLogSink(server.logs.store, "worker"))
	logger.Error("Alert delivery retry failed", "error", `Post "https://user:hunter2@pager.example.com/hook?token=abc123&x=1": EOF`, "header", "Authorization: Bearer abcdefghijklmnop")
	logger.Error("webhook https://pager.example.com/hook?secret=zzz failed")
	server.logs.store.Flush(5 * time.Second)
	list := getData[LogListResponse](t, server, "/v1/internal-logs?limit=10")
	var all string
	for _, e := range list.Entries {
		raw, _ := json.Marshal(e)
		all += string(raw)
	}
	for _, leaked := range []string{"hunter2", "abc123", "abcdefghijklmnop", "secret=zzz"} {
		if strings.Contains(all, leaked) {
			t.Errorf("internal logs leaked %q: %s", leaked, all)
		}
	}
	if !strings.Contains(all, "[redacted]") || !strings.Contains(all, "pager.example.com") {
		t.Errorf("scrubbing should keep the host and mark removals: %s", all)
	}
}
