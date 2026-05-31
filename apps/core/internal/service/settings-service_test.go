package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	if settings.ArchiveDir != defaultArchiveDir(dataDir) {
		t.Fatalf("ArchiveDir = %q", settings.ArchiveDir)
	}
}

func TestSettingsServiceUpdatesDataLifecycleSettings(t *testing.T) {
	database := openSettingsTestDatabase(t)
	dataDir := t.TempDir()
	service := NewSettingsService(database, logging.NewLogger(), dataDir)
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
	wantArchiveDir, err := normalizeArchiveDir(dataDir, "data/archive")
	if err != nil {
		t.Fatalf("normalize archive dir: %v", err)
	}
	if settings.ArchiveDir != wantArchiveDir {
		t.Fatalf("ArchiveDir = %q, want normalized data-dir path", settings.ArchiveDir)
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

func TestSettingsServiceRejectsUnsafeArchiveDirectories(t *testing.T) {
	dataDir := t.TempDir()
	externalDir := t.TempDir()
	tests := []struct {
		name       string
		archiveDir string
		wantErr    error
	}{
		{
			name:       "traversal outside data dir",
			archiveDir: "../outside",
			wantErr:    errArchiveDirOutsideDataDir,
		},
		{
			name:       "absolute external path",
			archiveDir: externalDir,
			wantErr:    errArchiveDirOutsideDataDir,
		},
		{
			name:       "data dir itself",
			archiveDir: dataDir,
			wantErr:    errArchiveDirOutsideDataDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := openSettingsTestDatabase(t)
			service := NewSettingsService(database, logging.NewLogger(), dataDir)

			_, err := service.UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
				RawReportHotDays:  90,
				ArchiveRawReports: true,
				ArchiveDir:        tt.archiveDir,
				RollupsEnabled:    true,
				ArchiveSchedule:   "daily",
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("UpdateDataLifecycleSettings() error = %v, want %v", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), externalDir) || strings.Contains(err.Error(), dataDir) {
				t.Fatalf("validation error leaked filesystem path: %q", err.Error())
			}
		})
	}
}

func TestSettingsServiceRejectsSymlinkArchiveEscape(t *testing.T) {
	dataDir := t.TempDir()
	externalDir := t.TempDir()
	linkPath := filepath.Join(dataDir, "external-link")
	if err := os.Symlink(externalDir, linkPath); err != nil {
		t.Skipf("create symlink: %v", err)
	}

	database := openSettingsTestDatabase(t)
	service := NewSettingsService(database, logging.NewLogger(), dataDir)

	_, err := service.UpdateDataLifecycleSettings(DataLifecycleSettingsPayload{
		RawReportHotDays:  90,
		ArchiveRawReports: true,
		ArchiveDir:        filepath.Join(linkPath, "archive"),
		RollupsEnabled:    true,
		ArchiveSchedule:   "daily",
	})
	if !errors.Is(err, errArchiveDirOutsideDataDir) {
		t.Fatalf("UpdateDataLifecycleSettings() error = %v, want %v", err, errArchiveDirOutsideDataDir)
	}
}

func TestSettingsServiceRejectsInvalidLifecycleValues(t *testing.T) {
	tooLong := maxDataLifecycleRetentionDays + 1
	tests := []struct {
		name    string
		payload DataLifecycleSettingsPayload
		wantErr string
	}{
		{
			name: "empty archive dir when archiving enabled",
			payload: DataLifecycleSettingsPayload{
				RawReportHotDays:  90,
				ArchiveRawReports: true,
				RollupsEnabled:    true,
				ArchiveSchedule:   "daily",
			},
			wantErr: errArchiveDirRequired.Error(),
		},
		{
			name: "invalid schedule",
			payload: DataLifecycleSettingsPayload{
				RawReportHotDays:  90,
				ArchiveRawReports: true,
				ArchiveDir:        "archive",
				RollupsEnabled:    true,
				ArchiveSchedule:   "weekly",
			},
			wantErr: "archive_schedule must be daily or manual",
		},
		{
			name: "excessive hot retention",
			payload: DataLifecycleSettingsPayload{
				RawReportHotDays:  tooLong,
				ArchiveRawReports: true,
				ArchiveDir:        "archive",
				RollupsEnabled:    true,
				ArchiveSchedule:   "daily",
			},
			wantErr: "raw_report_hot_days must be <= 3650",
		},
		{
			name: "excessive rollup retention",
			payload: DataLifecycleSettingsPayload{
				RawReportHotDays:    90,
				ArchiveRawReports:   true,
				ArchiveDir:          "archive",
				RollupsEnabled:      true,
				RollupRetentionDays: &tooLong,
				ArchiveSchedule:     "daily",
			},
			wantErr: "rollup_retention_days must be <= 3650 or null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := openSettingsTestDatabase(t)
			service := NewSettingsService(database, logging.NewLogger(), t.TempDir())

			_, err := service.UpdateDataLifecycleSettings(tt.payload)
			if err == nil {
				t.Fatal("UpdateDataLifecycleSettings() error = nil, want validation error")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("UpdateDataLifecycleSettings() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
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
