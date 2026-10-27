package service

import (
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/utils"
)

func TestBackfillRollsUpOnlyMissingDaysWithReports(t *testing.T) {
	database := openRollupTestDatabase(t)
	svc := NewRollupService(database, utils.NewLogger())
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	// Reports on days -5, -4, -2; -4 already has a rollup, -3 has no reports at all.
	for _, back := range []int{5, 4, 2} {
		insertRollupReport(t, database, "monitor_a", "up", now.AddDate(0, 0, -back))
	}
	if _, err := svc.RollupMonitorUptimeDay(now.AddDate(0, 0, -4)); err != nil {
		t.Fatal(err)
	}
	// A report today must not be rolled up: it is not a completed day.
	insertRollupReport(t, database, "monitor_a", "up", now)

	done, err := svc.BackfillMissingMonitorUptimeDays(now)
	if err != nil {
		t.Fatal(err)
	}
	if done != 2 {
		t.Fatalf("backfilled %d days, want 2 (days -5 and -2)", done)
	}
	var dates []string
	database.Model(&db.MonitorUptimeRollup{}).Order("date").Pluck("date", &dates)
	want := []string{"2026-06-05", "2026-06-06", "2026-06-08"}
	if len(dates) != len(want) {
		t.Fatalf("rollup dates = %v, want %v", dates, want)
	}
	for i := range want {
		if dates[i] != want[i] {
			t.Fatalf("rollup dates = %v, want %v", dates, want)
		}
	}

	again, err := svc.BackfillMissingMonitorUptimeDays(now)
	if err != nil || again != 0 {
		t.Fatalf("second run = %d, %v; want a no-op", again, err)
	}
}

func TestBackfillIsBoundedPerRunAndNewestFirst(t *testing.T) {
	database := openRollupTestDatabase(t)
	svc := NewRollupService(database, utils.NewLogger())
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	total := rollupBackfillMaxDaysPerRun + 5
	for back := 1; back <= total; back++ {
		insertRollupReport(t, database, "monitor_a", "up", now.AddDate(0, 0, -back))
	}

	done, err := svc.BackfillMissingMonitorUptimeDays(now)
	if err != nil || done != rollupBackfillMaxDaysPerRun {
		t.Fatalf("first run = %d, %v; want %d", done, err, rollupBackfillMaxDaysPerRun)
	}
	var newest, oldest string
	database.Model(&db.MonitorUptimeRollup{}).Select("MAX(date)").Scan(&newest)
	database.Model(&db.MonitorUptimeRollup{}).Select("MIN(date)").Scan(&oldest)
	if newest != "2026-06-09" || oldest != now.AddDate(0, 0, -rollupBackfillMaxDaysPerRun).Format(uptimeDateLayout) {
		t.Fatalf("rolled %s..%s, want newest days first", oldest, newest)
	}

	done, err = svc.BackfillMissingMonitorUptimeDays(now)
	if err != nil || done != 5 {
		t.Fatalf("second run = %d, %v; want the remaining 5", done, err)
	}
}

func TestBackfillWithNoReportsIsANoOp(t *testing.T) {
	database := openRollupTestDatabase(t)
	done, err := NewRollupService(database, utils.NewLogger()).BackfillMissingMonitorUptimeDays(time.Now().UTC())
	if err != nil || done != 0 {
		t.Fatalf("got %d, %v", done, err)
	}
}
