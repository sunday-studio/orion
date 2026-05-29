package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestServer(t *testing.T) *Server {
	return setupTestServerWithConfig(t, &config.Config{AlertRecoveryNotifications: true, AlertTLSExpiryDays: 14})
}

func setupTestServerWithConfig(t *testing.T, cfg *config.Config) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return NewServer(database, logging.NewLogger(), cfg)
}

func registerTestAgent(t *testing.T, server *Server) struct {
	Success bool `json:"success"`
	Data    struct {
		AgentID string `json:"agent_id"`
		Token   string `json:"token"`
	} `json:"data"`
} {
	t.Helper()

	registerBody := map[string]interface{}{
		"machine_id":                 "test-machine-" + t.Name(),
		"name":                       "test-server",
		"os":                         "linux",
		"arch":                       "arm64",
		"reporting_interval_seconds": 60,
	}
	registerResp := performJSONRequest(t, server, http.MethodPost, "/v1/register", registerBody, "")
	if registerResp.Code != http.StatusOK {
		t.Fatalf("register status = %d, body = %s", registerResp.Code, registerResp.Body.String())
	}

	var registered struct {
		Success bool `json:"success"`
		Data    struct {
			AgentID string `json:"agent_id"`
			Token   string `json:"token"`
		} `json:"data"`
	}
	decodeResponse(t, registerResp, &registered)
	if !registered.Success || registered.Data.AgentID == "" || registered.Data.Token == "" {
		t.Fatalf("registration response missing agent identity: %+v", registered)
	}

	return registered
}

func registerTestMonitor(t *testing.T, server *Server, agentID string, token string) struct {
	Success bool `json:"success"`
	Data    struct {
		MonitorID string `json:"monitor_id"`
	} `json:"data"`
} {
	t.Helper()

	description := "Checks the homepage"
	registerMonitorBody := map[string]interface{}{
		"agent_id":                   agentID,
		"name":                       "homepage",
		"description":                description,
		"type":                       "http-healthcheck",
		"last_checked":               time.Now().UTC().Format(time.RFC3339),
		"reporting_interval_seconds": 30,
	}
	registerMonitorPath := "/v1/agents/" + agentID + "/register-monitor"
	registerMonitorResp := performJSONRequest(t, server, http.MethodPost, registerMonitorPath, registerMonitorBody, token)
	if registerMonitorResp.Code != http.StatusOK {
		t.Fatalf("register monitor status = %d, body = %s", registerMonitorResp.Code, registerMonitorResp.Body.String())
	}

	var registeredMonitor struct {
		Success bool `json:"success"`
		Data    struct {
			MonitorID string `json:"monitor_id"`
		} `json:"data"`
	}
	decodeResponse(t, registerMonitorResp, &registeredMonitor)
	if !registeredMonitor.Success || registeredMonitor.Data.MonitorID == "" {
		t.Fatalf("registration response missing monitor identity: %+v", registeredMonitor)
	}

	return registeredMonitor
}

