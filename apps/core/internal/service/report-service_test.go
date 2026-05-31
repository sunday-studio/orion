package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestStoreMonitorReportRedactsSensitivePayloadBeforePersistence(t *testing.T) {
	database := openReportTestDatabase(t)
	logger := logging.NewLogger()
	insertReportServiceAgent(t, database, "agent_redaction")
	insertReportServiceMonitor(t, database, "monitor_redaction", "agent_redaction", "active")
	if err := database.Model(&db.Monitor{}).Where("id = ?", "monitor_redaction").Update("incident_state", "up").Error; err != nil {
		t.Fatalf("prime monitor incident state: %v", err)
	}

	reportService := NewReportService(database, logger, &config.Config{DataDir: t.TempDir()})
	reportService.SetDiagnostics(NewRuntimeDiagnosticsService(database, logger))
	secretValues := []string{
		"plain-token-value",
		"nested-password-value",
		"bearer-secret-value",
		"cookie-secret-value",
		"query-secret-value",
		"url-password-value",
	}

	reportID, err := reportService.StoreMonitorReport("monitor_redaction", MonitorReportPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Health:    "up",
		Metrics: map[string]interface{}{
			"type":          "api_request",
			"token":         "plain-token-value",
			"status_code":   200,
			"target_url":    "https://example.com/health?token=query-secret-value&ok=true",
			"credentialURL": "https://user:url-password-value@example.com/path",
			"headers": map[string]interface{}{
				"Authorization": "Bearer bearer-secret-value",
				"X-Trace-ID":    "trace-1",
			},
			"events": []interface{}{
				map[string]interface{}{"password": "nested-password-value"},
				"Set-Cookie: session=cookie-secret-value",
			},
		},
	})
	if err != nil {
		t.Fatalf("store report: %v", err)
	}

	var stored db.MonitorReport
	if err := database.Where("id = ?", *reportID).First(&stored).Error; err != nil {
		t.Fatalf("load stored report: %v", err)
	}
	for _, secretValue := range secretValues {
		if strings.Contains(stored.Payload, secretValue) {
			t.Fatalf("stored payload leaked %q: %s", secretValue, stored.Payload)
		}
	}
	if !strings.Contains(stored.Payload, monitorReportRedactedValue) {
		t.Fatalf("stored payload = %s, want redacted marker", stored.Payload)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(stored.Payload), &payload); err != nil {
		t.Fatalf("unmarshal stored payload: %v", err)
	}
	if payload["status_code"].(float64) != 200 {
		t.Fatalf("stored payload = %+v, want non-sensitive status_code retained", payload)
	}
}

func TestSafeMonitorReportPayloadRedactsExistingRows(t *testing.T) {
	jsonPayload := `{"type":"http","message":"failed with password=raw-password","nested":{"api_key":"raw-api-key"},"url":"https://example.com/path?token=raw-token"}`
	safePayload := SafeMonitorReportPayload(jsonPayload)

	for _, leaked := range []string{"raw-password", "raw-api-key", "raw-token"} {
		if strings.Contains(safePayload, leaked) {
			t.Fatalf("safe payload leaked %q: %s", leaked, safePayload)
		}
	}
	if !strings.Contains(safePayload, monitorReportRedactedValue) {
		t.Fatalf("safe payload = %s, want redacted marker", safePayload)
	}

	rawTextPayload := `authorization=Bearer raw-bearer-token cookie=session-cookie`
	safeTextPayload := SafeMonitorReportPayload(rawTextPayload)
	for _, leaked := range []string{"raw-bearer-token", "session-cookie"} {
		if strings.Contains(safeTextPayload, leaked) {
			t.Fatalf("safe text payload leaked %q: %s", leaked, safeTextPayload)
		}
	}
}

