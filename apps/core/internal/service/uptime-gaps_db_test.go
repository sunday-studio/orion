package service

import (
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"testing"
	"time"

	"gorm.io/gorm"
)

var gapDay = time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)

// gapTestDatabase has gap accounting active for May 2026 and a monitor that existed all month.
func gapTestDatabase(t *testing.T, monitorID string, intervalSeconds int) *gorm.DB {
	t.Helper()
	database := openReportTestDatabase(t)
	if err := database.Exec(`UPDATE schema_migrations SET applied_at = ? WHERE version = ?`, "2026-01-01 00:00:00+00:00", gapAccountingMigration).Error; err != nil {
		t.Fatalf("backdate migration: %v", err)
	}
	insertReportServiceNode(t, database, "node_gap")
	addGapMonitor(t, database, monitorID, "node_gap", intervalSeconds)
	return database
}

func addGapMonitor(t *testing.T, database *gorm.DB, monitorID, nodeID string, intervalSeconds int) {
	t.Helper()
	insertReportServiceMonitor(t, database, monitorID, nodeID, "active")
	if err := database.Model(&db.Monitor{}).Where("id = ?", monitorID).
		Updates(map[string]any{"created_at": time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "reporting_interval_seconds": intervalSeconds}).Error; err != nil {
		t.Fatalf("configure monitor: %v", err)
	}
}

func addReports(t *testing.T, database *gorm.DB, monitorID, health string, from time.Time, n int, step time.Duration) {
	t.Helper()
	for i := 0; i < n; i++ {
		insertReportServiceMonitorReport(t, database, monitorID, health, from.Add(time.Duration(i)*step))
	}
}

func rollupFor(t *testing.T, database *gorm.DB, monitorID string) db.MonitorUptimeRollup {
	t.Helper()
	var r db.MonitorUptimeRollup
	if err := database.Where("monitor_id = ? AND date = ?", monitorID, gapDay.Format("2006-01-02")).First(&r).Error; err != nil {
		t.Fatalf("rollup for %s: %v", monitorID, err)
	}
	return r
}

func TestRollupCountsSilenceAsDowntime(t *testing.T) {
	database := gapTestDatabase(t, "m1", 60)
	addReports(t, database, "m1", "up", gapDay, 600, time.Minute)                   // 00:00-09:59
	addReports(t, database, "m1", "up", gapDay.Add(12*time.Hour), 720, time.Minute) // 12:00-23:59

	if _, err := NewRollupService(database, utils.NewLogger()).RollupMonitorUptimeDay(gapDay); err != nil {
		t.Fatal(err)
	}
	r := rollupFor(t, database, "m1")
	if r.TotalCount != 1320 || r.MissedCount != 120 || r.IntervalSeconds != 60 {
		t.Fatalf("rollup = %+v, want total 1320, missed 120, interval 60", r)
	}
	if want := 100 * 1320.0 / 1440.0; r.UptimePercent < want-0.01 || r.UptimePercent > want+0.01 {
		t.Fatalf("uptime = %v, want about %v", r.UptimePercent, want)
	}
}

func TestRollupDoesNotChargePausedTime(t *testing.T) {
	database := gapTestDatabase(t, "m1", 60)
	addReports(t, database, "m1", "up", gapDay, 600, time.Minute)
	addReports(t, database, "m1", "up", gapDay.Add(12*time.Hour), 720, time.Minute)
	if err := database.Exec(`INSERT INTO uptime_exclusions (subject_type, subject_id, starts_at, ends_at) VALUES ('monitor_pause', 'm1', ?, ?)`,
		"2026-05-12 09:30:00+00:00", "2026-05-12 12:30:00+00:00").Error; err != nil {
		t.Fatal(err)
	}

	if _, err := NewRollupService(database, utils.NewLogger()).RollupMonitorUptimeDay(gapDay); err != nil {
		t.Fatal(err)
	}
	if r := rollupFor(t, database, "m1"); r.MissedCount != 0 || r.UptimePercent != 100 {
		t.Fatalf("rollup = %+v, want no missed checks while paused", r)
	}
}

func TestRollupCountsNodeMaintenanceAsExcluded(t *testing.T) {
	database := gapTestDatabase(t, "m1", 60)
	if err := database.Exec(`INSERT INTO uptime_exclusions (subject_type, subject_id, starts_at, ends_at) VALUES ('node_maintenance', 'node_gap', ?, ?)`,
		"2026-05-12 00:00:00+00:00", "2026-05-12 23:00:00+00:00").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewRollupService(database, utils.NewLogger()).RollupMonitorUptimeDay(gapDay); err != nil {
		t.Fatal(err)
	}
	// Silent all day but only the final hour is outside maintenance: 60 minutes less the first interval.
	if r := rollupFor(t, database, "m1"); r.TotalCount != 0 || r.MissedCount != 60 {
		t.Fatalf("rollup = %+v, want 0 reports and 60 missed", r)
	}
}

func TestRollupFullySilentDayIsDowntime(t *testing.T) {
	database := gapTestDatabase(t, "m1", 60)
	if _, err := NewRollupService(database, utils.NewLogger()).RollupMonitorUptimeDay(gapDay); err != nil {
		t.Fatal(err)
	}
	if r := rollupFor(t, database, "m1"); r.TotalCount != 0 || r.MissedCount != 1439 || r.UptimePercent != 0 {
		t.Fatalf("rollup = %+v, want a fully missed day", r)
	}
}

