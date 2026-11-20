package metricstore

import (
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"testing"
	"time"
)

func TestDeltaSumsFromTwoExportersSharingASeriesAreBothKept(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	key := "jobs.done|{}"
	at := func(sec int) time.Time { return base.Add(time.Duration(sec) * time.Second) }
	delta := func(v float64, sec int) Sample { return sumReading(key, v, false, at(sec-10), at(sec)) }

	writeAt(s, at(10), delta(5, 10)) // exporter A
	writeAt(s, at(7), delta(3, 7))   // exporter B: older, a different reading
	writeAt(s, at(10), delta(4, 10)) // B again at the same instant as A, different content
	if got := valuesOf(queryAll(t, s, key, base.Add(-time.Minute)).Points); !sameFloats(got, []float64{12}) {
		t.Fatalf("minute = %v, want [12]: the older and the same-time readings are real points of another process", got)
	}
	if st := s.Stats(); st.DroppedStale != 0 || st.Written != 3 {
		t.Fatalf("stats = %+v, want 3 written and none dropped", st)
	}

	// Only an exact repeat of the last accepted reading is a retry.
	writeAt(s, at(10), delta(4, 10))
	if got := valuesOf(queryAll(t, s, key, base.Add(-time.Minute)).Points); !sameFloats(got, []float64{12}) {
		t.Fatalf("minute = %v after a retry, want it unchanged", got)
	}
	if st := s.Stats(); st.DroppedStale != 1 || st.Written != 3 {
		t.Fatalf("stats = %+v, want the retry dropped and counted once", st)
	}
	// The same value at another time is a new reading, not a duplicate.
	writeAt(s, at(20), delta(4, 20))
	if got := valuesOf(queryAll(t, s, key, base.Add(-time.Minute)).Points); !sameFloats(got, []float64{16}) {
		t.Fatalf("minute = %v, want [16]", got)
	}
}

func TestDeltaHistogramsDedupeOnContentToo(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	key := "histogram:lat|{}"
	at := func(sec int) time.Time { return base.Add(time.Duration(sec) * time.Second) }
	h := func(a, b int64) Histogram {
		return Histogram{Count: a + b, Sum: fp(float64(a + 2*b)), Bounds: []float64{1}, Counts: []int64{a, b}}
	}
	writeAt(s, at(30), histReading(key, h(1, 1), false, base, at(30)))
	writeAt(s, at(20), histReading(key, h(2, 0), false, base, at(20))) // older and different: kept
	writeAt(s, at(20), histReading(key, h(2, 0), false, base, at(20))) // exact repeat of the last one: dropped
	writeAt(s, at(20), histReading(key, h(1, 1), false, base, at(20))) // same time, other content: kept
	pts := queryAll(t, s, key, base.Add(-time.Minute)).Points
	if len(pts) != 1 || *pts[0].Count != 6 {
		t.Fatalf("points = %+v, want one minute holding 2+2+2 observations", pts)
	}
	if st := s.Stats(); st.DroppedStale != 1 {
		t.Fatalf("dropped stale = %d, want 1", st.DroppedStale)
	}
}

func TestCumulativeSeriesKeepTheNotNewerRule(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	key := "requests|{}"
	at := func(m int) time.Time { return base.Add(time.Duration(m)*time.Minute + time.Second) }
	writeAt(s, at(0), sumReading(key, 10, true, base, at(0)))
	writeAt(s, at(2), sumReading(key, 30, true, base, at(2)))
	writeAt(s, at(1), sumReading(key, 20, true, base, at(1))) // older and different: still dropped
	writeAt(s, at(2), sumReading(key, 31, true, base, at(2))) // same time, other value: still dropped
	if got := valuesOf(queryAll(t, s, key, base.Add(-time.Minute)).Points); !sameFloats(got, []float64{20}) {
		t.Fatalf("increases = %v, want [20]", got)
	}
	if st := s.Stats(); st.DroppedStale != 2 {
		t.Fatalf("dropped stale = %d, want 2", st.DroppedStale)
	}
}

