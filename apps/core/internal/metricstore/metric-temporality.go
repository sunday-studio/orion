package metricstore

import (
	"database/sql"
	"encoding/binary"
	"hash/fnv"
	"math"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// cumState is what the writer remembers about one sum, histogram or summary series: the newest reading
// it accepted and, for cumulative series, that reading's value, so the next one can be turned into a
// change. It lives in metrics.db (series_state, one row per series), so a restart of Core neither loses
// nor repeats a reading. A row is removed with its series, and Prune also removes a row whose last reading
// is older than ActiveSeriesWindow, as it could never be used again (a gap that long starts a new
// baseline). Rows are therefore bounded by the series seen in the last 24 hours; nothing is kept in memory.
type cumState struct {
	LastAt int64 // unix nanoseconds of the last reading accepted
	// LastHash fingerprints that reading (see readingHash). Delta series use it to spot an exact retry.
	LastHash string
	StartAt  int64 // unix nanoseconds of its start time, 0 when the exporter did not say
	Value    float64
	Count    int64
	Sum      *float64
	Bounds   []float64
	Counts   []int64
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

// readingHash fingerprints one reading: its time, its start time and every number it carries. Two
// readings with the same time and hash are the same reading sent twice.
func readingHash(t Temporality, v Values) string {
	h := fnv.New64a()
	put := func(u uint64) {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], u)
		h.Write(b[:])
	}
	putF := func(f *float64) {
		if f == nil {
			put(1)
			return
		}
		put(0)
		put(math.Float64bits(*f))
	}
	put(uint64(nanos(t.At)))
	put(uint64(nanos(t.Start)))
	putF(v.Value)
	if hist := v.Histogram; hist != nil {
		put(uint64(hist.Count))
		putF(hist.Sum)
		putF(hist.Min)
		putF(hist.Max)
		put(uint64(len(hist.Bounds)))
		for _, b := range hist.Bounds {
			put(math.Float64bits(b))
		}
		put(uint64(len(hist.Counts)))
		for _, c := range hist.Counts {
			put(uint64(c))
		}
		put(uint64(len(hist.Quantiles)))
		for _, q := range hist.Quantiles {
			put(math.Float64bits(q.Q))
			put(math.Float64bits(q.V))
		}
	}
	return strconv.FormatUint(h.Sum64(), 16)
}

// applyTemporality decides what a sum, histogram or summary reading stores.
//
// Cumulative: a reading that is not newer than the last one accepted for its series (a retry of a request
// that was written, or one that arrived out of order) is dropped and counted, since storing it would
// count it twice. Otherwise it becomes the change since the previous one; the first reading of a series,
// and the first after a gap longer than ActiveSeriesWindow, only set the baseline, since what it counted
// before Orion saw it is not known.
//
// Delta: every reading is a change in its own right, and two exporter processes may share one series
// identity and report at the same moments, so "not newer" would lose real points. Only a reading that is
// an exact duplicate of the last one accepted (same time, same content) is dropped; an older or
// different one is added. Two processes that send identical readings at an identical time still look
// like one retry: give each process its own service.instance.id.
//
// stale counts the dropped readings; the caller adds it to the store's counter once the batch has
// committed. The state moves in the batch's transaction, so a failed write moves nothing.
func (s *Store) applyTemporality(tx *gorm.DB, id int64, smp Sample, stale *int64) (Values, bool, error) {
	t := *smp.Temporality
	at := nanos(t.At)
	prev, err := loadState(tx, id)
	if err != nil {
		return Values{}, false, err
	}
	hash := readingHash(t, smp.Values)
	if prev != nil {
		duplicate := at == prev.LastAt && hash == prev.LastHash
		if (t.Cumulative && at <= prev.LastAt) || (!t.Cumulative && duplicate) {
			*stale++
			return Values{}, false, nil
		}
	}
	if t.Cumulative && prev != nil && at-prev.LastAt > int64(ActiveSeriesWindow) {
		prev = nil
	}
	next := cumState{LastAt: at, LastHash: hash, StartAt: nanos(t.Start)}
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
	LastHash   string
}

func loadState(tx *gorm.DB, id int64) (*cumState, error) {
	rows, err := tx.Raw("SELECT last_at, start_at, last_value, last_count, last_sum, last_bounds, last_counts, last_hash FROM series_state WHERE series_id = ?", id).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var r stateRow
	if err := rows.Scan(&r.LastAt, &r.StartAt, &r.LastValue, &r.LastCount, &r.LastSum, &r.LastBounds, &r.LastCounts, &r.LastHash); err != nil {
		return nil, err
	}
	st := &cumState{LastAt: r.LastAt, StartAt: r.StartAt, Value: r.LastValue.Float64, Count: r.LastCount.Int64,
		Bounds: decodeFloats(r.LastBounds), Counts: decodeInts(r.LastCounts), LastHash: r.LastHash}
	if r.LastSum.Valid {
		st.Sum = &r.LastSum.Float64
	}
	return st, nil
}

func saveState(tx *gorm.DB, id int64, st cumState) error {
	return tx.Exec(`INSERT INTO series_state (series_id, last_at, start_at, last_value, last_count, last_sum, last_bounds, last_counts, last_hash)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(series_id) DO UPDATE SET last_at = excluded.last_at, start_at = excluded.start_at, last_value = excluded.last_value,
 last_count = excluded.last_count, last_sum = excluded.last_sum, last_bounds = excluded.last_bounds, last_counts = excluded.last_counts, last_hash = excluded.last_hash`,
		id, st.LastAt, st.StartAt, st.Value, st.Count, st.Sum, encodeFloats(st.Bounds), encodeInts(st.Counts), st.LastHash).Error
}
