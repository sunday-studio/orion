package service

import (
	"path/filepath"
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/utils"
)

func TestDataLifecycleSchedulerRunsDailyArchiveAndRollup(t *testing.T) {
	database := openArchiveTestDatabase(t)
	logger := utils.NewLogger()
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
	logger := utils.NewLogger()
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
	expiredPending := insertLifecycleSubscriber(t, database, lifecycleSubscriberSeed{
		ID:              "status_page_subscriber_manual_pending",
		StatusPageID:    "status_page_manual",
		State:           statusPageSubscriberStatePending,
		Email:           "manual-pending@example.com",
		ConfirmationDue: ptrTime(now.Add(-8 * 24 * time.Hour)),
		CreatedAt:       now.Add(-10 * 24 * time.Hour),
		UpdatedAt:       now.Add(-10 * 24 * time.Hour),
	})

	result, err := NewDataLifecycleSchedulerService(database, logger, dataDir, time.Hour).RunDue(now)
	if err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}
	if !result.SkippedManual || result.RollupRan || result.ArchiveRan || !result.SubscriberPrivacyRetentionRan || result.SubscriberPrivacyRetentionCount != 1 {
		t.Fatalf("RunDue() result = %+v, want manual archive skip with subscriber privacy retention", result)
	}

	assertArchiveReportExists(t, database, &db.MonitorReport{}, oldReportID)
	assertLifecycleSubscriberMissing(t, database, expiredPending.ID)
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
	logger := utils.NewLogger()
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
	if err := database.Model(&db.DataLifecycleSettings{}).Where("id = ?", 1).Updates(map[string]any{
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

func TestDataLifecycleSchedulerBackfillsOnEveryTickNotOncePerDay(t *testing.T) {
	database := openArchiveTestDatabase(t)
	logger := utils.NewLogger()
	dataDir := t.TempDir()
	now := time.Date(2026, 5, 27, 9, 0, 0, 0, time.UTC)
	if _, err := NewSettingsService(database, logger, dataDir).UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays: 30, RollupsEnabled: true, ArchiveSchedule: "daily",
	}); err != nil {
		t.Fatal(err)
	}
	for back := 1; back <= rollupBackfillMaxDaysPerRun+4; back++ {
		insertArchiveMonitorReport(t, database, "monitor_a", "up", now.AddDate(0, 0, -back).Add(time.Hour))
	}
	scheduler := NewDataLifecycleSchedulerService(database, logger, dataDir, time.Hour)

	first, err := scheduler.RunDue(now)
	if err != nil || !first.RollupRan || first.RollupBackfilledDays == 0 {
		t.Fatalf("first tick = %+v, %v; want the daily rollup and a backfill batch", first, err)
	}
	// Same day: the daily rollup is done, but the remaining history keeps catching up.
	second, err := scheduler.RunDue(now.Add(time.Hour))
	if err != nil || second.RollupRan || second.RollupBackfilledDays == 0 {
		t.Fatalf("second tick = %+v, %v; want no daily rollup but more backfill", second, err)
	}
	third, err := scheduler.RunDue(now.Add(2 * time.Hour))
	if err != nil || third.RollupBackfilledDays != 0 {
		t.Fatalf("third tick = %+v, %v; want nothing left to backfill", third, err)
	}
}

func TestDataLifecycleSchedulerDoesNotBackfillWhenRollupsAreOff(t *testing.T) {
	database := openArchiveTestDatabase(t)
	logger := utils.NewLogger()
	dataDir := t.TempDir()
	now := time.Date(2026, 5, 27, 9, 0, 0, 0, time.UTC)
	if _, err := NewSettingsService(database, logger, dataDir).UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays: 30, RollupsEnabled: false, ArchiveSchedule: "daily",
	}); err != nil {
		t.Fatal(err)
	}
	insertArchiveMonitorReport(t, database, "monitor_a", "up", now.AddDate(0, 0, -3))
	result, err := NewDataLifecycleSchedulerService(database, logger, dataDir, time.Hour).RunDue(now)
	if err != nil || result.RollupRan || result.RollupBackfilledDays != 0 {
		t.Fatalf("result = %+v, %v; want no rollup work", result, err)
	}
}
