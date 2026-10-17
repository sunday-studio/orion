package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/logstore"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupLogsServer(t *testing.T, cfg *config.Config) *Server {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{}
	}
	server := setupTestServerWithConfig(t, cfg)
	store, err := logstore.OpenWith(testkit.NewDB(t), utils.NewLogger(), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	server.EnableProductLogs(store)
	return server
}

func ingest(server *Server, key string, contentType string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/logs", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	return rawRequest(server, req)
}

func createAppWithKey(t *testing.T, server *Server, name string) (appID string, key string) {
	t.Helper()
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/apps", gin.H{"name": name}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("create app status = %d body = %s", resp.Code, resp.Body.String())
	}
	var app struct {
		Data struct {
			App struct {
				ID string `json:"id"`
			} `json:"app"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &app)
	keyResp := performJSONRequest(t, server, http.MethodPost, "/v1/apps/"+app.Data.App.ID+"/keys", gin.H{"name": "prod"}, "")
	if keyResp.Code != http.StatusCreated {
		t.Fatalf("create key status = %d body = %s", keyResp.Code, keyResp.Body.String())
	}
	var created struct {
		Data struct {
			Key struct {
				Key string `json:"key"`
			} `json:"key"`
		} `json:"data"`
	}
	json.Unmarshal(keyResp.Body.Bytes(), &created)
	if !strings.HasPrefix(created.Data.Key.Key, logstore.KeyPrefix) {
		t.Fatalf("key = %q, want an orion_ik_ key", created.Data.Key.Key)
	}
	return app.Data.App.ID, created.Data.Key.Key
}

type logList struct {
	Data struct {
		Entries []struct {
			ID         string                 `json:"id"`
			AppID      string                 `json:"app_id"`
			AppName    string                 `json:"app_name"`
			Severity   string                 `json:"severity"`
			Event      string                 `json:"event"`
			Body       string                 `json:"body"`
			Attributes map[string]interface{} `json:"attributes"`
		} `json:"entries"`
		NextCursor string `json:"next_cursor"`
	} `json:"data"`
}

func searchLogsAPI(t *testing.T, server *Server, query string) logList {
	t.Helper()
	server.logs.store.Flush(5 * time.Second)
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/logs?"+query, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("search status = %d body = %s", resp.Code, resp.Body.String())
	}
	var out logList
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// The documented minimum: one curl, one entry, visible through the API.
func TestCurlEntryShowsUpViaTheAPI(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, key := createAppWithKey(t, server, "Checkout API")

	resp := ingest(server, key, "application/json", `{"severity":"error","event":"payment.failed","body":"payment failed for order 8841","attributes":{"order_id":8841,"password":"hunter2"}}`)
	if resp.Code != http.StatusAccepted || strings.TrimSpace(resp.Body.String()) != `{"accepted":1,"rejected":0}` {
		t.Fatalf("ingest status = %d body = %s, want 202 {accepted:1}", resp.Code, resp.Body.String())
	}

	list := searchLogsAPI(t, server, "app_id="+appID)
	if len(list.Data.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(list.Data.Entries))
	}
	e := list.Data.Entries[0]
	if e.Event != "payment.failed" || e.Severity != "error" || e.AppName != "Checkout API" || e.Attributes["order_id"] != float64(8841) {
		t.Fatalf("entry = %+v", e)
	}
	if e.Attributes["password"] != "[redacted]" {
		t.Fatalf("password attribute = %v, want redacted by default", e.Attributes["password"])
	}

	one := performJSONRequest(t, server, http.MethodGet, "/v1/logs/"+e.ID, nil, "")
	if one.Code != http.StatusOK || !strings.Contains(one.Body.String(), "payment failed for order 8841") {
		t.Fatalf("get one status = %d body = %s", one.Code, one.Body.String())
	}
	if miss := performJSONRequest(t, server, http.MethodGet, "/v1/logs/log_missing", nil, ""); miss.Code != http.StatusNotFound {
		t.Fatalf("missing entry status = %d, want 404", miss.Code)
	}
}

func TestIngestFormatsLimitsAndErrors(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, key := createAppWithKey(t, server, "multi")

	ndjson := "{\"body\":\"a\"}\n{\"body\":\"b\",\"level\":\"warn\"}\nnot json\n"
	resp := ingest(server, key, "application/x-ndjson", ndjson)
	var r IngestResponse
	json.Unmarshal(resp.Body.Bytes(), &r)
	if resp.Code != http.StatusAccepted || r.Accepted != 2 || r.Rejected != 1 || r.Errors[0].Code != "invalid_json" || r.Errors[0].Index != 2 {
		t.Fatalf("ndjson status=%d resp=%+v", resp.Code, r)
	}
	arr := ingest(server, key, "application/json", `[{"body":"c"},{"body":"d"},{}]`)
	json.Unmarshal(arr.Body.Bytes(), &r)
	if arr.Code != http.StatusAccepted || r.Accepted != 2 || r.Errors[0].Code != "empty_entry" {
		t.Fatalf("array status=%d resp=%+v", arr.Code, r)
	}
	if n := len(searchLogsAPI(t, server, "app_id="+appID+"&limit=200").Data.Entries); n != 4 {
		t.Fatalf("stored %d entries, want 4", n)
	}

	cases := map[string]struct {
		key, ct, body string
		want          int
	}{
		"no key":            {"", "application/json", `{"body":"x"}`, http.StatusUnauthorized},
		"wrong key":         {"orion_ik_" + strings.Repeat("0", 40), "application/json", `{"body":"x"}`, http.StatusUnauthorized},
		"agent style token": {"tok_abc", "application/json", `{"body":"x"}`, http.StatusUnauthorized},
		"bad content type":  {key, "text/plain", `hello`, http.StatusUnsupportedMediaType},
		"not json":          {key, "application/json", `{broken`, http.StatusBadRequest},
		"empty":             {key, "application/json", ``, http.StatusBadRequest},
	}
	for name, tc := range cases {
		if got := ingest(server, tc.key, tc.ct, tc.body).Code; got != tc.want {
			t.Errorf("%s: status = %d, want %d", name, got, tc.want)
		}
	}

	big := fmt.Sprintf(`{"body":%q}`, strings.Repeat("x", 2<<20))
	if got := ingest(server, key, "application/json", big).Code; got != http.StatusRequestEntityTooLarge {
		t.Errorf("2 MiB JSON status = %d, want 413", got)
	}
	// An NDJSON request may be larger than 1 MiB (up to 5 MiB).
	line := fmt.Sprintf(`{"body":%q}`+"\n", strings.Repeat("y", 30<<10))
	if got := ingest(server, key, "application/x-ndjson", strings.Repeat(line, 50)).Code; got != http.StatusAccepted {
		t.Errorf("1.5 MiB NDJSON status = %d, want 202", got)
	}
}

func TestIngestKeysAreIsolatedWriteOnlyAndRevocable(t *testing.T) {
	server := setupLogsServer(t, &config.Config{AdminUsername: "admin", AdminPassword: "pw", JWTSecret: "secret-secret-secret-secret-1234", FrontendAuthOn: true})
	login := performJSONRequest(t, server, http.MethodPost, "/v1/auth/login", gin.H{"username": "admin", "password": "pw"}, "")
	var tok struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(login.Body.Bytes(), &tok)
	admin := tok.Data.Token

	mk := func(name string) (string, string) {
		r := performJSONRequest(t, server, http.MethodPost, "/v1/apps", gin.H{"name": name}, admin)
		var a struct {
			Data struct {
				App struct {
					ID string `json:"id"`
				} `json:"app"`
			} `json:"data"`
		}
		json.Unmarshal(r.Body.Bytes(), &a)
		kr := performJSONRequest(t, server, http.MethodPost, "/v1/apps/"+a.Data.App.ID+"/keys", gin.H{"name": "k"}, admin)
		var k struct {
			Data struct {
				Key struct{ ID, Key string } `json:"key"`
			} `json:"data"`
		}
		json.Unmarshal(kr.Body.Bytes(), &k)
		return a.Data.App.ID, k.Data.Key.Key + "|" + k.Data.Key.ID
	}
	appA, keyA := mk("alpha")
	appB, _ := mk("beta")
	keyAValue, keyAID := strings.Split(keyA, "|")[0], strings.Split(keyA, "|")[1]

	if ingest(server, keyAValue, "application/json", `{"body":"from A","app_id":"`+appB+`"}`).Code != http.StatusAccepted {
		t.Fatal("ingest failed")
	}
	server.logs.store.Flush(3 * time.Second)
	if n, _, _ := server.logs.store.CountForApp(appA); n != 1 {
		t.Fatalf("app A has %d entries, want 1", n)
	}
	if n, _, _ := server.logs.store.CountForApp(appB); n != 0 {
		t.Fatalf("app B has %d entries: a key for A must never write into B, even if the payload names B", n)
	}

	// Write-only: the ingest key cannot read logs, list apps or create keys.
	for _, path := range []string{"/v1/logs", "/v1/apps", "/v1/apps/" + appA + "/keys"} {
		if resp := performJSONRequest(t, server, http.MethodGet, path, nil, keyAValue); resp.Code != http.StatusUnauthorized {
			t.Errorf("GET %s with an ingest key status = %d, want 401", path, resp.Code)
		}
	}
	// And management needs the admin login.
	if resp := performJSONRequest(t, server, http.MethodGet, "/v1/apps", nil, ""); resp.Code != http.StatusUnauthorized {
		t.Errorf("GET /v1/apps without login status = %d, want 401", resp.Code)
	}
	// An agent token cannot ingest as an app.
	agent := registerTestAgent(t, server)
	if got := ingest(server, agent.Data.Token, "application/json", `{"body":"x"}`).Code; got != http.StatusUnauthorized {
		t.Errorf("agent token ingest status = %d, want 401", got)
	}

	// Listing keys never reveals the secret; revoking is immediate.
	list := performJSONRequest(t, server, http.MethodGet, "/v1/apps/"+appA+"/keys", nil, admin)
	if strings.Contains(list.Body.String(), keyAValue) || !strings.Contains(list.Body.String(), keyAValue[:len(logstore.KeyPrefix)+8]) {
		t.Fatalf("key list = %s, want only the prefix", list.Body.String())
	}
	if resp := performJSONRequest(t, server, http.MethodDelete, "/v1/apps/"+appA+"/keys/"+keyAID, nil, admin); resp.Code != http.StatusOK {
		t.Fatalf("revoke status = %d", resp.Code)
	}
	if got := ingest(server, keyAValue, "application/json", `{"body":"after revoke"}`).Code; got != http.StatusUnauthorized {
		t.Fatalf("revoked key status = %d, want 401 immediately", got)
	}
}

func TestIngestRateLimitReturns429WithRetryAfter(t *testing.T) {
	server := setupLogsServer(t, &config.Config{LogsAppRatePerSecond: 5})
	_, key := createAppWithKey(t, server, "noisy")
	var entries []string
	for i := 0; i < 10; i++ {
		entries = append(entries, `{"body":"x"}`)
	}
	payload := "[" + strings.Join(entries, ",") + "]"
	if got := ingest(server, key, "application/json", payload).Code; got != http.StatusAccepted {
		t.Fatalf("first burst status = %d, want 202", got)
	}
	resp := ingest(server, key, "application/json", payload)
	if resp.Code != http.StatusTooManyRequests || resp.Header().Get("Retry-After") == "" {
		t.Fatalf("second burst status = %d Retry-After = %q, want 429 with Retry-After", resp.Code, resp.Header().Get("Retry-After"))
	}
}

func TestAppSettingsDriveRedactionAndDeletionRemovesLogs(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, key := createAppWithKey(t, server, "settings")

	patch := performJSONRequest(t, server, http.MethodPatch, "/v1/apps/"+appID, gin.H{"retention_days": 30, "redaction": gin.H{"extra_keys": []string{"customer_note"}}}, "")
	if patch.Code != http.StatusOK {
		t.Fatalf("patch status = %d body = %s", patch.Code, patch.Body.String())
	}
	ingest(server, key, "application/json", `{"body":"x","attributes":{"customer_note":"secret stuff","plain":"fine"}}`)
	entry := searchLogsAPI(t, server, "app_id="+appID).Data.Entries[0]
	if entry.Attributes["customer_note"] != "[redacted]" || entry.Attributes["plain"] != "fine" {
		t.Fatalf("attributes = %v, want the per-app redaction applied immediately", entry.Attributes)
	}

	for name, body := range map[string]gin.H{"retention": {"retention_days": 0}, "sample": {"debug_sample_rate": 2}, "empty name": {"name": "  "}, "storage": {"max_storage_mb": -1}} {
		if resp := performJSONRequest(t, server, http.MethodPatch, "/v1/apps/"+appID, body, ""); resp.Code != http.StatusBadRequest {
			t.Errorf("%s: patch status = %d, want 400", name, resp.Code)
		}
	}
	if resp := performJSONRequest(t, server, http.MethodPost, "/v1/apps", gin.H{}, ""); resp.Code != http.StatusBadRequest {
		t.Errorf("create without name status = %d, want 400", resp.Code)
	}
	// Slugs stay unique for the same name.
	second := performJSONRequest(t, server, http.MethodPost, "/v1/apps", gin.H{"name": "settings"}, "")
	if !strings.Contains(second.Body.String(), `"slug":"settings-2"`) {
		t.Fatalf("second app = %s, want slug settings-2", second.Body.String())
	}

	del := performJSONRequest(t, server, http.MethodDelete, "/v1/apps/"+appID, nil, "")
	if del.Code != http.StatusOK {
		t.Fatalf("delete status = %d", del.Code)
	}
	if got := ingest(server, key, "application/json", `{"body":"after delete"}`).Code; got != http.StatusUnauthorized {
		t.Fatalf("ingest after app deletion status = %d, want 401", got)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if n, _, _ := server.logs.store.CountForApp(appID); n == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("app logs were not removed after the app was deleted")
}

func TestSearchFiltersAndPaginationThroughTheAPI(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, key := createAppWithKey(t, server, "search")
	var lines []string
	for i := 0; i < 30; i++ {
		sev := "info"
		if i%10 == 0 {
			sev = "error"
		}
		lines = append(lines, fmt.Sprintf(`{"body":"line %d","severity":"%s","request_id":"req-%d","attributes":{"shard":%d}}`, i, sev, i%3, i%2))
	}
	ingest(server, key, "application/x-ndjson", strings.Join(lines, "\n"))

	if n := len(searchLogsAPI(t, server, "app_id="+appID+"&severity=error").Data.Entries); n != 3 {
		t.Errorf("severity=error returned %d, want 3", n)
	}
	if n := len(searchLogsAPI(t, server, "app_id="+appID+"&request_id=req-1&limit=200").Data.Entries); n != 10 {
		t.Errorf("request_id filter returned %d, want 10", n)
	}
	if n := len(searchLogsAPI(t, server, "app_id="+appID+"&attr=shard:1&limit=200").Data.Entries); n != 15 {
		t.Errorf("attr filter returned %d, want 15", n)
	}
	if n := len(searchLogsAPI(t, server, "app_id="+appID+"&q=line%2012").Data.Entries); n != 1 {
		t.Errorf("q search returned %d, want 1", n)
	}
	page := searchLogsAPI(t, server, "app_id="+appID+"&limit=20")
	next := searchLogsAPI(t, server, "app_id="+appID+"&limit=20&cursor="+page.Data.NextCursor)
	if len(page.Data.Entries) != 20 || len(next.Data.Entries) != 10 || next.Data.NextCursor != "" {
		t.Fatalf("pagination: %d then %d (next %q), want 20 then 10 and no more", len(page.Data.Entries), len(next.Data.Entries), next.Data.NextCursor)
	}
	for _, bad := range []string{"from=yesterday", "attr=nokeyvalue", "cursor=%25%25", "attr=a%22b:1"} {
		if resp := performJSONRequest(t, server, http.MethodGet, "/v1/logs?"+bad, nil, ""); resp.Code != http.StatusBadRequest {
			t.Errorf("GET /v1/logs?%s status = %d, want 400", bad, resp.Code)
		}
	}
	rel := searchLogsAPI(t, server, "app_id="+appID+"&from=-1h&to="+time.Now().UTC().Add(time.Hour).Format(time.RFC3339))
	if len(rel.Data.Entries) == 0 {
		t.Error("relative from=-1h returned nothing")
	}
}

func TestSendTestLogAndNotEnabledAnd501Stubs(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, _ := createAppWithKey(t, server, "tester")
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/apps/"+appID+"/test-log", nil, "")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("test-log status = %d body = %s", resp.Code, resp.Body.String())
	}
	if e := searchLogsAPI(t, server, "app_id="+appID+"&event=orion.test").Data.Entries; len(e) != 1 {
		t.Fatalf("test log entries = %d, want 1", len(e))
	}
	if got := performJSONRequest(t, server, http.MethodPost, "/v1/apps/missing/test-log", nil, "").Code; got != http.StatusNotFound {
		t.Errorf("test-log for a missing app status = %d, want 404", got)
	}

	disabled := setupTestServer(t)
	if got := ingest(disabled, "orion_ik_x", "application/json", `{"body":"x"}`).Code; got != http.StatusServiceUnavailable {
		t.Errorf("ingest with logs disabled status = %d, want 503", got)
	}
	if got := performJSONRequest(t, disabled, http.MethodGet, "/v1/logs", nil, "").Code; got != http.StatusServiceUnavailable {
		t.Errorf("search with logs disabled status = %d, want 503", got)
	}
	var buf bytes.Buffer
	_ = buf
}

func TestTooManyEntriesInOneRequestIs413(t *testing.T) {
	server := setupLogsServer(t, nil)
	_, key := createAppWithKey(t, server, "bulk")
	var b strings.Builder
	for i := 0; i <= logstore.MaxEntriesPerRequest; i++ {
		b.WriteString("{\"body\":\"x\"}\n")
	}
	if got := ingest(server, key, "application/x-ndjson", b.String()).Code; got != http.StatusRequestEntityTooLarge {
		t.Fatalf("10001 entries status = %d, want 413", got)
	}
}