func TestRollupBeforeHistoryKeepsOldBehaviour(t *testing.T) {
	database := gapTestDatabase(t, "m1", 60)
	if err := database.Exec(`UPDATE schema_migrations SET applied_at = ? WHERE version = ?`, "2026-06-01 00:00:00+00:00", gapAccountingMigration).Error; err != nil {
		t.Fatal(err)
	}
	addReports(t, database, "m1", "up", gapDay, 10, time.Minute)
	if _, err := NewRollupService(database, utils.NewLogger()).RollupMonitorUptimeDay(gapDay); err != nil {
		t.Fatal(err)
	}
	if r := rollupFor(t, database, "m1"); r.MissedCount != 0 || r.UptimePercent != 100 {
		t.Fatalf("rollup = %+v, want silence ignored before pause history existed", r)
	}
}

func TestUptimeTodayCountsTrailingSilence(t *testing.T) {
	database := gapTestDatabase(t, "m1", 60)
	addReports(t, database, "m1", "up", gapDay, 480, time.Minute) // 00:00-07:59, then nothing
	now := gapDay.Add(12 * time.Hour)

	service := NewReportService(database, utils.NewLogger(), &config.Config{DataDir: t.TempDir()})
	result, err := service.getMonitorUptime("m1", "7d", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.UpCount != 480 || result.TotalCount != 720 {
		t.Fatalf("up/total = %d/%d, want 480/720 (240 missed checks since 08:00)", result.UpCount, result.TotalCount)
	}
}

func TestNodeUptimeWeightsMonitorsByTime(t *testing.T) {
	database := gapTestDatabase(t, "fast", 10)
	// Isolate weighting from silence: the test data covers only part of the day.
	if err := database.Exec(`UPDATE schema_migrations SET applied_at = ? WHERE version = ?`, "2026-06-01 00:00:00+00:00", gapAccountingMigration).Error; err != nil {
		t.Fatal(err)
	}
	addGapMonitor(t, database, "slow", "node_gap", 60)
	addReports(t, database, "fast", "up", gapDay, 600, 10*time.Second) // 100% up, 600 checks
	addReports(t, database, "slow", "up", gapDay, 50, time.Minute)
	addReports(t, database, "slow", "down", gapDay.Add(time.Hour), 50, time.Minute) // 50% up, 100 checks

	service := NewReportService(database, utils.NewLogger(), &config.Config{DataDir: t.TempDir()})
	result, err := service.getNodeUptime("node_gap", "7d", gapDay.Add(36*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// Equal time weight: (6000 + 3000) / (6000 + 6000) = 75%. Counting checks would give 92.9%.
	if result.UptimePercent < 74.99 || result.UptimePercent > 75.01 {
		t.Fatalf("node uptime = %v, want 75 (time weighted)", result.UptimePercent)
	}
}

func TestExclusionTriggersRecordPauseAndMaintenance(t *testing.T) {
	database := gapTestDatabase(t, "m1", 60)
	if err := database.Create(&db.CoreMonitorConfig{MonitorID: "m1", Kind: "http"}).Error; err != nil {
		t.Fatal(err)
	}
	count := func(subjectType, subjectID string, open bool) int64 {
		var n int64
		q := database.Model(&db.UptimeExclusion{}).Where("subject_type = ? AND subject_id = ?", subjectType, subjectID)
		if open {
			q = q.Where("ends_at IS NULL")
		}
		q.Count(&n)
		return n
	}

	database.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", "m1").Update("paused", true)
	if count("monitor_pause", "m1", true) != 1 {
		t.Fatal("pausing should open one exclusion")
	}
	database.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", "m1").Update("paused", false)
	if count("monitor_pause", "m1", true) != 0 || count("monitor_pause", "m1", false) != 1 {
		t.Fatal("resuming should close the exclusion and keep it as history")
	}

	start, end := time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(time.Hour)
	database.Model(&db.Monitor{}).Where("id = ?", "m1").Updates(map[string]any{"maintenance_starts_at": start, "maintenance_ends_at": end})
	if count("monitor_maintenance", "m1", false) != 1 {
		t.Fatal("scheduling maintenance should record a window")
	}
	database.Model(&db.Monitor{}).Where("id = ?", "m1").Updates(map[string]any{"maintenance_starts_at": nil, "maintenance_ends_at": nil})
	var window db.UptimeExclusion
	database.Where("subject_type = ? AND subject_id = ?", "monitor_maintenance", "m1").First(&window)
	if window.EndsAt == nil || window.EndsAt.After(time.Now().UTC().Add(time.Minute)) {
		t.Fatalf("ending maintenance should cut the window to now, got %+v", window)
	}

	until := time.Now().UTC().Add(2 * time.Hour)
	database.Model(&db.Node{}).Where("id = ?", "node_gap").Updates(map[string]any{"maintenance_mode": true, "maintenance_until": until})
	if count("node_maintenance", "node_gap", false) != 1 {
		t.Fatal("node maintenance should record a window")
	}
}
