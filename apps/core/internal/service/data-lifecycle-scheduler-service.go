package service

import (
	"context"
	"errors"
	"orion/core/internal/logging"
	"time"

	"gorm.io/gorm"
)

const defaultDataLifecycleSchedulerInterval = time.Hour

type DataLifecycleScheduleResult struct {
	RollupRan     bool `json:"rollup_ran"`
	ArchiveRan    bool `json:"archive_ran"`
	SkippedManual bool `json:"skipped_manual"`
}

type DataLifecycleSchedulerService struct {
	db       *gorm.DB
	logger   *logging.Logger
	dataDir  string
	interval time.Duration
}

func NewDataLifecycleSchedulerService(database *gorm.DB, logger *logging.Logger, dataDir string, interval time.Duration) *DataLifecycleSchedulerService {
	if interval <= 0 {
		interval = defaultDataLifecycleSchedulerInterval
	}
	return &DataLifecycleSchedulerService{
		db:       database,
		logger:   logger,
		dataDir:  dataDir,
		interval: interval,
	}
}

func (s *DataLifecycleSchedulerService) Run(ctx context.Context) error {
	s.runAndLog(time.Now().UTC())

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			s.runAndLog(now.UTC())
		}
	}
}

func (s *DataLifecycleSchedulerService) RunDue(now time.Time) (*DataLifecycleScheduleResult, error) {
	settings, err := NewSettingsService(s.db, s.logger, s.dataDir).GetDataLifecycleSettings()
	if err != nil {
		return nil, err
	}

	result := &DataLifecycleScheduleResult{}
	if settings.ArchiveSchedule == "manual" {
		result.SkippedManual = true
		return result, nil
	}

	var runErrors []error
	if settings.RollupsEnabled && shouldRunDailyJob(settings.LastRollupRunAt, now) {
		if _, err := NewRollupService(s.db, s.logger).RunDailyMonitorUptimeRollup(now); err != nil {
			runErrors = append(runErrors, err)
		} else {
			result.RollupRan = true
		}
	}

	if settings.ArchiveRawReports && shouldRunDailyJob(settings.LastArchiveRunAt, now) {
		if _, err := NewArchiveService(s.db, s.logger, s.dataDir).RunRawReportArchive(now); err != nil {
			runErrors = append(runErrors, err)
		} else {
			result.ArchiveRan = true
		}
	}

	return result, errors.Join(runErrors...)
}

func (s *DataLifecycleSchedulerService) runAndLog(now time.Time) {
	result, err := s.RunDue(now)
	if err != nil {
		s.logger.Error("Scheduled data lifecycle run failed", "error", err)
		return
	}
	if result.RollupRan || result.ArchiveRan {
		s.logger.Info("Scheduled data lifecycle run completed", "rollup_ran", result.RollupRan, "archive_ran", result.ArchiveRan)
	}
}

func shouldRunDailyJob(lastRunAt *time.Time, now time.Time) bool {
	if lastRunAt == nil {
		return true
	}
	last := lastRunAt.In(now.Location())
	return last.Year() != now.Year() || last.YearDay() != now.YearDay()
}
