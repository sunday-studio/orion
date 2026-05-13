package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

type DataLifecycleSettingsPayload struct {
	RawReportHotDays    int    `json:"raw_report_hot_days"`
	ArchiveRawReports   bool   `json:"archive_raw_reports"`
	ArchiveDir          string `json:"archive_dir"`
	RollupsEnabled      bool   `json:"rollups_enabled"`
	RollupRetentionDays *int   `json:"rollup_retention_days"`
	ArchiveSchedule     string `json:"archive_schedule"`
}

type SettingsService struct {
	db      *gorm.DB
	logger  *logging.Logger
	dataDir string
}

func NewSettingsService(database *gorm.DB, logger *logging.Logger, dataDir string) *SettingsService {
	return &SettingsService{
		db:      database,
		logger:  logger,
		dataDir: dataDir,
	}
}

func (s *SettingsService) GetDataLifecycleSettings() (*db.DataLifecycleSettings, error) {
	settings, err := s.ensureDataLifecycleDefaults()
	if err != nil {
		return nil, err
	}
	return settings, nil
}

func (s *SettingsService) UpdateDataLifecycleSettings(payload DataLifecycleSettingsPayload) (*db.DataLifecycleSettings, error) {
	if err := s.validateDataLifecyclePayload(payload); err != nil {
		return nil, err
	}

	settings, err := s.ensureDataLifecycleDefaults()
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"raw_report_hot_days":   payload.RawReportHotDays,
		"archive_raw_reports":   payload.ArchiveRawReports,
		"archive_dir":           payload.ArchiveDir,
		"rollups_enabled":       payload.RollupsEnabled,
		"rollup_retention_days": payload.RollupRetentionDays,
		"archive_schedule":      payload.ArchiveSchedule,
	}
	if err := s.db.Model(settings).Updates(updates).Error; err != nil {
		return nil, err
	}

	return s.GetDataLifecycleSettings()
}

func (s *SettingsService) ensureDataLifecycleDefaults() (*db.DataLifecycleSettings, error) {
	var settings db.DataLifecycleSettings
	err := s.db.Where("id = ?", 1).First(&settings).Error
	if err == nil {
		return &settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	settings = db.DataLifecycleSettings{
		ID:                1,
		RawReportHotDays:  90,
		ArchiveRawReports: true,
		ArchiveDir:        filepath.Join(s.dataDir, "archive"),
		RollupsEnabled:    true,
		ArchiveSchedule:   "daily",
	}
	if err := s.db.Create(&settings).Error; err != nil {
		return nil, err
	}
	s.logger.Info("Created default data lifecycle settings")
	return &settings, nil
}

func (s *SettingsService) validateDataLifecyclePayload(payload DataLifecycleSettingsPayload) error {
	if payload.RawReportHotDays < 1 {
		return errors.New("raw_report_hot_days must be >= 1")
	}
	if payload.ArchiveRawReports && strings.TrimSpace(payload.ArchiveDir) == "" {
		return errors.New("archive_dir is required when archive_raw_reports is enabled")
	}
	if payload.ArchiveRawReports && !payload.RollupsEnabled {
		return errors.New("rollups_enabled is required when archive_raw_reports is enabled")
	}
	if payload.RollupRetentionDays != nil && *payload.RollupRetentionDays < 1 {
		return errors.New("rollup_retention_days must be >= 1 or null")
	}
	switch payload.ArchiveSchedule {
	case "daily", "manual":
		return nil
	default:
		return fmt.Errorf("archive_schedule must be daily or manual")
	}
}