func TestAMetricsFileFromBeforeTheHashColumnIsUpgraded(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, utils.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	base := baseMinute()
	key := "jobs|{}"
	writeAt(s, base.Add(5*time.Second), sumReading(key, 5, false, base, base.Add(5*time.Second)))
	s.Close()

	// Turn the file back into one an older Core wrote: no last_hash column.
	raw, err := db.OpenFile(dir, metricsDatabaseFileName, "warn")
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Exec("ALTER TABLE series_state DROP COLUMN last_hash").Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := raw.DB(); err == nil {
		sqlDB.Close()
	}

	s, err = Open(dir, utils.NewLogger())
	if err != nil {
		t.Fatalf("reopening an old file: %v", err)
	}
	defer s.Close()
	var n int
	s.db.Raw("SELECT COUNT(*) FROM pragma_table_info('series_state') WHERE name = 'last_hash'").Scan(&n)
	if n != 1 {
		t.Fatalf("last_hash columns = %d, want it added", n)
	}
	writeAt(s, base.Add(15*time.Second), sumReading(key, 6, false, base, base.Add(15*time.Second)))
	writeAt(s, base.Add(15*time.Second), sumReading(key, 6, false, base, base.Add(15*time.Second))) // retry now caught
	if got := valuesOf(queryAll(t, s, key, base.Add(-time.Minute)).Points); !sameFloats(got, []float64{11}) {
		t.Fatalf("minute = %v, want [11]", got)
	}
}

func TestARolledBackBatchDoesNotCountStaleReadings(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	at := base.Add(5 * time.Second)
	writeAt(s, at, sumReading("a|{}", 1, false, base, at))
	if err := s.db.Exec(`CREATE TRIGGER fail_on_marker BEFORE INSERT ON samples WHEN NEW.value = 999 BEGIN SELECT RAISE(ABORT, 'boom'); END`).Error; err != nil {
		t.Fatal(err)
	}
	// One duplicate (dropped as stale) and then a write that fails: the batch rolls back as a whole.
	s.Enqueue(otlpBatch(base, sumReading("a|{}", 1, false, base, at), sumReading("b|{}", 999, false, base, at)))
	s.Flush()
	if st := s.Stats(); st.BatchesFailed != 1 || st.DroppedStale != 0 {
		t.Fatalf("stats = %+v, want the failed batch counted and its stale reading not", st)
	}
	// The next batch that does commit counts its stale readings.
	s.db.Exec("DROP TRIGGER fail_on_marker")
	writeAt(s, at, sumReading("a|{}", 1, false, base, at))
	if st := s.Stats(); st.DroppedStale != 1 {
		t.Fatalf("dropped stale = %d, want 1", st.DroppedStale)
	}
}

func TestPruneRemovesSeriesStateOlderThanTheActiveWindow(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	old := base.Add(-30 * time.Hour)
	writeAt(s, old, sumReading("old|{}", 1, true, old, old))
	writeAt(s, base, sumReading("fresh|{}", 1, true, base, base))
	count := func() (n int) {
		s.db.Raw("SELECT COUNT(*) FROM series_state").Scan(&n)
		return n
	}
	if count() != 2 {
		t.Fatalf("state rows = %d, want 2", count())
	}
	// Retention keeps the series themselves; only the unusable state goes.
	if _, err := s.Prune(Retention{Raw: 90 * 24 * time.Hour, Rollup: 90 * 24 * time.Hour}, time.Now()); err != nil {
		t.Fatal(err)
	}
	var left []string
	s.db.Raw("SELECT s.key FROM series_state st JOIN series s ON s.id = st.series_id").Scan(&left)
	if len(left) != 1 || left[0] != "fresh|{}" {
		t.Fatalf("state left for %v, want only the series seen within %v", left, ActiveSeriesWindow)
	}
	var series int
	s.db.Raw("SELECT COUNT(*) FROM series").Scan(&series)
	if series != 2 {
		t.Fatalf("series = %d: pruning the state must not remove the series", series)
	}
	// The next reading of the old series is a new baseline, as a gap this long always was.
	writeAt(s, base.Add(time.Minute), sumReading("old|{}", 500, true, old, base.Add(time.Minute)))
	if got := valuesOf(queryAll(t, s, "old|{}", base.Add(-time.Minute)).Points); len(got) != 0 {
		t.Fatalf("increases = %v, want none: the first reading after the state is gone only sets a baseline", got)
	}
}
