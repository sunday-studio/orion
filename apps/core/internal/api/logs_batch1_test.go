package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/logstore"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func ndjson(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

func getData[T any](t *testing.T, server *Server, path string) T {
	t.Helper()
	server.logs.store.Flush(5 * time.Second)
	resp := performJSONRequest(t, server, http.MethodGet, path, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %s", path, resp.Code, resp.Body.String())
	}
	var out struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Data
}

func TestFacetsPatternsRelatedAndInferredSeverity(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, key := createAppWithKey(t, server, "billing-api")
	body := ndjson(
		`{"severity":"error","event":"payment.failed","body":"upstream returned 502 for order 8841","request_id":"req_1","user":{"id":"u_41"}}`,
		`{"severity":"error","event":"payment.failed","body":"upstream returned 504 for order 9120","request_id":"req_2","user":{"id":"u_41"}}`,
		`{"severity":"info","event":"payment.started","body":"charging order 8841","request_id":"req_1","user":{"id":"u_77"}}`,
		`{"severity":"info","event":"invoice.created","body":"invoice inv_4411 created","request_id":"req_1"}`,
		`{"body":"ERROR upload failed: connection reset by peer"}`,
		`{"body":"Starting backup job nightly-sync"}`,
	)
	if r := ingest(server, key, "application/x-ndjson", body); r.Code != http.StatusAccepted {
		t.Fatalf("ingest = %d %s", r.Code, r.Body.String())
	}

	facets := getData[LogFacetsResponse](t, server, "/v1/logs/facets?app_id="+appID+"&from=-1h&field=user.id")
	if facets.Total != 6 || facets.Severity["error"] != 3 || facets.Severity["info"] != 3 {
		t.Fatalf("severity facets = %+v", facets)
	}
	if len(facets.Sources) != 1 || facets.Sources[0].Label != "billing-api" || facets.Sources[0].Count != 6 {
		t.Fatalf("source facets = %+v", facets.Sources)
	}
	if len(facets.Events) == 0 || facets.Events[0].Value != "payment.failed" || facets.Events[0].Count != 2 || facets.EventsDistinct != 3 {
		t.Fatalf("event facets = %+v distinct=%d", facets.Events, facets.EventsDistinct)
	}
	if len(facets.Values) != 2 || facets.Values[0].Value != "u_41" || facets.Values[0].Count != 2 {
		t.Fatalf("field values = %+v", facets.Values)
	}
	if len(facets.Fields) == 0 {
		t.Fatalf("expected the field catalog for a single app")
	}
	if errOnly := getData[LogFacetsResponse](t, server, "/v1/logs/facets?app_id="+appID+"&severity=error"); errOnly.Total != 3 {
		t.Fatalf("filtered facets total = %d, want 3", errOnly.Total)
	}

	patterns := getData[LogPatternsResponse](t, server, "/v1/logs/patterns?app_id="+appID+"&from=-1h")
	var found *LogPattern
	for i := range patterns.Patterns {
		if strings.HasPrefix(patterns.Patterns[i].Pattern, "payment.failed: upstream returned <n> for order <n>") {
			found = &patterns.Patterns[i]
		}
	}
	if found == nil || found.Count != 2 || found.Severity != "error" || len(found.Trend) != 12 || found.SampleID == "" {
		t.Fatalf("patterns = %+v", patterns.Patterns)
	}

	list := searchLogsAPI(t, server, "app_id="+appID+"&limit=50")
	byBody := map[string]string{}
	for _, e := range list.Data.Entries {
		byBody[e.Body] = e.ID
	}
	related := getData[LogRelatedResponse](t, server, "/v1/logs/"+byBody["charging order 8841"]+"/related")
	if related.By != "request_id" || related.Value != "req_1" || related.Total != 3 || len(related.Entries) != 3 {
		t.Fatalf("related = %+v", related)
	}
	nearby := getData[LogRelatedResponse](t, server, "/v1/logs/"+byBody["ERROR upload failed: connection reset by peer"]+"/related")
	if nearby.By != "nearby" || len(nearby.Entries) < 2 {
		t.Fatalf("nearby = %+v", nearby)
	}

	type entryEnvelope struct {
		Entry LogEntry `json:"entry"`
	}
	inferred := getData[entryEnvelope](t, server, "/v1/logs/"+byBody["ERROR upload failed: connection reset by peer"])
	plain := getData[entryEnvelope](t, server, "/v1/logs/"+byBody["Starting backup job nightly-sync"])
	if inferred.Entry.Severity != "error" || !inferred.Entry.SeverityInferred || plain.Entry.Severity != "info" || plain.Entry.SeverityInferred {
		t.Fatalf("inferred=%+v plain=%+v", inferred.Entry, plain.Entry)
	}
}

