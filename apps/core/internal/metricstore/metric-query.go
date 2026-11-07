package metricstore

import (
	"database/sql"
	"time"
)

const (
	defaultMaxPoints = 500
	hardMaxPoints    = 2000
	// hourlyAbove is the range from which a query reads hourly rollups instead of one-minute rows.
	hourlyAbove = 48 * time.Hour
)

const rawQuery = `SELECT (ts / ?) * ? AS t, AVG(cpu_percent), MAX(cpu_percent), AVG(memory_bytes), MAX(memory_bytes),
 AVG(io_read_bps), AVG(io_write_bps), AVG(net_rx_bps), AVG(net_tx_bps), MAX(pids), MAX(storage_bytes), AVG(value), MAX(value)
FROM samples WHERE series_id = ? AND ts >= ? AND ts < ? GROUP BY t ORDER BY t`

// wavg weights an hourly average by the number of rows it summarises, so a short hour does not count as much as a full one.
func wavg(col string) string {
	return "SUM(" + col + " * n) / SUM(CASE WHEN " + col + " IS NOT NULL THEN n END)"
}

var hourlyQuery = `SELECT (hour / ?) * ? AS t, ` + wavg("cpu_avg") + `, MAX(cpu_max), ` + wavg("mem_avg") + `, MAX(mem_max), ` +
	wavg("io_read_avg") + `, ` + wavg("io_write_avg") + `, ` + wavg("net_rx_avg") + `, ` + wavg("net_tx_avg") + `, MAX(pids_max), MAX(storage_max), ` +
	wavg("value_avg") + `, MAX(value_max)
FROM samples_hourly WHERE series_id = ? AND hour >= ? AND hour < ? GROUP BY t ORDER BY t`

// Query implements MetricStore. It reads one-minute rows for ranges up to 48 hours and hourly
// rollups beyond, and widens the step so a series never returns more than MaxPoints points.
func (s *Store) Query(q Query) (QueryResult, error) {
	maxPoints := q.MaxPoints
	if maxPoints <= 0 {
		maxPoints = defaultMaxPoints
	}
	maxPoints = min(maxPoints, hardMaxPoints)
	span := q.To.Sub(q.From)
	hourly := span > hourlyAbove
	base := int64(60)
	sqlText := rawQuery
	res := QueryResult{Resolution: "raw", Series: []SeriesResult{}}
	if hourly {
		base, sqlText, res.Resolution = 3600, hourlyQuery, "hourly"
	}
	step := base
	if span > 0 {
		want := int64(span/time.Second) / int64(maxPoints)
		if want > step {
			step = (want + base - 1) / base * base
		}
	}
	res.StepSeconds = int(step)

	metas, err := s.findSeries(q)
	if err != nil {
		return res, err
	}
	for _, m := range metas {
		points, err := s.readPoints(sqlText, step, m.ID, q.From.Unix(), q.To.Unix())
		if err != nil {
			return res, err
		}
		res.Series = append(res.Series, SeriesResult{Ref: SeriesRef{Source: m.Source, NodeID: m.NodeID, AppID: m.AppID, Key: m.Key},
			Kind: m.Kind, Name: m.Name, Metric: m.Metric, Unit: m.Unit, Labels: m.Labels, Points: points})
	}
	return res, nil
}

type seriesMeta struct {
	ID                                                           int64
	Source, NodeID, AppID, Key, Kind, Name, Metric, Unit, Labels string
}

const seriesColumns = "id, source, node_id, app_id, key, kind, name, metric, unit, labels"

// findSeries resolves the series a query asks for: the listed refs, then the most recently seen
// OTLP series of the listed apps (at most MaxAppSeries per query).
func (s *Store) findSeries(q Query) ([]seriesMeta, error) {
	var out []seriesMeta
	seen := map[int64]bool{}
	for _, ref := range q.Series {
		var found []seriesMeta
		if err := s.db.Raw("SELECT "+seriesColumns+" FROM series WHERE source = ? AND node_id = ? AND app_id = ? AND key = ?",
			ref.Source, ref.NodeID, ref.AppID, ref.Key).Scan(&found).Error; err != nil {
			return nil, err
		}
		for _, m := range found {
			if !seen[m.ID] {
				seen[m.ID] = true
				out = append(out, m)
			}
		}
	}
	if len(q.AppIDs) > 0 {
		var found []seriesMeta
		if err := s.db.Raw("SELECT "+seriesColumns+" FROM series WHERE source = ? AND app_id IN ? ORDER BY last_seen DESC, id LIMIT ?",
			SourceOTLP, q.AppIDs, MaxAppSeries).Scan(&found).Error; err != nil {
			return nil, err
		}
		for _, m := range found {
			if !seen[m.ID] {
				seen[m.ID] = true
				out = append(out, m)
			}
		}
	}
	return out, nil
}

func (s *Store) readPoints(sqlText string, step, id, from, to int64) ([]Point, error) {
	rows, err := s.db.Raw(sqlText, step, step, id, from, to).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points := []Point{}
	for rows.Next() {
		var t int64
		var cpu, cpuMax, mem, memMax, ioR, ioW, rx, tx, val, valMax sql.NullFloat64
		var pids, storage sql.NullInt64
		if err := rows.Scan(&t, &cpu, &cpuMax, &mem, &memMax, &ioR, &ioW, &rx, &tx, &pids, &storage, &val, &valMax); err != nil {
			return nil, err
		}
		points = append(points, Point{T: time.Unix(t, 0).UTC(), CPUPercent: f(cpu), CPUPercentMax: f(cpuMax), MemoryBytes: f(mem), MemoryBytesMax: f(memMax),
			IOReadBps: f(ioR), IOWriteBps: f(ioW), NetRxBps: f(rx), NetTxBps: f(tx), PidsMax: i(pids), StorageMax: i(storage), Value: f(val), ValueMax: f(valMax)})
	}
	return points, rows.Err()
}

func f(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func i(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}
