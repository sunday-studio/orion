package service

import (
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRollupMonitorUptimeDayCreatesDailyRows(t *testing.T) {
	database := openRollupTestDatabase(t)
	service := NewRollupService(database, logging.NewLogger())
	day := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	insertRollupReport(t, database, "monitor_a", "up", day.Add(time.Hour))
	insertRollupReport(t, database, "monitor_a", "down", day.Add(2*time.Hour))
	insertRollupReport(t, database, "monitor_a", "degraded", day.Add(3*time.Hour))
	insertRollupReport(t, database, "monitor_b", "up", day.Add(4*time.Hour))
	insertRollupReport(t, database, "monitor_a", "up", day.AddDate(0, 0, 1))

	result, err := service.RollupMonitorUptimeDay(day)
	if err != nil {
		t.Fatalf("RollupMonitorUptimeDay() error = %v", err)
	}

	if result.MonitorDays != 2 || result.ReportCount != 4 {
		t.Fatalf("result = %+v, want 2 monitor days and 4 reports", result)
	}

	var rollup db.MonitorUptimeRollup
	if err := database.Where("monitor_id = ? AND date = ?", "monitor_a", "2026-05-12").First(&rollup).Error; err != nil {
		t.Fatalf("find rollup: %v", err)
	}
	if rollup.UpCount != 1 || rollup.DownCount != 1 || rollup.DegradedCount != 1 || rollup.TotalCount != 3 {
		t.Fatalf("rollup = %+v, want one up, down, degraded and three total", rollup)
	}
	if rollup.UptimePercent != 100.0/3.0 {
		t.Fatalf("UptimePercent = %v, want %v", rollup.UptimePercent, 100.0/3.0)
	}
}

func TestRollupMonitorUptimeDayIsIdempotent(t *testing.T) {
	database := openRollupTestDatabase(t)
	service := NewRollupService(database, logging.NewLogger())
	day := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	insertRollupReport(t, database, "monitor_a", "up", day)
	if _, err := service.RollupMonitorUptimeDay(day); err != nil {
		t.Fatalf("first RollupMonitorUptimeDay() error = %v", err)
	}
	insertRollupReport(t, database, "monitor_a", "down", day.Add(time.Hour))
	if _, err := service.RollupMonitorUptimeDay(day); err != nil {
		t.Fatalf("second RollupMonitorUptimeDay() error = %v", err)
	}

	var count int64
	if err := database.Model(&db.MonitorUptimeRollup{}).Count(&count).Error; err != nil {
		t.Fatalf("count rollups: %v", err)
	}
	if count != 1 {
		t.Fatalf("rollup count = %d, want 1", count)
	}

	var rollup db.MonitorUptimeRollup
	if err := database.First(&rollup).Error; err != nil {
		t.Fatalf("find rollup: %v", err)
	}
	if rollup.UpCount != 1 || rollup.DownCount != 1 || rollup.TotalCount != 2 {
		t.Fatalf("rollup = %+v, want updated counts", rollup)
	}
}

func openRollupTestDatabase(t *testing.T) *gorm.DB {
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

func insertRollupReport(t *testing.T, database *gorm.DB, monitorID string, health string, createdAt time.Time) {
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
