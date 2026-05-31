package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunRawReportArchiveMovesOldReportsToArchiveDatabase(t *testing.T) {
	database := openArchiveTestDatabase(t)
	logger := logging.NewLogger()
	dataDir := t.TempDir()
	archiveDir := filepath.Join(dataDir, "archive")
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)

	settingsService := NewSettingsService(database, logger, dataDir)
	if _, err := settingsService.UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays:  30,
		ArchiveRawReports: true,
		ArchiveDir:        archiveDir,
		RollupsEnabled:    true,
		ArchiveSchedule:   "manual",
	}); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	oldAgentID := insertArchiveAgentReport(t, database, "agent_a", now.AddDate(0, 0, -31))
	newAgentID := insertArchiveAgentReport(t, database, "agent_a", now.AddDate(0, 0, -2))
	oldMonitorID := insertArchiveMonitorReport(t, database, "monitor_a", "up", now.AddDate(0, 0, -31))
	newMonitorID := insertArchiveMonitorReport(t, database, "monitor_a", "down", now.AddDate(0, 0, -2))

	result, err := NewArchiveService(database, logger, dataDir).RunRawReportArchive(now)
	if err != nil {
		t.Fatalf("RunRawReportArchive() error = %v", err)
	}

	if result.AgentReportsArchived != 1 || result.MonitorReportsArchived != 1 {
		t.Fatalf("result = %+v, want one agent and one monitor report archived", result)
	}
	assertArchiveReportMissing(t, database, &db.AgentReport{}, oldAgentID)
	assertArchiveReportExists(t, database, &db.AgentReport{}, newAgentID)
	assertArchiveReportMissing(t, database, &db.MonitorReport{}, oldMonitorID)
	assertArchiveReportExists(t, database, &db.MonitorReport{}, newMonitorID)

	archiveDB, err := openArchiveDatabase(result.ArchivePath)
	if err != nil {
		t.Fatalf("open archive database: %v", err)
	}
	assertArchiveReportExists(t, archiveDB, &db.AgentReport{}, oldAgentID)
	assertArchiveReportExists(t, archiveDB, &db.MonitorReport{}, oldMonitorID)

	var settings db.DataLifecycleSettings
	if err := database.First(&settings, 1).Error; err != nil {
		t.Fatalf("find settings: %v", err)
	}
	if settings.LastArchiveRunAt == nil || settings.LastArchiveStatus != "success" {
		t.Fatalf("settings archive state = %+v, want success with run timestamp", settings)
	}
}

func TestRunRawReportArchiveSkipsWhenDisabled(t *testing.T) {
	database := openArchiveTestDatabase(t)
	logger := logging.NewLogger()
	dataDir := t.TempDir()
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)

	settingsService := NewSettingsService(database, logger, dataDir)
	if _, err := settingsService.UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays:  30,
		ArchiveRawReports: false,
		ArchiveDir:        "",
		RollupsEnabled:    true,
		ArchiveSchedule:   "manual",
	}); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	oldReportID := insertArchiveMonitorReport(t, database, "monitor_a", "up", now.AddDate(0, 0, -31))

	result, err := NewArchiveService(database, logger, dataDir).RunRawReportArchive(now)
	if err != nil {
		t.Fatalf("RunRawReportArchive() error = %v", err)
	}

	if !result.SkippedBecauseDisabled {
		t.Fatalf("SkippedBecauseDisabled = false, want true")
	}
	assertArchiveReportExists(t, database, &db.MonitorReport{}, oldReportID)
}

func TestRunRawReportArchiveRejectsSymlinkEscapingDataDir(t *testing.T) {
	database := openArchiveTestDatabase(t)
	logger := logging.NewLogger()
	dataDir := t.TempDir()
	externalDir := t.TempDir()
	archiveLink := filepath.Join(dataDir, "archive-link")
	if err := os.Symlink(externalDir, archiveLink); err != nil {
		t.Skipf("create symlink: %v", err)
	}

	settings := db.DataLifecycleSettings{
		ID:                1,
		RawReportHotDays:  30,
		ArchiveRawReports: true,
		ArchiveDir:        archiveLink,
		RollupsEnabled:    true,
		ArchiveSchedule:   "manual",
	}
	if err := database.Create(&settings).Error; err != nil {
		t.Fatalf("create settings: %v", err)
	}
	oldReportID := insertArchiveMonitorReport(t, database, "monitor_a", "up", time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC))

	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	_, err := NewArchiveService(database, logger, dataDir).RunRawReportArchive(now)
	if !errors.Is(err, errArchiveDirOutsideDataDir) {
		t.Fatalf("RunRawReportArchive() error = %v, want archive directory policy error", err)
	}
	assertArchiveReportExists(t, database, &db.MonitorReport{}, oldReportID)
	if err := database.First(&settings, 1).Error; err != nil {
		t.Fatalf("find settings: %v", err)
	}
	if settings.LastArchiveStatus != "failed" {
		t.Fatalf("LastArchiveStatus = %q, want failed", settings.LastArchiveStatus)
	}
	if _, err := os.Stat(filepath.Join(externalDir, "raw-reports-2026-05.sqlite")); !os.IsNotExist(err) {
		t.Fatalf("archive file escaped data dir, stat err = %v", err)
	}
}

func openArchiveTestDatabase(t *testing.T) *gorm.DB {
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

func insertArchiveAgentReport(t *testing.T, database *gorm.DB, agentID string, createdAt time.Time) string {
	t.Helper()

	report := db.AgentReport{
		ID:            utils.GenerateID("agent_report"),
		AgentID:       agentID,
		ConfigSummary: "{}",
		Timestamp:     createdAt.Format(time.RFC3339),
		CreatedAt:     createdAt,
	}
	if err := database.Create(&report).Error; err != nil {
		t.Fatalf("create agent report: %v", err)
	}
	return report.ID
}

func insertArchiveMonitorReport(t *testing.T, database *gorm.DB, monitorID string, health string, createdAt time.Time) string {
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
	return report.ID
}

func assertArchiveReportExists(t *testing.T, database *gorm.DB, model interface{}, id string) {
	t.Helper()

	var count int64
	if err := database.Model(model).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatalf("count report %s: %v", id, err)
	}
	if count != 1 {
		t.Fatalf("report %s count = %d, want 1", id, count)
	}
}

func assertArchiveReportMissing(t *testing.T, database *gorm.DB, model interface{}, id string) {
	t.Helper()

	var count int64
	if err := database.Model(model).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatalf("count report %s: %v", id, err)
	}
	if count != 0 {
		t.Fatalf("report %s count = %d, want 0", id, count)
	}
}
