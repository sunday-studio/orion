package api

import (
	"encoding/json"
	"net/http"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type incidentLogsBody struct {
	Data struct {
		Entries []struct {
			Body string `json:"body"`
		} `json:"entries"`
		Source          string   `json:"source"`
		ServiceID       string   `json:"service_id"`
		ServiceName     string   `json:"service_name"`
		AppIDs          []string `json:"app_ids"`
		PreIncidentOnly bool     `json:"pre_incident_only"`
	} `json:"data"`
}

func (b incidentLogsBody) bodies() string {
	var all []string
	for _, e := range b.Data.Entries {
		all = append(all, e.Body)
	}
	return strings.Join(all, "|")
}

func getIncidentLogsBody(t *testing.T, server *Server, query string) incidentLogsBody {
	t.Helper()
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/inc-1/logs"+query, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.Code, resp.Body.String())
	}
	var out incidentLogsBody
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// incidentLogsFixture has an incident that opened ten minutes ago on monitor mon-1 and an app with
// a line before, during and after the opening.
func incidentLogsFixture(t *testing.T) (server *Server, appID string) {
	t.Helper()
	server = setupLogsServer(t, nil)
	appID, key := createAppWithKey(t, server, "checkout-api")
	node := registerTestNode(t, server)
	opened := time.Now().UTC().Add(-10 * time.Minute)
	now := time.Now().UTC()
	server.db.Create(&db.Monitor{ID: "mon-1", NodeID: node.Data.NodeID, Name: "checkout health", Type: "http", Lifecycle: "active", Health: "down", ComputedHealth: "down", IncidentState: "down", CreatedAt: now, UpdatedAt: now})
	server.db.Create(&db.Incident{ID: "inc-1", Status: "open", Severity: "high", Title: "checkout down", NodeID: node.Data.NodeID, MonitorID: "mon-1", OpenedAt: opened, LastEventAt: now, NotificationStatus: "sent"})
	ts := func(d time.Duration) string { return opened.Add(d).Format(time.RFC3339Nano) }
	ingest(server, key, "application/x-ndjson", strings.Join([]string{
		`{"body":"too early","timestamp":"` + ts(-40*time.Minute) + `"}`,
		`{"body":"db pool exhausted","severity":"error","timestamp":"` + ts(-3*time.Minute) + `"}`,
		`{"body":"after it opened","severity":"error","timestamp":"` + ts(2*time.Minute) + `"}`,
	}, "\n"))
	server.logs.store.Flush(3 * time.Second)
	return server, appID
}

func TestIncidentLogsResolveThroughTheMonitorsService(t *testing.T) {
	server, appID := incidentLogsFixture(t)
	svcID := createServiceVia(t, server, gin.H{"name": "Checkout"})

	// Unlinked monitor, no log rule: nothing to show.
	if got := getIncidentLogsBody(t, server, ""); got.Data.Source != "none" || len(got.Data.AppIDs) != 0 || strings.Contains(got.bodies(), "db pool") {
		t.Fatalf("unlinked = %+v", got.Data)
	}

	// Linking the monitor and the app is all it takes: no log rule needed.
	for _, body := range []gin.H{{"kind": "monitor", "id": "mon-1"}, {"kind": "app", "id": appID}} {
		if resp := performJSONRequest(t, server, http.MethodPost, "/v1/services/"+svcID+"/links", body, ""); resp.Code != http.StatusOK {
			t.Fatalf("link %v = %d %s", body, resp.Code, resp.Body.String())
		}
	}
	got := getIncidentLogsBody(t, server, "")
	if got.Data.Source != "service" || got.Data.ServiceID != svcID || got.Data.ServiceName != "Checkout" || len(got.Data.AppIDs) != 1 || got.Data.AppIDs[0] != appID {
		t.Fatalf("linked = %+v", got.Data)
	}
	if !strings.Contains(got.bodies(), "db pool exhausted") || !strings.Contains(got.bodies(), "after it opened") || strings.Contains(got.bodies(), "too early") {
		t.Fatalf("entries = %s", got.bodies())
	}

	// Pre-incident only stops at the moment the incident opened.
	pre := getIncidentLogsBody(t, server, "?pre_incident_only=true")
	if !pre.Data.PreIncidentOnly || !strings.Contains(pre.bodies(), "db pool exhausted") || strings.Contains(pre.bodies(), "after it opened") {
		t.Fatalf("pre-incident = %+v entries = %s", pre.Data, pre.bodies())
	}

	// An explicit app_id still wins over the service.
	explicit := getIncidentLogsBody(t, server, "?app_id="+appID)
	if explicit.Data.Source != "query" || explicit.Data.ServiceID != "" {
		t.Fatalf("explicit = %+v", explicit.Data)
	}

	// Unlinking the app keeps its logs but the incident no longer shows them.
	if resp := performJSONRequest(t, server, http.MethodDelete, "/v1/services/"+svcID+"/links?kind=app&target_id="+appID, nil, ""); resp.Code != http.StatusOK {
		t.Fatalf("unlink = %d %s", resp.Code, resp.Body.String())
	}
	after := getIncidentLogsBody(t, server, "")
	if after.Data.Source != "none" || after.Data.ServiceID != svcID || strings.Contains(after.bodies(), "db pool") {
		t.Fatalf("after unlink = %+v", after.Data)
	}
	if lines, _, err := server.logs.store.CountForApp(appID); err != nil || lines != 3 {
		t.Fatalf("app lines = %d, %v; unlinking must not touch logs", lines, err)
	}
}

func TestIncidentLogsFallBackToTheLogRuleWhenThereIsNoService(t *testing.T) {
	server, appID := incidentLogsFixture(t)
	now := time.Now().UTC()
	server.db.Create(&db.LogAlertRule{ID: "r1", Name: "errors", AppID: appID, Mode: "threshold", WindowSeconds: 300, Enabled: true, State: "firing", MonitorID: "mon-1", CreatedAt: now, UpdatedAt: now})

	got := getIncidentLogsBody(t, server, "")
	if got.Data.Source != "log_rule" || got.Data.ServiceID != "" || len(got.Data.AppIDs) != 1 || !strings.Contains(got.bodies(), "db pool exhausted") {
		t.Fatalf("rule-only = %+v entries = %s", got.Data, got.bodies())
	}

	// A service that has no app yet falls back to the rule too, and still says which service it is.
	svcID := createServiceVia(t, server, gin.H{"name": "Checkout"})
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+svcID+"/links", gin.H{"kind": "monitor", "id": "mon-1"}, "")
	fallback := getIncidentLogsBody(t, server, "")
	if fallback.Data.Source != "log_rule" || fallback.Data.ServiceID != svcID || !strings.Contains(fallback.bodies(), "db pool exhausted") {
		t.Fatalf("service without apps = %+v", fallback.Data)
	}
}
