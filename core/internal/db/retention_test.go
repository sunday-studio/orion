package db

import (
	"fmt"
	"orion/core/internal/logging"
	"testing"
	"time"
)

func TestPruneMonitorReportsArchivesAndRollsUp(t *testing.T) {
	dir := t.TempDir()
	main, err := Initialize(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(main); err != nil {
		t.Fatal(err)
	}
	archive, err := InitializeArchive(dir)
	if err != nil {
		t.Fatal(err)
	}

	old := time.Now().AddDate(0, 0, -40)
	recent := time.Now().AddDate(0, 0, -1)
	for i := 0; i < 5; i++ {
		health := "up"
		if i == 0 {
			health = "down"
		}
		main.Create(&MonitorReport{ID: fmt.Sprintf("old%d", i), MonitorID: "m1", Payload: "{}", CollectedAt: "x", Health: health, CreatedAt: old})
	}
	main.Create(&MonitorReport{ID: "new", MonitorID: "m1", Payload: "{}", CollectedAt: "x", Health: "up", CreatedAt: recent})

	cutoff := time.Now().AddDate(0, 0, -30)
	log := logging.NewLogger()
	// Run twice: the second pass must not double-count the rollup.
	for i := 0; i < 2; i++ {
		if err := pruneMonitorReports(main, archive, cutoff, log); err != nil {
			t.Fatal(err)
		}
	}

	var left, archived int64
	main.Model(&MonitorReport{}).Count(&left)
	archive.Model(&MonitorReport{}).Count(&archived)
	if left != 1 || archived != 5 {
		t.Fatalf("left=%d archived=%d, want 1 and 5", left, archived)
	}

	var roll MonitorDailyRollup
	if err := main.First(&roll, "monitor_id = ?", "m1").Error; err != nil {
		t.Fatal(err)
	}
	if roll.Total != 5 || roll.Up != 4 {
		t.Fatalf("rollup total=%d up=%d, want 5 and 4", roll.Total, roll.Up)
	}
}

func TestMigrateUsesCompositeIndexAndWAL(t *testing.T) {
	main, err := Initialize(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(main); err != nil {
		t.Fatal(err)
	}

	var mode string
	main.Raw("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" {
		t.Fatalf("journal_mode=%s, want wal", mode)
	}

	var plan []struct {
		Detail string
	}
	main.Raw("EXPLAIN QUERY PLAN SELECT * FROM monitor_reports WHERE monitor_id = 'x' ORDER BY created_at DESC LIMIT 20").Scan(&plan)
	for _, p := range plan {
		if p.Detail == "USE TEMP B-TREE FOR ORDER BY" {
			t.Fatalf("query still sorts via temp b-tree: %v", plan)
		}
	}
}