func TestIngestStatusAppStatsAndRateLimit(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, key := createAppWithKey(t, server, "web-store")
	type apps struct {
		Apps []App `json:"apps"`
	}
	if got := getData[apps](t, server, "/v1/apps"); got.Apps[0].Status != "never" || got.Apps[0].RatePerSecond != 0 {
		t.Fatalf("before ingest: %+v", got.Apps[0])
	}
	if r := performJSONRequest(t, server, http.MethodPatch, "/v1/apps/"+appID, gin.H{"max_storage_mb": 5, "rate_limit_per_second": 2}, ""); r.Code != http.StatusOK {
		t.Fatalf("patch = %d %s", r.Code, r.Body.String())
	}
	ingest(server, key, "application/json", `{"body":"hello"}`)
	got := getData[apps](t, server, "/v1/apps")
	app := got.Apps[0]
	if app.Status != "receiving" || app.Lines != 1 || app.StorageCapBytes != 5<<20 || app.RateLimitPerSecond != 2 || app.RatePerSecond <= 0 || len(app.Redaction.DefaultKeys) == 0 {
		t.Fatalf("after ingest: %+v", app)
	}
	// 2 lines per second means a burst of 4; a 10 line request is refused with 429 and flips the banner.
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, `{"body":"burst"}`)
	}
	if r := ingest(server, key, "application/x-ndjson", ndjson(lines...)); r.Code != http.StatusTooManyRequests {
		t.Fatalf("burst = %d, want 429", r.Code)
	}
	status := getData[LogIngestStatus](t, server, "/v1/logs/ingest-status")
	if !status.RateLimited || status.LinesToday < 1 || status.StorageLimitBytes != 5<<20 || status.QueueMax == 0 {
		t.Fatalf("status = %+v", status)
	}
}

