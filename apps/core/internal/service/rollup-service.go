package service

import (
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RollupRunResult struct {
	Date         string `json:"date"`
	MonitorDays  int    `json:"monitor_days"`
	ReportCount  int    `json:"report_count"`
	SkippedToday bool   `json:"skipped_today"`
}

type RollupService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewRollupService(database *gorm.DB, logger *logging.Logger) *RollupService {
	return &RollupService{
		db:     database,
		logger: logger,
	}
}

func (s *RollupService) RunDailyMonitorUptimeRollup(now time.Time) (*RollupRunResult, error) {
	date := now.AddDate(0, 0, -1)
	result, err := s.RollupMonitorUptimeDay(date)
	if err != nil {
		return nil, err
	}

	if err := s.db.Model(&db.DataLifecycleSettings{}).Where("id = ?", 1).Update("last_rollup_run_at", now).Error; err != nil {
		return nil, err
	}

	return result, nil
}

func (s *RollupService) RollupMonitorUptimeDay(date time.Time) (*RollupRunResult, error) {
	start := dayStart(date)
	end := start.AddDate(0, 0, 1)
	dateKey := start.Format("2006-01-02")

	var rows []struct {
		MonitorID string
		Health    string
		Count     int
	}
	if err := s.db.Model(&db.MonitorReport{}).
		Select("monitor_id, health, count(*) as count").
		Where("created_at >= ? AND created_at < ?", start, end).
		Group("monitor_id, health").
		Scan(&rows).Error; err != nil {
		s.logger.Error("Failed to query monitor uptime rollup rows", "date", dateKey, "error", err)
		return nil, err
	}

	rollups := map[string]*db.MonitorUptimeRollup{}
	reportCount := 0
	for _, row := range rows {
		rollup := rollups[row.MonitorID]
		if rollup == nil {
			rollup = &db.MonitorUptimeRollup{
				MonitorID: row.MonitorID,
				Date:      dateKey,
			}
			rollups[row.MonitorID] = rollup
		}

		rollup.TotalCount += row.Count
		reportCount += row.Count
		switch row.Health {
		case "up":
			rollup.UpCount += row.Count
		case "down":
			rollup.DownCount += row.Count
		case "degraded":
			rollup.DegradedCount += row.Count
		default:
			rollup.UnknownCount += row.Count
		}
	}

	for _, rollup := range rollups {
		if rollup.TotalCount > 0 {
			rollup.UptimePercent = 100 * float64(rollup.UpCount) / float64(rollup.TotalCount)
		}
		if err := s.upsertMonitorUptimeRollup(rollup); err != nil {
			s.logger.Error("Failed to upsert monitor uptime rollup", "monitor_id", rollup.MonitorID, "date", dateKey, "error", err)
			return nil, err
		}
	}

	return &RollupRunResult{
		Date:        dateKey,
		MonitorDays: len(rollups),
		ReportCount: reportCount,
	}, nil
}

func (s *RollupService) upsertMonitorUptimeRollup(rollup *db.MonitorUptimeRollup) error {
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "monitor_id"},
			{Name: "date"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"up_count",
			"down_count",
			"degraded_count",
			"unknown_count",
			"total_count",
			"uptime_percent",
			"updated_at",
		}),
	}).Create(rollup).Error
}

func dayStart(date time.Time) time.Time {
	year, month, day := date.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, date.Location())
}
