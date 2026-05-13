package service

import (
	"path/filepath"
	"testing"

	"orion/core/internal/db"
	"orion/core/internal/logging"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSettingsServiceCreatesDataLifecycleDefaults(t *testing.T) {
	database := openSettingsTestDatabase(t)
	dataDir := filepath.Join(t.TempDir(), "data")
	service := NewSettingsService(database, logging.NewLogger(), dataDir)

	settings, err := service.GetDataLifecycleSettings()
	if err != nil {
		t.Fatalf("GetDataLifecycleSettings() error = %v", err)
	}

	if settings.RawReportHotDays != 90 {
		t.Fatalf("RawReportHotDays = %d, want 90", settings.RawReportHotDays)
	}
	if !settings.ArchiveRawReports || !settings.RollupsEnabled {
		t.Fatalf("settings = %+v, want archiving and rollups enabled", settings)
	}
	if settings.ArchiveDir != filepath.Join(dataDir, "archive") {
		t.Fatalf("ArchiveDir = %q", settings.ArchiveDir)
	}
}

func TestSettingsServiceUpdatesDataLifecycleSettings(t *testing.T) {
	database := openSettingsTestDatabase(t)
	service := NewSettingsService(database, logging.NewLogger(), t.TempDir())
	retentionDays := 365

	settings, err := service.UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays:    120,
		ArchiveRawReports:   true,
		ArchiveDir:          "data/archive",
		RollupsEnabled:      true,
		RollupRetentionDays: &retentionDays,
		ArchiveSchedule:     "manual",
	})
	if err != nil {
		t.Fatalf("UpdateDataLifecycleSettings() error = %v", err)
	}

	if settings.RawReportHotDays != 120 || settings.ArchiveSchedule != "manual" {
		t.Fatalf("settings = %+v, want updated values", settings)
	}
	if settings.RollupRetentionDays == nil || *settings.RollupRetentionDays != retentionDays {
		t.Fatalf("RollupRetentionDays = %+v, want %d", settings.RollupRetentionDays, retentionDays)
	}
}

func TestSettingsServiceRejectsArchivingWithoutRollups(t *testing.T) {
	database := openSettingsTestDatabase(t)
	service := NewSettingsService(database, logging.NewLogger(), t.TempDir())

	_, err := service.UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays:  90,
		ArchiveRawReports: true,
		ArchiveDir:        "data/archive",
		RollupsEnabled:    false,
		ArchiveSchedule:   "daily",
	})
	if err == nil {
		t.Fatal("UpdateDataLifecycleSettings() error = nil, want validation error")
	}
}

func openSettingsTestDatabase(t *testing.T) *gorm.DB {
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
