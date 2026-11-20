package metricstore

import (
	"gorm.io/gorm"
)

const (
	upsertHistogramHourly = `INSERT OR REPLACE INTO histogram_hourly (series_id, hour, n, count, sum, min, max, bounds, counts, quantiles)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	// rollupChunk is how many seconds of one-minute rows one read holds, a multiple of an hour.
	rollupChunk = 24 * 3600
)

// hourAcc merges the one-minute rows of one hour. Bucket counts are added (Histogram.Merge), so a
// percentile of the hour, or of several hours merged again by a query, is computed from the real
// distribution and not from an average of percentiles.
type hourAcc struct {
	hour int64
	rows int64
	h    Histogram
	q    *quantileMean
}

func (a *hourAcc) add(row Histogram) {
	a.rows++
	a.h.Merge(row)
	a.q.add(row.Count, row.Quantiles)
}

// rollupHistograms summarises the hours of one histogram or summary series inside [from, to) into
// histogram_hourly, like rollupSQL does for numbers. It reads a day of rows at a time. It is a no-op for
// a series without histogram rows.
func rollupHistograms(tx *gorm.DB, id, from, to int64) error {
	var done []hourAcc
	for start := from; start < to; start += rollupChunk {
		end := min(start+rollupChunk, to)
		hours, err := readHourlyAccs(tx, id, start, end)
		if err != nil {
			return err
		}
		done = append(done, hours...)
	}
	for _, a := range done {
		h := a.h
		h.Quantiles = a.q.result()
		if err := storeHistogram(tx, upsertHistogramHourly, []any{id, a.hour, a.rows}, h); err != nil {
			return err
		}
	}
	return nil
}

func readHourlyAccs(tx *gorm.DB, id, start, end int64) ([]hourAcc, error) {
	rows, err := tx.Raw("SELECT ts, "+histogramColumns+" FROM histogram_samples WHERE series_id = ? AND ts >= ? AND ts < ? ORDER BY ts", id, start, end).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []hourAcc
	for rows.Next() {
		var ts int64
		var r histogramRow
		if err := rows.Scan(append([]any{&ts}, r.scanArgs()...)...); err != nil {
			return nil, err
		}
		hour := ts / 3600 * 3600
		if len(out) == 0 || out[len(out)-1].hour != hour {
			out = append(out, hourAcc{hour: hour, q: newQuantileMean()})
		}
		out[len(out)-1].add(r.histogram())
	}
	return out, rows.Err()
}