func TestKeyRotationRedactionRulesPurgeAndTest(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, key := createAppWithKey(t, server, "billing-api")

	var keys struct {
		Data struct {
			Keys []IngestKey `json:"keys"`
		} `json:"data"`
	}
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/apps/"+appID+"/keys", nil, "")
	json.Unmarshal(resp.Body.Bytes(), &keys)
	rot := performJSONRequest(t, server, http.MethodPost, "/v1/apps/"+appID+"/keys/"+keys.Data.Keys[0].ID+"/rotate", nil, "")
	var rotated struct {
		Data struct {
			Key   IngestKeyCreated `json:"key"`
			Until time.Time        `json:"previous_key_valid_until"`
		} `json:"data"`
	}
	json.Unmarshal(rot.Body.Bytes(), &rotated)
	if rot.Code != http.StatusCreated || rotated.Data.Key.Key == "" || rotated.Data.Key.Key == key || time.Until(rotated.Data.Until) < 9*time.Minute {
		t.Fatalf("rotate = %d %s", rot.Code, rot.Body.String())
	}
	for name, k := range map[string]string{"old key during grace": key, "new key": rotated.Data.Key.Key} {
		if r := ingest(server, k, "application/json", `{"body":"hello"}`); r.Code != http.StatusAccepted {
			t.Errorf("%s = %d", name, r.Code)
		}
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/apps/"+appID+"/keys/"+keys.Data.Keys[0].ID+"/rotate", nil, ""); r.Code != http.StatusNotFound {
		t.Errorf("rotating an already rotated key = %d, want 404", r.Code)
	}
	// A leaked key can be killed at once, even inside the rotation grace period.
	if r := performJSONRequest(t, server, http.MethodDelete, "/v1/apps/"+appID+"/keys/"+keys.Data.Keys[0].ID, nil, ""); r.Code != http.StatusOK {
		t.Errorf("revoking a rotated key = %d, want 200", r.Code)
	}
	if r := ingest(server, key, "application/json", `{"body":"hello"}`); r.Code != http.StatusUnauthorized {
		t.Errorf("revoked old key ingest = %d, want 401", r.Code)
	}

	rules := gin.H{"redaction": gin.H{"rules": []gin.H{{"type": "key", "pattern": "card_number", "replace_with": "[hidden]"}, {"type": "regex", "pattern": `\b\d{16}\b`, "replace_with": "[card]"}}}}
	if r := performJSONRequest(t, server, http.MethodPatch, "/v1/apps/"+appID, rules, ""); r.Code != http.StatusOK {
		t.Fatalf("patch rules = %d %s", r.Code, r.Body.String())
	}
	for name, bad := range map[string]gin.H{
		"bad regex": {"redaction": gin.H{"rules": []gin.H{{"type": "regex", "pattern": "(unclosed"}}}},
		"bad type":  {"redaction": gin.H{"rules": []gin.H{{"type": "glob", "pattern": "x"}}}},
		"empty":     {"redaction": gin.H{"rules": []gin.H{{"type": "key", "pattern": ""}}}},
	} {
		if r := performJSONRequest(t, server, http.MethodPatch, "/v1/apps/"+appID, bad, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", name, r.Code)
		}
	}
	ingest(server, rotated.Data.Key.Key, "application/json", `{"body":"paid with 4242424242424242","attributes":{"user":"u_41","card_number":"4242424242424242","token":"abc123"}}`)
	list := searchLogsAPI(t, server, "app_id="+appID+"&q=paid")
	if len(list.Data.Entries) != 1 || list.Data.Entries[0].Body != "paid with [card]" || list.Data.Entries[0].Attributes["card_number"] != "[hidden]" || list.Data.Entries[0].Attributes["token"] != "[redacted]" {
		t.Fatalf("redacted entry = %+v", list.Data.Entries)
	}

	var tested struct {
		Data RedactionTestResponse `json:"data"`
	}
	r := performJSONRequest(t, server, http.MethodPost, "/v1/apps/"+appID+"/redaction/test", gin.H{"sample": gin.H{"user": "u_41", "card_number": "4242424242424242", "token": "abc123", "note": "card 1111222233334444"},
		"redaction": gin.H{"rules": []gin.H{{"type": "key", "pattern": "card_number", "replace_with": "[redacted]"}, {"type": "regex", "pattern": `\b\d{16}\b`, "replace_with": "[card]"}}}}, "")
	json.Unmarshal(r.Body.Bytes(), &tested)
	redacted, _ := tested.Data.Redacted.(map[string]interface{})
	if r.Code != http.StatusOK || !tested.Data.Changed || redacted["card_number"] != "[redacted]" || redacted["token"] != "[redacted]" || redacted["note"] != "card [card]" || redacted["user"] != "u_41" {
		t.Fatalf("redaction test = %d %s", r.Code, r.Body.String())
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/apps/"+appID+"/redaction/test", gin.H{}, ""); r.Code != http.StatusBadRequest {
		t.Errorf("empty sample = %d", r.Code)
	}

	purge := performJSONRequest(t, server, http.MethodPost, "/v1/apps/"+appID+"/purge", nil, "")
	if purge.Code != http.StatusOK || len(searchLogsAPI(t, server, "app_id="+appID).Data.Entries) != 0 {
		t.Fatalf("purge = %d %s", purge.Code, purge.Body.String())
	}
}

