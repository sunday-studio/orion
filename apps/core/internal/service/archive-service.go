package service

import (
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ArchiveRunResult struct {
	ArchivePath             string `json:"archive_path"`
	Cutoff                  string `json:"cutoff"`
	AgentReportsArchived    int    `json:"agent_reports_archived"`
	MonitorReportsArchived  int    `json:"monitor_reports_archived"`
	ArchiveRawReports       bool   `json:"archive_raw_reports"`
	SkippedBecauseDisabled  bool   `json:"skipped_because_disabled"`
	SkippedBecauseNoReports bool   `json:"skipped_because_no_reports"`
}

type ArchiveService struct {
	db      *gorm.DB
	logger  *logging.Logger
	dataDir string
}

func NewArchiveService(database *gorm.DB, logger *logging.Logger, dataDir string) *ArchiveService {
	return &ArchiveService{
		db:      database,
		logger:  logger,
		dataDir: dataDir,
	}
}

func (s *ArchiveService) RunRawReportArchive(now time.Time) (*ArchiveRunResult, error) {
	settingsService := NewSettingsService(s.db, s.logger, s.dataDir)
	settings, err := settingsService.GetDataLifecycleSettings()
	if err != nil {
		return nil, err
	}

	result := &ArchiveRunResult{
		Cutoff:            now.AddDate(0, 0, -settings.RawReportHotDays).Format(time.RFC3339),
		ArchiveRawReports: settings.ArchiveRawReports,
	}
	if !settings.ArchiveRawReports {
		result.SkippedBecauseDisabled = true
		if err := s.recordArchiveRun(now, "suppressed", ""); err != nil {
			return nil, err
		}
		return result, nil
	}

	archivePath, err := s.archivePath(settings.ArchiveDir, now)
	if err != nil {
		_ = s.recordArchiveRun(now, "failed", err.Error())
		return nil, err
	}
	result.ArchivePath = archivePath

	archiveDB, err := openArchiveDatabase(archivePath)
	if err != nil {
		_ = s.recordArchiveRun(now, "failed", err.Error())
		return nil, err
	}

	cutoff := now.AddDate(0, 0, -settings.RawReportHotDays)
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var agentReports []db.AgentReport
		if err := tx.Where("created_at < ?", cutoff).Order("created_at ASC").Find(&agentReports).Error; err != nil {
			return err
		}

		var monitorReports []db.MonitorReport
		if err := tx.Where("created_at < ?", cutoff).Order("created_at ASC").Find(&monitorReports).Error; err != nil {
			return err
		}

		if len(agentReports) == 0 && len(monitorReports) == 0 {
			result.SkippedBecauseNoReports = true
			return nil
		}

		if len(agentReports) > 0 {
			if err := archiveDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&agentReports).Error; err != nil {
				return err
			}
			ids := make([]string, 0, len(agentReports))
			for _, report := range agentReports {
				ids = append(ids, report.ID)
			}
			if err := tx.Where("id IN ?", ids).Delete(&db.AgentReport{}).Error; err != nil {
				return err
			}
			result.AgentReportsArchived = len(agentReports)
		}

		if len(monitorReports) > 0 {
			if err := archiveDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&monitorReports).Error; err != nil {
				return err
			}
			ids := make([]string, 0, len(monitorReports))
			for _, report := range monitorReports {
				ids = append(ids, report.ID)
			}
			if err := tx.Where("id IN ?", ids).Delete(&db.MonitorReport{}).Error; err != nil {
				return err
			}
			result.MonitorReportsArchived = len(monitorReports)
		}

		return nil
	}); err != nil {
		_ = s.recordArchiveRun(now, "failed", err.Error())
		s.logger.Error("Failed to archive raw reports", "error", err)
		return nil, err
	}

	if err := s.recordArchiveRun(now, "success", ""); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ArchiveService) archivePath(archiveDir string, now time.Time) (string, error) {
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(archiveDir, "raw-reports-"+now.Format("2006-01")+".sqlite"), nil
}

func (s *ArchiveService) recordArchiveRun(now time.Time, status string, message string) error {
	return s.db.Model(&db.DataLifecycleSettings{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"last_archive_run_at": now,
		"last_archive_status": status,
		"last_archive_error":  message,
	}).Error
}

func openArchiveDatabase(path string) (*gorm.DB, error) {
	archiveDB, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := archiveDB.AutoMigrate(&db.AgentReport{}, &db.MonitorReport{}); err != nil {
		return nil, err
	}
	return archiveDB, nil
}
