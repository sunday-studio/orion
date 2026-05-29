package service

import (
	"path/filepath"
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/logging"
)

func TestDataLifecycleSchedulerRunsDailyArchiveAndRollup(t *testing.T) {
	database := openArchiveTestDatabase(t)
	logger := logging.NewLogger()
	dataDir := t.TempDir()
	archiveDir := filepath.Join(dataDir, "archive")
	now := time.Date(2026, 5, 27, 9, 0, 0, 0, time.UTC)

	if _, err := NewSettingsService(database, logger, dataDir).UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays:  30,
		ArchiveRawReports: true,
		ArchiveDir:        archiveDir,
		RollupsEnabled:    true,
		ArchiveSchedule:   "daily",
	}); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	oldReportID := insertArchiveMonitorReport(t, database, "monitor_old", "down", now.AddDate(0, 0, -31))
	insertArchiveMonitorReport(t, database, "monitor_yesterday", "up", now.AddDate(0, 0, -1).Add(time.Hour))

	result, err := NewDataLifecycleSchedulerService(database, logger, dataDir, time.Hour).RunDue(now)
	if err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	if !result.RollupRan || !result.ArchiveRan || result.SkippedManual {
		t.Fatalf("RunDue() result = %+v, want rollup and archive", result)
	}

	var rollup db.MonitorUptimeRollup
	if err := database.Where("monitor_id = ? AND date = ?", "monitor_yesterday", "2026-05-26").First(&rollup).Error; err != nil {
		t.Fatalf("find rollup: %v", err)
	}
	if rollup.UpCount != 1 || rollup.TotalCount != 1 {
		t.Fatalf("rollup = %+v, want one up report", rollup)
	}
	assertArchiveReportMissing(t, database, &db.MonitorReport{}, oldReportID)

	var settings db.DataLifecycleSettings
	if err := database.First(&settings, 1).Error; err != nil {
		t.Fatalf("find settings: %v", err)
	}
	if settings.LastRollupRunAt == nil || settings.LastArchiveRunAt == nil || settings.LastArchiveStatus != "success" {
		t.Fatalf("settings = %+v, want rollup and archive metadata", settings)
	}
}

func TestDataLifecycleSchedulerSkipsManualMode(t *testing.T) {
	database := openArchiveTestDatabase(t)
	logger := logging.NewLogger()
	dataDir := t.TempDir()
	now := time.Date(2026, 5, 27, 9, 0, 0, 0, time.UTC)

	if _, err := NewSettingsService(database, logger, dataDir).UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays:  30,
		ArchiveRawReports: true,
		ArchiveDir:        filepath.Join(dataDir, "archive"),
		RollupsEnabled:    true,
		ArchiveSchedule:   "manual",
	}); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	oldReportID := insertArchiveMonitorReport(t, database, "monitor_manual", "down", now.AddDate(0, 0, -31))

	result, err := NewDataLifecycleSchedulerService(database, logger, dataDir, time.Hour).RunDue(now)
	if err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	if !result.SkippedManual || result.RollupRan || result.ArchiveRan {
		t.Fatalf("RunDue() result = %+v, want manual skip", result)
	}

	assertArchiveReportExists(t, database, &db.MonitorReport{}, oldReportID)
	var settings db.DataLifecycleSettings
	if err := database.First(&settings, 1).Error; err != nil {
		t.Fatalf("find settings: %v", err)
	}
	if settings.LastRollupRunAt != nil || settings.LastArchiveRunAt != nil {
		t.Fatalf("settings = %+v, want no scheduler metadata in manual mode", settings)
	}
}

func TestDataLifecycleSchedulerRunsOncePerDay(t *testing.T) {
	database := openArchiveTestDatabase(t)
	logger := logging.NewLogger()
	dataDir := t.TempDir()
	now := time.Date(2026, 5, 27, 9, 0, 0, 0, time.UTC)

	if _, err := NewSettingsService(database, logger, dataDir).UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays:  30,
		ArchiveRawReports: true,
		ArchiveDir:        filepath.Join(dataDir, "archive"),
		RollupsEnabled:    true,
		ArchiveSchedule:   "daily",
	}); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if err := database.Model(&db.DataLifecycleSettings{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"last_rollup_run_at":  now.Add(-time.Hour),
		"last_archive_run_at": now.Add(-time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed last run metadata: %v", err)
	}

	result, err := NewDataLifecycleSchedulerService(database, logger, dataDir, time.Hour).RunDue(now)
	if err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	if result.RollupRan || result.ArchiveRan || result.SkippedManual {
		t.Fatalf("RunDue() result = %+v, want no duplicate daily run", result)
	}
}
