package service

import (
	"errors"
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

var (
	errArchiveStorageUnavailable = errors.New("failed to prepare archive storage")
	errArchiveMoveFailed         = errors.New("failed to move old raw reports")
)

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
		_ = s.recordArchiveRun(now, "failed", errArchiveStorageUnavailable.Error())
		s.logger.Error("Failed to open archive database", "error", err)
		return nil, errArchiveStorageUnavailable
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
		_ = s.recordArchiveRun(now, "failed", errArchiveMoveFailed.Error())
		s.logger.Error("Failed to archive raw reports", "error", err)
		return nil, errArchiveMoveFailed
	}

	if err := s.recordArchiveRun(now, "success", ""); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ArchiveService) archivePath(archiveDir string, now time.Time) (string, error) {
	safeArchiveDir, err := s.prepareArchiveDir(archiveDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(safeArchiveDir, "raw-reports-"+now.Format("2006-01")+".sqlite"), nil
}

func (s *ArchiveService) prepareArchiveDir(archiveDir string) (string, error) {
	normalizedArchiveDir, err := normalizeArchiveDir(s.dataDir, archiveDir)
	if err != nil {
		return "", err
	}

	dataRoot, err := cleanAbsPath(s.dataDir)
	if err != nil {
		return "", errCoreDataDirInvalid
	}
	if err := os.MkdirAll(dataRoot, 0o755); err != nil {
		return "", errArchiveStorageUnavailable
	}

	resolvedDataRoot, err := filepath.EvalSymlinks(dataRoot)
	if err != nil {
		return "", errArchiveStorageUnavailable
	}
	resolvedDataRoot = filepath.Clean(resolvedDataRoot)

	existingAncestor, err := nearestExistingPath(normalizedArchiveDir)
	if err != nil {
		return "", errArchiveStorageUnavailable
	}
	resolvedExistingAncestor, err := filepath.EvalSymlinks(existingAncestor)
	if err != nil {
		return "", errArchiveStorageUnavailable
	}
	resolvedExistingAncestor = filepath.Clean(resolvedExistingAncestor)
	if resolvedExistingAncestor != resolvedDataRoot && !pathStrictlyInside(resolvedDataRoot, resolvedExistingAncestor) {
		return "", errArchiveDirOutsideDataDir
	}

	if err := os.MkdirAll(normalizedArchiveDir, 0o755); err != nil {
		return "", errArchiveStorageUnavailable
	}
	resolvedArchiveDir, err := filepath.EvalSymlinks(normalizedArchiveDir)
	if err != nil {
		return "", errArchiveStorageUnavailable
	}
	resolvedArchiveDir = filepath.Clean(resolvedArchiveDir)
	if !pathStrictlyInside(resolvedDataRoot, resolvedArchiveDir) {
		return "", errArchiveDirOutsideDataDir
	}

	return normalizedArchiveDir, nil
}

func nearestExistingPath(path string) (string, error) {
	path = filepath.Clean(path)
	for {
		if _, err := os.Lstat(path); err == nil {
			return path, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(path)
		if parent == path {
			return "", os.ErrNotExist
		}
		path = parent
	}
}

func (s *ArchiveService) recordArchiveRun(now time.Time, status string, message string) error {
	return s.db.Model(&db.DataLifecycleSettings{}).Where("id = ?", 1).Updates(map[string]any{
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
