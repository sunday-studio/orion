package service

import (
	"errors"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

const maxDataLifecycleRetentionDays = 3650

var (
	errArchiveDirRequired       = errors.New("archive_dir is required when archive_raw_reports is enabled")
	errArchiveDirOutsideDataDir = errors.New("archive_dir must stay inside the Core data directory")
	errArchiveDirInvalid        = errors.New("archive_dir is invalid")
	errCoreDataDirInvalid       = errors.New("Core data directory is invalid")
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
	archiveDir, err := s.validateDataLifecyclePayload(payload)
	if err != nil {
		return nil, err
	}

	settings, err := s.ensureDataLifecycleDefaults()
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"raw_report_hot_days":   payload.RawReportHotDays,
		"archive_raw_reports":   payload.ArchiveRawReports,
		"archive_dir":           archiveDir,
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
		ArchiveDir:        defaultArchiveDir(s.dataDir),
		RollupsEnabled:    true,
		ArchiveSchedule:   "daily",
	}
	if err := s.db.Create(&settings).Error; err != nil {
		return nil, err
	}
	s.logger.Info("Created default data lifecycle settings")
	return &settings, nil
}

func (s *SettingsService) validateDataLifecyclePayload(payload DataLifecycleSettingsPayload) (string, error) {
	if payload.RawReportHotDays < 1 {
		return "", errors.New("raw_report_hot_days must be >= 1")
	}
	if payload.RawReportHotDays > maxDataLifecycleRetentionDays {
		return "", errors.New("raw_report_hot_days must be <= 3650")
	}

	archiveDir := strings.TrimSpace(payload.ArchiveDir)
	if archiveDir == "" {
		if payload.ArchiveRawReports {
			return "", errArchiveDirRequired
		}
	} else {
		normalizedArchiveDir, err := normalizeArchiveDir(s.dataDir, archiveDir)
		if err != nil {
			return "", err
		}
		archiveDir = normalizedArchiveDir
	}
	if payload.ArchiveRawReports && !payload.RollupsEnabled {
		return "", errors.New("rollups_enabled is required when archive_raw_reports is enabled")
	}
	if payload.RollupRetentionDays != nil && *payload.RollupRetentionDays < 1 {
		return "", errors.New("rollup_retention_days must be >= 1 or null")
	}
	if payload.RollupRetentionDays != nil && *payload.RollupRetentionDays > maxDataLifecycleRetentionDays {
		return "", errors.New("rollup_retention_days must be <= 3650 or null")
	}
	switch payload.ArchiveSchedule {
	case "daily", "manual":
		return archiveDir, nil
	default:
		return "", errors.New("archive_schedule must be daily or manual")
	}
}

func defaultArchiveDir(dataDir string) string {
	dataRoot, err := cleanAbsPath(dataDir)
	if err != nil {
		return filepath.Join(dataDir, "archive")
	}
	return filepath.Join(dataRoot, "archive")
}

func normalizeArchiveDir(dataDir string, archiveDir string) (string, error) {
	if strings.ContainsRune(archiveDir, 0) {
		return "", errArchiveDirInvalid
	}

	dataRoot, err := cleanAbsPath(dataDir)
	if err != nil {
		return "", errCoreDataDirInvalid
	}

	cleanArchiveDir := filepath.Clean(archiveDir)
	candidate := cleanArchiveDir
	if !filepath.IsAbs(candidate) {
		if cleanArchiveDir == ".." || strings.HasPrefix(cleanArchiveDir, ".."+string(os.PathSeparator)) {
			return "", errArchiveDirOutsideDataDir
		}
		absFromWorkingDir, err := cleanAbsPath(cleanArchiveDir)
		if err != nil {
			return "", errArchiveDirInvalid
		}
		if pathStrictlyInside(dataRoot, absFromWorkingDir) {
			candidate = absFromWorkingDir
		} else {
			candidate = filepath.Join(dataRoot, cleanArchiveDir)
		}
	}
	candidate, err = cleanAbsPath(candidate)
	if err != nil {
		return "", errArchiveDirInvalid
	}
	if !pathStrictlyInside(dataRoot, candidate) {
		return "", errArchiveDirOutsideDataDir
	}
	if archiveDirEscapesThroughExistingSymlink(dataRoot, candidate) {
		return "", errArchiveDirOutsideDataDir
	}
	return candidate, nil
}

func cleanAbsPath(path string) (string, error) {
	if strings.ContainsRune(path, 0) {
		return "", errArchiveDirInvalid
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absPath = filepath.Clean(absPath)
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return filepath.Clean(resolvedPath), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	existingPath := absPath
	missingParts := []string{}
	for {
		parent := filepath.Dir(existingPath)
		if parent == existingPath {
			return absPath, nil
		}
		if _, statErr := os.Lstat(existingPath); statErr == nil {
			resolvedExisting, evalErr := filepath.EvalSymlinks(existingPath)
			if evalErr != nil {
				return "", evalErr
			}
			for i := len(missingParts) - 1; i >= 0; i-- {
				resolvedExisting = filepath.Join(resolvedExisting, missingParts[i])
			}
			return filepath.Clean(resolvedExisting), nil
		} else if os.IsNotExist(statErr) {
			missingParts = append(missingParts, filepath.Base(existingPath))
			existingPath = parent
		} else {
			return "", statErr
		}
	}
}

func pathStrictlyInside(root string, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func archiveDirEscapesThroughExistingSymlink(dataRoot string, candidate string) bool {
	resolvedDataRoot, err := filepath.EvalSymlinks(dataRoot)
	if err != nil {
		return false
	}
	existingAncestor, err := nearestExistingPath(candidate)
	if err != nil {
		return false
	}
	resolvedExistingAncestor, err := filepath.EvalSymlinks(existingAncestor)
	if err != nil {
		return false
	}

	resolvedDataRoot = filepath.Clean(resolvedDataRoot)
	resolvedExistingAncestor = filepath.Clean(resolvedExistingAncestor)
	if resolvedExistingAncestor == resolvedDataRoot {
		return false
	}
	return !pathStrictlyInside(resolvedDataRoot, resolvedExistingAncestor)
}
