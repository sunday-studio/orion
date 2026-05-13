package service

import (
	"testing"
	"time"

	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetMonitorUptimeUsesRollupsForArchivedDays(t *testing.T) {
	database := openReportTestDatabase(t)
	logger := logging.NewLogger()
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)

	settingsService := NewSettingsService(database, logger, t.TempDir())
	if _, err := settingsService.UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays:  1,
		ArchiveRawReports: true,
		ArchiveDir:        t.TempDir(),
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

	reportService := NewReportService(database, logger, &config.Config{DataDir: t.TempDir()})
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
