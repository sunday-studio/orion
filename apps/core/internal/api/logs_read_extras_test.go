package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestHistogramAndFieldCatalog(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, key := createAppWithKey(t, server, "analytics")
	now := time.Now().UTC()
	var lines []string
	for i := 0; i < 40; i++ {
		sev := "info"
		if i%4 == 0 {
			sev = "error"
		}
		at := now.Add(-time.Duration(i) * time.Minute).Format(time.RFC3339Nano)
		lines = append(lines, fmt.Sprintf(`{"body":"m%d","severity":"%s","timestamp":%q,"attributes":{"http.status_code":%d,"user":{"tier":"pro"},"cache_hit":%t,"mixed":%s}}`, i, sev, at, 200+(i%3)*100, i%2 == 0, map[bool]string{true: `1`, false: `"one"`}[i%2 == 0]))
	}
	ingest(server, key, "application/x-ndjson", strings.Join(lines, "\n"))
	server.logs.store.Flush(5 * time.Second)

	resp := performJSONRequest(t, server, http.MethodGet, "/v1/logs/histogram?app_id="+appID+"&from=-1h&buckets=6", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("histogram status = %d body = %s", resp.Code, resp.Body.String())
	}
	var hist struct {
		Data LogHistogramResponse `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &hist)
	var total, errors int64
	for _, b := range hist.Data.Buckets {
		total += b.Total
		errors += int64(b.Severity["error"])
	}
	if hist.Data.BucketSeconds < 600 || len(hist.Data.Buckets) < 6 || total != 40 || errors != 10 {
		t.Fatalf("histogram = width %ds buckets %d total %d errors %d, want ~10 min buckets and all 40 entries (10 errors)", hist.Data.BucketSeconds, len(hist.Data.Buckets), total, errors)
	}
	if bad := performJSONRequest(t, server, http.MethodGet, "/v1/logs/histogram?from=-1h&to=-2h", nil, ""); bad.Code != http.StatusBadRequest {
		t.Errorf("inverted range status = %d, want 400", bad.Code)
	}

	fields := performJSONRequest(t, server, http.MethodGet, "/v1/logs/fields?app_id="+appID, nil, "")
	var cat struct {
		Data LogFieldCatalogResponse `json:"data"`
	}
	json.Unmarshal(fields.Body.Bytes(), &cat)
	byKey := map[string]LogFieldStat{}
	for _, f := range cat.Data.Fields {
		byKey[f.Key] = f
	}
	if byKey["http.status_code"].Type != "number" || byKey["http.status_code"].Cardinality != 3 || byKey["user"].Type != "object" || byKey["cache_hit"].Type != "boolean" || byKey["mixed"].Type != "mixed" {
		t.Fatalf("catalog = %+v", byKey)
	}
	if len(byKey["http.status_code"].SampleValues) != 3 {
		t.Fatalf("sample values = %v", byKey["http.status_code"].SampleValues)
	}
	if got := performJSONRequest(t, server, http.MethodGet, "/v1/logs/fields", nil, "").Code; got != http.StatusBadRequest {
		t.Errorf("fields without app_id status = %d, want 400", got)
	}
}

func TestSavedViewsCRUD(t *testing.T) {
	server := setupLogsServer(t, nil)
	create := performJSONRequest(t, server, http.MethodPost, "/v1/logs/views", gin.H{"name": "prod errors", "filters": gin.H{"severity": ">=error", "app_id": "app-1"}}, "")
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", create.Code, create.Body.String())
	}
	var created struct {
		Data struct {
			View LogSavedView `json:"view"`
		} `json:"data"`
	}
	json.Unmarshal(create.Body.Bytes(), &created)
	list := performJSONRequest(t, server, http.MethodGet, "/v1/logs/views", nil, "")
	var listed struct {
		Data struct {
			Views []LogSavedView `json:"views"`
		} `json:"data"`
	}
	json.Unmarshal(list.Body.Bytes(), &listed)
	if len(listed.Data.Views) != 1 || listed.Data.Views[0].Name != "prod errors" || listed.Data.Views[0].Filters["severity"] != ">=error" {
		t.Fatalf("list = %s", list.Body.String())
	}
	for name, body := range map[string]gin.H{"unknown filter": {"name": "x", "filters": gin.H{"sql": "1=1"}}, "no name": {"name": " ", "filters": gin.H{"q": "a"}}} {
		if got := performJSONRequest(t, server, http.MethodPost, "/v1/logs/views", body, "").Code; got != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400", name, got)
		}
	}
	if got := performJSONRequest(t, server, http.MethodDelete, "/v1/logs/views/"+created.Data.View.ID, nil, "").Code; got != http.StatusOK {
		t.Errorf("delete status = %d", got)
	}
	if got := performJSONRequest(t, server, http.MethodDelete, "/v1/logs/views/"+created.Data.View.ID, nil, "").Code; got != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404", got)
	}
}

func TestSearchAgentSourceReturnsAgentServiceLogs(t *testing.T) {
	server := setupLogsServer(t, nil)
	agent := registerTestAgent(t, server)
	now := time.Now().UTC()
	for i, level := range []string{"INFO", "ERROR", "WARNING"} {
		server.db.Create(&db.ServiceLogEntry{ID: fmt.Sprintf("sl-%d", i), AgentID: agent.Data.AgentID, Level: level, Message: fmt.Sprintf("agent said %d", i), Component: "reporter", Fingerprint: fmt.Sprintf("f%d", i), OccurredAt: now.Add(-time.Duration(i) * time.Minute), CollectedAt: now, FieldsJSON: `{"k":"v"}`})
	}
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/logs?source=agent&agent_id="+agent.Data.AgentID+"&limit=2", nil, "")
	var out logList
	json.Unmarshal(resp.Body.Bytes(), &out)
	if resp.Code != http.StatusOK || len(out.Data.Entries) != 2 || out.Data.NextCursor == "" || out.Data.Entries[0].Body != "agent said 0" {
		t.Fatalf("agent logs page 1 = %d %s", resp.Code, resp.Body.String())
	}
	page2 := performJSONRequest(t, server, http.MethodGet, "/v1/logs?source=agent&limit=2&cursor="+out.Data.NextCursor, nil, "")
	if !strings.Contains(page2.Body.String(), "agent said 2") || !strings.Contains(page2.Body.String(), `"source":"agent"`) {
		t.Fatalf("agent logs page 2 = %s", page2.Body.String())
	}
	errs := performJSONRequest(t, server, http.MethodGet, "/v1/logs?source=agent&severity=error", nil, "")
	if !strings.Contains(errs.Body.String(), "agent said 1") || strings.Contains(errs.Body.String(), "agent said 0") {
		t.Fatalf("severity=error agent logs = %s", errs.Body.String())
	}
}

type sseReader struct {
	lines chan string
}

func openStream(t *testing.T, ts *httptest.Server, path string, header http.Header) (*sseReader, func()) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream status = %d content-type = %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	r := &sseReader{lines: make(chan string, 1000)}
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			r.lines <- scanner.Text()
		}
		close(r.lines)
	}()
	return r, func() { resp.Body.Close() }
}

func (r *sseReader) nextData(t *testing.T) string {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case line, ok := <-r.lines:
			if !ok {
				t.Fatal("stream closed")
			}
			if strings.HasPrefix(line, "data: ") {
				return strings.TrimPrefix(line, "data: ")
			}
		case <-deadline:
			t.Fatal("timed out waiting for a stream event")
		}
	}
}

func (r *sseReader) waitFor(t *testing.T, comment string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case line := <-r.lines:
			if strings.Contains(line, comment) {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q", comment)
		}
	}
}

func TestLiveTailStreamsFilteredEntriesAndResumes(t *testing.T) {
	server := setupLogsServer(t, nil)
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()
	appID, key := createAppWithKey(t, server, "tail")

	stream, closeStream := openStream(t, ts, "/v1/logs/stream?app_id="+appID+"&severity=error", nil)
	stream.waitFor(t, "connected")
	ingest(server, key, "application/json", `[{"body":"chatter","severity":"info"},{"body":"boom","severity":"error","event":"job.failed"}]`)
	var got LogEntry
	if err := json.Unmarshal([]byte(stream.nextData(t)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Body != "boom" || got.Event != "job.failed" || got.Source != "app" {
		t.Fatalf("streamed entry = %+v, want only the error entry", got)
	}
	closeStream()

	// While disconnected two more errors arrive; reconnecting with Last-Event-ID replays them.
	ingest(server, key, "application/json", `[{"body":"missed-1","severity":"error"},{"body":"missed-2","severity":"error"}]`)
	server.logs.store.Flush(3 * time.Second)
	resumed, closeResumed := openStream(t, ts, "/v1/logs/stream?app_id="+appID+"&severity=error", http.Header{"Last-Event-Id": {got.ID}})
	defer closeResumed()
	first, second := resumed.nextData(t), resumed.nextData(t)
	if !strings.Contains(first, "missed-1") || !strings.Contains(second, "missed-2") {
		t.Fatalf("replayed %q then %q, want the two missed entries in order", first, second)
	}
	ingest(server, key, "application/json", `{"body":"live-again","severity":"error"}`)
	if live := resumed.nextData(t); !strings.Contains(live, "live-again") {
		t.Fatalf("after replay got %q, want the live entry", live)
	}
}
