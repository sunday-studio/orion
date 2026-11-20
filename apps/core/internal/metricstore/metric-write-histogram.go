package metricstore

import (
	"database/sql"

	"gorm.io/gorm"
)

const upsertHistogram = `INSERT INTO histogram_samples (series_id, ts, count, sum, min, max, bounds, counts, quantiles)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(series_id, ts) DO UPDATE SET count = excluded.count, sum = excluded.sum, min = excluded.min, max = excluded.max,
 bounds = excluded.bounds, counts = excluded.counts, quantiles = excluded.quantiles`

const histogramColumns = "count, sum, min, max, bounds, counts, quantiles"

// histogramRow is a stored histogram or summary: a one-minute row, or an hourly rollup (which also holds
// the number of one-minute rows it summarises in N).
type histogramRow struct {
	Count                    int64
	Sum, Min, Max            sql.NullFloat64
	Bounds, Counts, Quantile string
}

func (r *histogramRow) scanArgs() []any {
	return []any{&r.Count, &r.Sum, &r.Min, &r.Max, &r.Bounds, &r.Counts, &r.Quantile}
}

func (r histogramRow) histogram() Histogram {
	h := Histogram{Count: r.Count, Bounds: decodeFloats(r.Bounds), Counts: decodeInts(r.Counts), Quantiles: decodeQuantiles(r.Quantile)}
	if r.Sum.Valid {
		h.Sum = &r.Sum.Float64
	}
	if r.Min.Valid {
		h.Min = &r.Min.Float64
	}
	if r.Max.Valid {
		h.Max = &r.Max.Float64
	}
	return h
}

// storeHistogram writes h as one row. A row without observations keeps no buckets: they would only repeat
// the bounds of the series.
func storeHistogram(tx *gorm.DB, query string, args []any, h Histogram) error {
	bounds, counts := "", ""
	if h.Count > 0 {
		bounds, counts = encodeFloats(h.Bounds), encodeInts(h.Counts)
	}
	return tx.Exec(query, append(args, h.Count, h.Sum, h.Min, h.Max, bounds, counts, encodeQuantiles(h.Quantiles))...).Error
}

// writeHistogram adds a change to the histogram its minute already holds (same bounds add bucket by
// bucket; see Histogram.Merge for the rest). A summary keeps the newest quantiles.
func writeHistogram(tx *gorm.DB, id, ts int64, h Histogram) error {
	rows, err := tx.Raw("SELECT "+histogramColumns+" FROM histogram_samples WHERE series_id = ? AND ts = ?", id, ts).Rows()
	if err != nil {
		return err
	}
	var current *Histogram
	if rows.Next() {
		var r histogramRow
		if err := rows.Scan(r.scanArgs()...); err != nil {
			rows.Close()
			return err
		}
		c := r.histogram()
		current = &c
	}
	rows.Close()
	if current != nil {
		current.Merge(h)
		if len(h.Quantiles) > 0 {
			current.Quantiles = h.Quantiles
		}
		h = *current
	}
	return storeHistogram(tx, upsertHistogram, []any{id, ts}, h)
}