func assertIncidentCandidateCount(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()

	if response.Code != http.StatusOK {
		t.Fatalf("incident candidates status = %d, body = %s", response.Code, response.Body.String())
	}

	var candidates struct {
		Success bool `json:"success"`
		Data    struct {
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, response, &candidates)
	if !candidates.Success || candidates.Data.Count != want {
		t.Fatalf("incident candidate count = %+v, want %d", candidates, want)
	}
}

func assertAlertDelivery(t *testing.T, server *Server, incidentID string, eventType string, wantStatus string) {
	t.Helper()

	var delivery db.AlertDelivery
	if err := server.db.Where("incident_id = ? AND event_type = ?", incidentID, eventType).First(&delivery).Error; err != nil {
		t.Fatalf("find alert delivery: %v", err)
	}
	if delivery.Status != wantStatus {
		t.Fatalf("alert delivery status = %q, want %q", delivery.Status, wantStatus)
	}
}

func assertIncidentEvent(t *testing.T, server *Server, incidentID string, eventType string, wantMessage string) {
	t.Helper()

	var event db.IncidentEvent
	if err := server.db.Where("incident_id = ? AND type = ? AND message = ?", incidentID, eventType, wantMessage).First(&event).Error; err != nil {
		t.Fatalf("find incident event %s: %v", eventType, err)
	}
}

func assertIncidentEventMetadata(t *testing.T, server *Server, incidentID string, eventType string, wantActorType string, wantActorID string, wantNote string) {
	t.Helper()

	var event db.IncidentEvent
	if err := server.db.Where("incident_id = ? AND type = ?", incidentID, eventType).Order("created_at DESC").First(&event).Error; err != nil {
		t.Fatalf("find incident event %s: %v", eventType, err)
	}
	if event.ActorType != wantActorType || event.ActorID != wantActorID || event.Note != wantNote {
		t.Fatalf("incident event metadata = actor_type:%q actor_id:%q note:%q, want actor_type:%q actor_id:%q note:%q", event.ActorType, event.ActorID, event.Note, wantActorType, wantActorID, wantNote)
	}
}

func assertMonitorIncidentState(t *testing.T, server *Server, monitorID string, wantActiveIncidentID string, wantIncidentState string) {
	t.Helper()

	var monitor db.Monitor
	if err := server.db.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
		t.Fatalf("find monitor: %v", err)
	}
	if monitor.ActiveIncidentID != wantActiveIncidentID || monitor.IncidentState != wantIncidentState {
		t.Fatalf("monitor incident state = active %q state %q, want active %q state %q", monitor.ActiveIncidentID, monitor.IncidentState, wantActiveIncidentID, wantIncidentState)
	}
}

func seedCoreConfirmationMonitor(t *testing.T, server *Server, monitorID string, confirmationPeriodSeconds int, confirmationCheckCount int) db.Monitor {
	t.Helper()

	return seedCoreNoiseMonitor(t, server, monitorID, confirmationPeriodSeconds, confirmationCheckCount, 0)
}

func seedCoreNoiseMonitor(t *testing.T, server *Server, monitorID string, confirmationPeriodSeconds int, confirmationCheckCount int, recoveryPeriodSeconds int) db.Monitor {
	t.Helper()

	now := time.Now().UTC()
	monitor := db.Monitor{
		ID:                       monitorID,
		AgentID:                  "agent-core-worker",
		Name:                     monitorID,
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		ReportingIntervalSeconds: 30,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create core confirmation monitor: %v", err)
	}
	config := db.CoreMonitorConfig{
		MonitorID:                 monitor.ID,
		Kind:                      "http",
		ConfigJSON:                "{}",
		SecretRefJSON:             "{}",
		IntervalSeconds:           30,
		TimeoutSeconds:            10,
		ConfirmationPeriodSeconds: confirmationPeriodSeconds,
		ConfirmationCheckCount:    confirmationCheckCount,
		RecoveryPeriodSeconds:     recoveryPeriodSeconds,
		NextRunAt:                 now,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := server.db.Create(&config).Error; err != nil {
		t.Fatalf("create core confirmation config: %v", err)
	}
	return monitor
}

func seedCoreMaintenanceMonitor(t *testing.T, server *Server, monitorID string, startAt time.Time, endAt time.Time) db.Monitor {
	t.Helper()

	monitor := seedCoreNoiseMonitor(t, server, monitorID, 0, 0, 0)
	configJSON := fmt.Sprintf(
		`{"url":"https://api.example.com/health","maintenance_windows":[{"start_at":%q,"end_at":%q}]}`,
		startAt.Format(time.RFC3339),
		endAt.Format(time.RFC3339),
	)
	if err := server.db.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", monitor.ID).Update("config_json", configJSON).Error; err != nil {
		t.Fatalf("set core maintenance config: %v", err)
	}
	return monitor
}