func TestLogExportCSVAndJSONLines(t *testing.T) {
	server := setupLogsServer(t, nil)
	server.cfg.DataDir = t.TempDir()
	appID, key := createAppWithKey(t, server, "billing-api")
	ingest(server, key, "application/x-ndjson", ndjson(
		`{"severity":"error","event":"payment.failed","body":"=HYPERLINK(\"http://evil\")","attributes":{"user":"u_41"}}`,
		`{"severity":"info","event":"invoice.created","body":"fine"}`,
	))
	server.logs.store.Flush(5 * time.Second)

	start := func(body gin.H) ExportResponse {
		r := performJSONRequest(t, server, http.MethodPost, "/v1/logs/export", body, "")
		var out struct {
			Data struct {
				Export ExportResponse `json:"export"`
			} `json:"data"`
		}
		json.Unmarshal(r.Body.Bytes(), &out)
		if r.Code != http.StatusAccepted {
			t.Fatalf("export = %d %s", r.Code, r.Body.String())
		}
		for i := 0; i < 200; i++ {
			// The in-memory test database can briefly lock while the export job writes; retry instead of failing.
			poll := performJSONRequest(t, server, http.MethodGet, "/v1/exports/"+out.Data.Export.ID, nil, "")
			var g struct {
				Data struct {
					Export ExportResponse `json:"export"`
				} `json:"data"`
			}
			if poll.Code == http.StatusOK && json.Unmarshal(poll.Body.Bytes(), &g) == nil && (g.Data.Export.Status == "done" || g.Data.Export.Status == "failed") {
				return g.Data.Export
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal("export did not finish")
		return ExportResponse{}
	}
	csvExport := start(gin.H{"format": "csv", "query": "app_id=" + appID + "&severity=error&from=-1h", "include_raw": true})
	if csvExport.Status != "done" || csvExport.Rows != 1 || csvExport.DownloadURL == "" || csvExport.Bytes == 0 {
		t.Fatalf("csv export = %+v", csvExport)
	}
	dl := performJSONRequest(t, server, http.MethodGet, csvExport.DownloadURL, nil, "")
	body := dl.Body.String()
	if dl.Code != http.StatusOK || !strings.HasPrefix(dl.Header().Get("Content-Type"), "text/csv") || !strings.HasPrefix(body, "timestamp,severity,source,event,message,request_id,trace_id,attributes,raw\n") ||
		!strings.Contains(body, "'=HYPERLINK") || strings.Contains(body, "\n=HYPERLINK") || !strings.Contains(body, "billing-api") {
		t.Fatalf("csv download = %d %s", dl.Code, body)
	}
	jsonl := start(gin.H{"format": "jsonl", "query": "app_id=" + appID + "&from=-1h"})
	if jsonl.Rows != 2 {
		t.Fatalf("jsonl rows = %d", jsonl.Rows)
	}
	lines := strings.Split(strings.TrimSpace(performJSONRequest(t, server, http.MethodGet, jsonl.DownloadURL, nil, "").Body.String()), "\n")
	var first map[string]interface{}
	if len(lines) != 2 || json.Unmarshal([]byte(lines[0]), &first) != nil || first["app_name"] != "billing-api" {
		t.Fatalf("jsonl lines = %v", lines)
	}
	for name, body := range map[string]gin.H{"bad format": {"format": "xml"}, "bad kind": {"kind": "nope"}, "bad time": {"query": "from=yesterday"}} {
		if r := performJSONRequest(t, server, http.MethodPost, "/v1/logs/export", body, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", name, r.Code)
		}
	}
	// Expired exports disappear with their files.
	server.db.Exec("UPDATE log_exports SET expires_at = ?", time.Now().Add(-time.Minute).UTC())
	if n := server.cleanupExports(time.Now().UTC()); n != 2 {
		t.Fatalf("cleanup removed %d, want 2", n)
	}
	if r := performJSONRequest(t, server, http.MethodGet, csvExport.DownloadURL, nil, ""); r.Code != http.StatusNotFound {
		t.Errorf("expired download = %d", r.Code)
	}
}

func TestDottedAttributeKeysMustBeWellFormed(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, _ := createAppWithKey(t, server, "x")
	for _, bad := range []string{"a.", ".a", "a..b"} {
		if r := performJSONRequest(t, server, http.MethodGet, "/v1/logs?app_id="+appID+"&attr="+bad+":1", nil, ""); r.Code != http.StatusBadRequest {
			t.Errorf("attr %q = %d, want 400", bad, r.Code)
		}
	}
}

func TestSearchTimeoutIsTyped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	logQueryFailed(c, context.DeadlineExceeded, "x")
	if w.Code != http.StatusGatewayTimeout || !strings.Contains(w.Body.String(), `"code":"search_timeout"`) {
		t.Fatalf("timeout = %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	logQueryFailed(c, logstore.ErrBadFilter, "x")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad filter = %d", w.Code)
	}
}
