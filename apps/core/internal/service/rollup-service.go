package service

import (
	"orion/core/internal/db"
	"orion/core/internal/utils"
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
	logger *utils.Logger
}

func NewRollupService(database *gorm.DB, logger *utils.Logger) *RollupService {
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
	window := timeSpan{start, end}

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

	if err := s.addMissedChecks(rollups, window); err != nil {
		s.logger.Error("Failed to account for missed checks", "date", dateKey, "error", err)
		return nil, err
	}

	for _, rollup := range rollups {
		if denominator := rollup.TotalCount + rollup.MissedCount; denominator > 0 {
			rollup.UptimePercent = 100 * float64(rollup.UpCount) / float64(denominator)
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

// addMissedChecks fills missed_count and interval_seconds on the day's rollups and adds a rollup for
// every active monitor that sent nothing at all (a fully silent day is the clearest downtime).
// Days before paused/maintenance history existed keep missed_count at 0; see gapAccountingMigration.
func (s *RollupService) addMissedChecks(rollups map[string]*db.MonitorUptimeRollup, window timeSpan) error {
	dateKey := window.start.Format("2006-01-02")
	monitors, err := loadUptimeMonitors(s.db, nil)
	if err != nil {
		return err
	}
	for id, rollup := range rollups {
		if m, ok := monitors[id]; ok {
			rollup.IntervalSeconds = int(m.interval() / time.Second)
		}
	}
	since := gapAccountingStart(s.db)
	if since.IsZero() || window.start.Before(since) {
		return nil
	}

	excluded, err := loadExcludedSpans(s.db, window, monitors)
	if err != nil {
		return err
	}

	// Stream report times grouped by monitor so memory stays at one monitor's day.
	reports, err := s.db.Model(&db.MonitorReport{}).Select("monitor_id", "created_at").
		Where("created_at >= ? AND created_at < ?", window.start, window.end).
		Order("monitor_id, created_at DESC").Rows()
	if err != nil {
		return err
	}
	defer reports.Close()

	var current string
	var times []time.Time
	seen := map[string]bool{}
	flush := func() {
		if current == "" {
			return
		}
		seen[current] = true
		m, ok := monitors[current]
		if !ok {
			return
		}
		for i, j := 0, len(times)-1; i < j; i, j = i+1, j-1 {
			times[i], times[j] = times[j], times[i]
		}
		missed := missedChecks(times, m.coverage(window), m.interval(), excluded[current])
		if rollup := rollups[current]; rollup != nil {
			rollup.MissedCount = missed
		}
	}
	for reports.Next() {
		var monitorID string
		var at time.Time
		if err := reports.Scan(&monitorID, &at); err != nil {
			return err
		}
		if monitorID != current {
			flush()
			current, times = monitorID, times[:0]
		}
		times = append(times, at.UTC())
	}
	if err := reports.Err(); err != nil {
		return err
	}
	flush()

	for id, m := range monitors {
		if seen[id] || !m.CreatedAt.Before(window.end) {
			continue
		}
		missed := missedChecks(nil, m.coverage(window), m.interval(), excluded[id])
		if missed == 0 {
			continue
		}
		rollups[id] = &db.MonitorUptimeRollup{
			MonitorID:       id,
			Date:            dateKey,
			MissedCount:     missed,
			IntervalSeconds: int(m.interval() / time.Second),
		}
	}
	return nil
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
			"missed_count",
			"interval_seconds",
			"uptime_percent",
			"updated_at",
		}),
	}).Create(rollup).Error
}

func dayStart(date time.Time) time.Time {
	year, month, day := date.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, date.Location())
}
