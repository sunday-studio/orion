package metricstore

import (
	"database/sql"
	"time"

	"gorm.io/gorm"
)

// cumState is what the writer remembers about one sum, histogram or summary series: the newest reading
// it accepted and, for cumulative series, that reading's value, so the next one can be turned into a
// change. It lives in metrics.db (series_state, one row per series, removed with the series), so a restart
// of Core neither loses nor repeats a reading. Because it is one row per series and a series counts
// against the app's series cap, it is bounded by the cap; nothing is kept in memory.
type cumState struct {
	LastAt  int64 // unix nanoseconds of the newest reading accepted
	StartAt int64 // unix nanoseconds of its start time, 0 when the exporter did not say
	Value   float64
	Count   int64
	Sum     *float64
	Bounds  []float64
	Counts  []int64
}

func nanos(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

// deltaOf turns a cumulative reading into the change since prev, and says whether the series was reset
// (the exporter restarted: its start time changed or a count went down), in which case the new reading
// is the change, as it counted from zero again.
func deltaOf(prev cumState, t Temporality, cur Values) (out Values, reset bool) {
	reset = nanos(t.Start) != prev.StartAt
	if cur.Histogram == nil {
		v := *cur.Value
		if v < prev.Value {
			reset = true
		}
		if !reset {
			v -= prev.Value
		}
		return Values{Value: &v}, reset
	}
	h := *cur.Histogram
	if h.Count < prev.Count {
		reset = true
	}
	if reset {
		return Values{Histogram: &h}, true
	}
	d := h.clone()
	d.Count = h.Count - prev.Count
	if h.Sum != nil && prev.Sum != nil {
		s := *h.Sum - *prev.Sum
		d.Sum = &s
	} else {
		d.Sum = nil
	}
	if len(h.Counts) > 0 {
		before := prev.Counts
		if !equalFloats(prev.Bounds, h.Bounds) || len(before) != len(h.Counts) {
			// The bounds moved (an exponential histogram changed its scale): put the old counts on the new
			// buckets. A scale change only merges buckets, so this is exact.
			before = rebucket(Histogram{Count: prev.Count, Sum: prev.Sum, Bounds: prev.Bounds, Counts: prev.Counts}, h.Bounds)
		}
		for i := range d.Counts {
			d.Counts[i] = h.Counts[i] - before[i]
			if d.Counts[i] < 0 {
				return Values{Histogram: &h}, true
			}
		}
	}
	return Values{Histogram: &d}, false
}

// applyTemporality decides what a sum, histogram or summary reading stores. A reading that is not newer
// than the last one accepted for its series (a retry of a request that was written, or one that arrived
// out of order) is dropped and counted: storing it would count it twice. A cumulative reading becomes the
// change since the previous one; the first reading of a series, and the first after a gap longer than
// ActiveSeriesWindow, only set the baseline, since what it counted before Orion saw it is not known.
// The state moves in the batch's transaction, so a failed write moves nothing.
func (s *Store) applyTemporality(tx *gorm.DB, id int64, smp Sample) (Values, bool, error) {
	t := *smp.Temporality
	at := nanos(t.At)
	prev, err := loadState(tx, id)
	if err != nil {
		return Values{}, false, err
	}
	if prev != nil && at <= prev.LastAt {
		s.droppedStale.Add(1)
		return Values{}, false, nil
	}
	if prev != nil && at-prev.LastAt > int64(ActiveSeriesWindow) {
		prev = nil
	}
	next := cumState{LastAt: at, StartAt: nanos(t.Start)}
	if smp.Values.Value != nil {
		next.Value = *smp.Values.Value
	}
	if h := smp.Values.Histogram; h != nil {
		next.Count, next.Sum, next.Bounds, next.Counts = h.Count, h.Sum, h.Bounds, h.Counts
	}
	out := smp.Values
	emit := true
	if t.Cumulative {
		if prev == nil {
			emit = false
		} else {
			out, _ = deltaOf(*prev, t, smp.Values)
		}
	}
	if err := saveState(tx, id, next); err != nil {
		return Values{}, false, err
	}
	return out, emit, nil
}

type stateRow struct {
	LastAt     int64
	StartAt    int64
	LastValue  sql.NullFloat64
	LastCount  sql.NullInt64
	LastSum    sql.NullFloat64
	LastBounds string
	LastCounts string
}

func loadState(tx *gorm.DB, id int64) (*cumState, error) {
	rows, err := tx.Raw("SELECT last_at, start_at, last_value, last_count, last_sum, last_bounds, last_counts FROM series_state WHERE series_id = ?", id).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var r stateRow
	if err := rows.Scan(&r.LastAt, &r.StartAt, &r.LastValue, &r.LastCount, &r.LastSum, &r.LastBounds, &r.LastCounts); err != nil {
		return nil, err
	}
	st := &cumState{LastAt: r.LastAt, StartAt: r.StartAt, Value: r.LastValue.Float64, Count: r.LastCount.Int64,
		Bounds: decodeFloats(r.LastBounds), Counts: decodeInts(r.LastCounts)}
	if r.LastSum.Valid {
		st.Sum = &r.LastSum.Float64
	}
	return st, nil
}

func saveState(tx *gorm.DB, id int64, st cumState) error {
	return tx.Exec(`INSERT INTO series_state (series_id, last_at, start_at, last_value, last_count, last_sum, last_bounds, last_counts)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(series_id) DO UPDATE SET last_at = excluded.last_at, start_at = excluded.start_at, last_value = excluded.last_value,
 last_count = excluded.last_count, last_sum = excluded.last_sum, last_bounds = excluded.last_bounds, last_counts = excluded.last_counts`,
		id, st.LastAt, st.StartAt, st.Value, st.Count, st.Sum, encodeFloats(st.Bounds), encodeInts(st.Counts)).Error
}
