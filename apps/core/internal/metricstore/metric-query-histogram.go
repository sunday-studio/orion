package metricstore

import (
	"math"
	"time"
)

// The quantiles every histogram point reports, and the Point fields they fill.
const (
	quantileEpsilon = 1e-9
)

type stepAcc struct {
	t int64
	h Histogram
	q *quantileMean
}

// readHistogramPoints reads one histogram or summary series: one-minute rows or hourly rollups, merged
// into steps. Buckets are merged, never percentiles, so a point's percentiles come from the distribution
// of the whole step. Rows are small (bounds and counts), and a series has at most 48 hours of minutes or
// 90 days of hours, so the merge happens in code rather than in SQL.
func (s *Store) readHistogramPoints(kind string, hourly bool, step, id, from, to int64) ([]Point, error) {
	text := "SELECT ts, " + histogramColumns + " FROM histogram_samples WHERE series_id = ? AND ts >= ? AND ts < ? ORDER BY ts"
	if hourly {
		text = "SELECT hour, " + histogramColumns + " FROM histogram_hourly WHERE series_id = ? AND hour >= ? AND hour < ? ORDER BY hour"
	}
	rows, err := s.db.Raw(text, id, from, to).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var steps []*stepAcc
	for rows.Next() {
		var ts int64
		var r histogramRow
		if err := rows.Scan(append([]any{&ts}, r.scanArgs()...)...); err != nil {
			return nil, err
		}
		t := ts / step * step
		if len(steps) == 0 || steps[len(steps)-1].t != t {
			steps = append(steps, &stepAcc{t: t, q: newQuantileMean()})
		}
		acc := steps[len(steps)-1]
		row := r.histogram()
		acc.h.Merge(row)
		acc.q.add(row.Count, row.Quantiles)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	points := make([]Point, 0, len(steps))
	for _, acc := range steps {
		points = append(points, histogramPoint(kind, acc))
	}
	return points, nil
}

// histogramPoint is what one step of a histogram or summary series reports. A value that cannot be
// computed is absent, never zero: no average without observations or a sum, no percentile without buckets
// (a histogram) or without that quantile (a summary).
func histogramPoint(kind string, acc *stepAcc) Point {
	h := acc.h
	count := h.Count
	p := Point{T: time.Unix(acc.t, 0).UTC(), Count: &count, Sum: h.Sum, Min: h.Min, Max: h.Max}
	if h.Sum != nil && count > 0 {
		avg := *h.Sum / float64(count)
		p.Avg = &avg
	}
	if kind == KindSummary {
		p.Quantiles = acc.q.result()
		p.P50, p.P95, p.P99 = quantileOf(p.Quantiles, 0.5), quantileOf(p.Quantiles, 0.95), quantileOf(p.Quantiles, 0.99)
		return p
	}
	for _, pc := range []struct {
		q   float64
		dst **float64
	}{{0.5, &p.P50}, {0.95, &p.P95}, {0.99, &p.P99}} {
		if v, ok := h.Percentile(pc.q); ok {
			value := v
			*pc.dst = &value
		}
	}
	return p
}

func quantileOf(qs []Quantile, want float64) *float64 {
	for _, q := range qs {
		if math.Abs(q.Q-want) < quantileEpsilon {
			v := q.V
			return &v
		}
	}
	return nil
}
