package service

import (
	"orion/core/internal/db"
	"time"
)

const (
	// rollupBackfillWindowDays is how far back missing days are looked for; it matches the longest
	// period the read path serves.
	rollupBackfillWindowDays = maxPeriodDays
	// rollupBackfillMaxDaysPerRun bounds the work of one scheduler tick. Each day is one indexed scan of
	// that day's reports, so a long history catches up over a few ticks instead of in one long job.
	rollupBackfillMaxDaysPerRun = 31
)

// BackfillMissingMonitorUptimeDays rolls up completed days that still have raw reports but no rollup.
// The daily job only covers yesterday, so without this an upgraded install, or one that was down when
// the job was due, serves those days by aggregating raw rows on every request. Newest days go first
// because the Console reads the most recent periods. Days with no raw reports are skipped: the read
// path already handles them without scanning anything. It returns how many days it rolled up.
func (s *RollupService) BackfillMissingMonitorUptimeDays(now time.Time) (int, error) {
	today := dayStart(now.UTC())
	var firstAt *string
	if err := s.db.Model(&db.MonitorReport{}).Select("MIN(created_at)").Scan(&firstAt).Error; err != nil {
		return 0, err
	}
	if firstAt == nil || len(*firstAt) < 10 {
		return 0, nil
	}
	oldest, err := time.Parse(uptimeDateLayout, (*firstAt)[:10])
	if err != nil {
		return 0, nil
	}
	if floor := today.AddDate(0, 0, -rollupBackfillWindowDays); oldest.Before(floor) {
		oldest = floor
	}

	var have []string
	if err := s.db.Model(&db.MonitorUptimeRollup{}).Where("date >= ? AND date < ?", oldest.Format(uptimeDateLayout), today.Format(uptimeDateLayout)).
		Distinct().Pluck("date", &have).Error; err != nil {
		return 0, err
	}
	rolled := make(map[string]bool, len(have))
	for _, d := range have {
		rolled[d] = true
	}

	done := 0
	for day := today.AddDate(0, 0, -1); !day.Before(oldest) && done < rollupBackfillMaxDaysPerRun; day = day.AddDate(0, 0, -1) {
		if rolled[day.Format(uptimeDateLayout)] {
			continue
		}
		var probe []string
		if err := s.db.Model(&db.MonitorReport{}).Where("created_at >= ? AND created_at < ?", day, day.AddDate(0, 0, 1)).Limit(1).Pluck("id", &probe).Error; err != nil {
			return done, err
		}
		if len(probe) == 0 {
			continue
		}
		if _, err := s.RollupMonitorUptimeDay(day); err != nil {
			return done, err
		}
		done++
	}
	if done > 0 {
		s.logger.Info("Backfilled missing monitor uptime rollups", "days", done)
	}
	return done, nil
}