func storeCoreConfirmationReport(t *testing.T, server *Server, monitorID string, health string, reportedAt time.Time) {
	t.Helper()

	statusCode := 500
	if health == "up" {
		statusCode = 200
	}
	reportID, err := server.reportService.StoreMonitorReport(monitorID, service.MonitorReportPayload{
		Timestamp: reportedAt.Format(time.RFC3339),
		Health:    health,
		Metrics: map[string]interface{}{
			"runner":      "core",
			"status_code": statusCode,
		},
	})
	if err != nil {
		t.Fatalf("store core confirmation %s report: %v", health, err)
	}
	if reportID == nil || *reportID == "" {
		t.Fatalf("core confirmation report id = %v, want generated id", reportID)
	}
}

func assertCoreIncidentCount(t *testing.T, server *Server, monitorID string, want int64) {
	t.Helper()

	var count int64
	if err := server.db.Model(&db.Incident{}).Where("monitor_id = ?", monitorID).Count(&count).Error; err != nil {
		t.Fatalf("count core confirmation incidents: %v", err)
	}
	if count != want {
		t.Fatalf("core confirmation incident count = %d, want %d", count, want)
	}
}

func assertIncidentEventCount(t *testing.T, server *Server, incidentID string, eventType string, want int64) {
	t.Helper()

	var count int64
	if err := server.db.Model(&db.IncidentEvent{}).Where("incident_id = ? AND type = ?", incidentID, eventType).Count(&count).Error; err != nil {
		t.Fatalf("count incident events: %v", err)
	}
	if count != want {
		t.Fatalf("incident event count for %s = %d, want %d", eventType, count, want)
	}
}

func assertFrontendResponseDoesNotExposeAgentSecrets(t *testing.T, body string, token string) {
	t.Helper()

	if strings.Contains(body, token) {
		t.Fatalf("frontend response exposed agent token: %s", body)
	}
	if strings.Contains(body, `"token"`) {
		t.Fatalf("frontend response exposed token field: %s", body)
	}
	if strings.Contains(body, `"machine_id"`) {
		t.Fatalf("frontend response exposed machine_id field: %s", body)
	}
}

func assertNotContains(t *testing.T, body string, value string) {
	t.Helper()

	if strings.Contains(body, value) {
		t.Fatalf("response exposed %q: %s", value, body)
	}
}

func startAPITestSMTPServer(t *testing.T) (string, <-chan string) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen smtp: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	messages := make(chan string, 8)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()

				reader := bufio.NewReader(conn)
				_, _ = fmt.Fprint(conn, "220 orion-test-smtp\r\n")
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					command := strings.TrimRight(line, "\r\n")
					upperCommand := strings.ToUpper(command)
					switch {
					case strings.HasPrefix(upperCommand, "EHLO"), strings.HasPrefix(upperCommand, "HELO"):
						_, _ = fmt.Fprint(conn, "250 orion-test-smtp\r\n")
					case strings.HasPrefix(upperCommand, "MAIL FROM:"), strings.HasPrefix(upperCommand, "RCPT TO:"):
						_, _ = fmt.Fprint(conn, "250 ok\r\n")
					case upperCommand == "DATA":
						_, _ = fmt.Fprint(conn, "354 send message\r\n")
						var message strings.Builder
						for {
							dataLine, err := reader.ReadString('\n')
							if err != nil {
								return
							}
							if strings.TrimRight(dataLine, "\r\n") == "." {
								break
							}
							message.WriteString(dataLine)
						}
						messages <- message.String()
						_, _ = fmt.Fprint(conn, "250 queued\r\n")
					case upperCommand == "QUIT":
						_, _ = fmt.Fprint(conn, "221 bye\r\n")
						return
					default:
						_, _ = fmt.Fprint(conn, "250 ok\r\n")
					}
				}
			}()
		}
	}()

	return listener.Addr().String(), messages
}

func performJSONRequest(t *testing.T, server *Server, method string, path string, body interface{}, token string) *httptest.ResponseRecorder {
	t.Helper()

	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, req)
	return recorder
}

func performRawRequest(t *testing.T, server *Server, method string, path string, body io.Reader, token string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, req)
	return recorder
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target interface{}) {
	t.Helper()

	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
}
