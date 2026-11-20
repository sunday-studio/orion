package metricstore

import (
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"testing"
	"time"
)

// TestClearingTheSumDeltasFlagRunsTheSumCleanupAgain backs the instruction in docs/product-metrics.md for
// an operator who downgraded and upgraded again: with Core stopped, removing the otlp_sum_deltas row of
// the meta table makes the next start remove the history of the OTLP sums, and nothing else.
func TestClearingTheSumDeltasFlagRunsTheSumCleanupAgain(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, utils.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	base := baseMinute()
	at := base.Add(5 * time.Second)
	writeAt(s, at, sumReading("jobs|{}", 5, false, base, at))
	writeAt(s, at, Sample{Key: "mem|{}", Kind: KindGauge, Name: "mem", Metric: "mem", Values: Values{Value: fp(30)}})
	if v, ok, err := s.readMeta(sumDeltasKey); err != nil || !ok || v != 1 {
		t.Fatalf("flag after the first start = %d %v %v, want it set", v, ok, err)
	}
	// What an older Core wrote after a downgrade: a cumulative total stored in the sum series.
	var sumID int64
	s.db.Raw("SELECT id FROM series WHERE kind = 'sum'").Scan(&sumID)
	s.db.Exec("INSERT INTO samples (series_id, ts, value) VALUES (?, ?, ?)", sumID, base.Add(10*time.Minute).Unix(), 987654)
	s.Close()

	raw, err := db.OpenFile(dir, metricsDatabaseFileName, "warn")
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Exec("DELETE FROM meta WHERE k = 'otlp_sum_deltas'").Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := raw.DB(); err == nil {
		sqlDB.Close()
	}

	s, err = Open(dir, utils.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	count := func(query string) (n int) {
		s.db.Raw(query).Scan(&n)
		return n
	}
	if n := count("SELECT COUNT(*) FROM samples WHERE series_id = (SELECT id FROM series WHERE kind = 'sum')"); n != 0 {
		t.Fatalf("%d sum rows left: the cleanup should have removed the sum history", n)
	}
	if n := count("SELECT COUNT(*) FROM series WHERE kind = 'sum'"); n != 1 {
		t.Fatalf("sum series = %d: the series stays and fills again", n)
	}
	if n := count("SELECT COUNT(*) FROM samples WHERE series_id = (SELECT id FROM series WHERE kind = 'gauge')"); n != 1 {
		t.Fatalf("gauge rows = %d: only sums are cleared", n)
	}
	if v, ok, _ := s.readMeta(sumDeltasKey); !ok || v != 1 {
		t.Fatalf("flag after the repeat = %d %v, want it set again", v, ok)
	}
}
