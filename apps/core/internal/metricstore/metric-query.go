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

const hourlyQuery = `SELECT (hour / ?) * ? AS t, AVG(cpu_avg), MAX(cpu_max), AVG(mem_avg), MAX(mem_max),
 AVG(io_read_avg), AVG(io_write_avg), AVG(net_rx_avg), AVG(net_tx_avg), MAX(pids_max), MAX(storage_max), AVG(value_avg), MAX(value_max)
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

	for _, ref := range q.Series {
		var meta []struct {
			ID                               int64
			Kind, Name, Metric, Unit, Labels string
		}
		if err := s.db.Raw("SELECT id, kind, name, metric, unit, labels FROM series WHERE source = ? AND node_id = ? AND app_id = ? AND key = ?",
			ref.Source, ref.NodeID, ref.AppID, ref.Key).Scan(&meta).Error; err != nil {
			return res, err
		}
		if len(meta) == 0 {
			continue
		}
		points, err := s.readPoints(sqlText, step, meta[0].ID, q.From.Unix(), q.To.Unix())
		if err != nil {
			return res, err
		}
		res.Series = append(res.Series, SeriesResult{Ref: ref, Kind: meta[0].Kind, Name: meta[0].Name, Metric: meta[0].Metric,
			Unit: meta[0].Unit, Labels: meta[0].Labels, Points: points})
	}
	return res, nil
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