func TestGetMonitorUptimeUsesRollupsForArchivedDays(t *testing.T) {
	database := openReportTestDatabase(t)
	logger := logging.NewLogger()
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	dataDir := t.TempDir()

	settingsService := NewSettingsService(database, logger, dataDir)
	if _, err := settingsService.UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays:  1,
		ArchiveRawReports: true,
		ArchiveDir:        filepath.Join(dataDir, "archive"),
		RollupsEnabled:    true,
		ArchiveSchedule:   "manual",
	}); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	if err := database.Create(&db.MonitorUptimeRollup{
		MonitorID:     "monitor_a",
		Date:          "2026-05-11",
		UpCount:       8,
		DownCount:     2,
		TotalCount:    10,
		UptimePercent: 80,
	}).Error; err != nil {
		t.Fatalf("create rollup: %v", err)
	}
	insertReportServiceMonitorReport(t, database, "monitor_a", "up", time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC))
	insertReportServiceMonitorReport(t, database, "monitor_a", "down", time.Date(2026, 5, 12, 11, 0, 0, 0, time.UTC))

	reportService := NewReportService(database, logger, &config.Config{DataDir: dataDir})
	result, err := reportService.getMonitorUptime("monitor_a", "3d", now)
	if err != nil {
		t.Fatalf("getMonitorUptime() error = %v", err)
	}

	buckets := map[string]UptimeDayBucket{}
	for _, bucket := range result.DailyBuckets {
		buckets[bucket.Date] = bucket
	}
	if buckets["2026-05-11"].Up != 8 || buckets["2026-05-11"].Total != 10 {
		t.Fatalf("archived bucket = %+v, want rollup counts", buckets["2026-05-11"])
	}
	if buckets["2026-05-12"].Up != 1 || buckets["2026-05-12"].Total != 2 {
		t.Fatalf("hot bucket = %+v, want raw counts", buckets["2026-05-12"])
	}
	if result.UptimePercent != 75 {
		t.Fatalf("UptimePercent = %v, want 75", result.UptimePercent)
	}
}

func TestGetAgentUptimeAggregatesActiveMonitorBuckets(t *testing.T) {
	database := openReportTestDatabase(t)
	logger := logging.NewLogger()
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	dataDir := t.TempDir()

	settingsService := NewSettingsService(database, logger, dataDir)
	if _, err := settingsService.UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays:  30,
		ArchiveRawReports: true,
		ArchiveDir:        filepath.Join(dataDir, "archive"),
		RollupsEnabled:    true,
		ArchiveSchedule:   "manual",
	}); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	insertReportServiceAgent(t, database, "agent_a")
	insertReportServiceMonitor(t, database, "monitor_a", "agent_a", "active")
	insertReportServiceMonitor(t, database, "monitor_b", "agent_a", "active")
	insertReportServiceMonitor(t, database, "monitor_deleted", "agent_a", "deleted")
	insertReportServiceMonitorReport(t, database, "monitor_a", "up", time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC))
	insertReportServiceMonitorReport(t, database, "monitor_a", "down", time.Date(2026, 5, 12, 11, 0, 0, 0, time.UTC))
	insertReportServiceMonitorReport(t, database, "monitor_b", "up", time.Date(2026, 5, 12, 10, 30, 0, 0, time.UTC))
	insertReportServiceMonitorReport(t, database, "monitor_b", "up", time.Date(2026, 5, 12, 11, 30, 0, 0, time.UTC))
	insertReportServiceMonitorReport(t, database, "monitor_deleted", "down", time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC))

	reportService := NewReportService(database, logger, &config.Config{DataDir: dataDir})
	result, err := reportService.getAgentUptime("agent_a", "3d", now)
	if err != nil {
		t.Fatalf("getAgentUptime() error = %v", err)
	}

	buckets := map[string]UptimeDayBucket{}
	for _, bucket := range result.DailyBuckets {
		buckets[bucket.Date] = bucket
	}
	if buckets["2026-05-12"].Up != 3 || buckets["2026-05-12"].Total != 4 {
		t.Fatalf("agent bucket = %+v, want active monitor aggregate counts", buckets["2026-05-12"])
	}
	if result.UptimePercent != 75 {
		t.Fatalf("UptimePercent = %v, want 75", result.UptimePercent)
	}
}

func openReportTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return database
}

func insertReportServiceAgent(t *testing.T, database *gorm.DB, agentID string) {
	t.Helper()

	agent := db.Agent{
		ID:        agentID,
		MachineId: agentID + "-machine",
		Name:      agentID,
		OS:        "linux",
		Arch:      "arm64",
		Token:     agentID + "-token",
		LastSeen:  time.Now(),
	}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
}

func insertReportServiceMonitor(t *testing.T, database *gorm.DB, monitorID string, agentID string, lifecycle string) {
	t.Helper()

	monitor := db.Monitor{
		ID:        monitorID,
		AgentID:   agentID,
		Name:      monitorID,
		Type:      "http",
		Lifecycle: lifecycle,
		Health:    "unknown",
	}
	if err := database.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}
}

func insertReportServiceMonitorReport(t *testing.T, database *gorm.DB, monitorID string, health string, createdAt time.Time) {
	t.Helper()

	report := db.MonitorReport{
		ID:          utils.GenerateID("monitor_report"),
		MonitorID:   monitorID,
		Payload:     "{}",
		CollectedAt: createdAt.Format(time.RFC3339),
		Health:      health,
		CreatedAt:   createdAt,
	}
	if err := database.Create(&report).Error; err != nil {
		t.Fatalf("create monitor report: %v", err)
	}
}
