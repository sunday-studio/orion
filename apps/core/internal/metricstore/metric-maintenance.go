package metricstore

import (
	"context"
	"orion/core/internal/utils"
	"time"
)

const (
	rollupThroughKey = "rollup_through"
	rollupChunk      = 24 * time.Hour      // one transaction per day of raw rows
	rollupMaxPerRun  = 14 * 24 * time.Hour // a first run on a large file catches up over several passes
)

const rollupSQL = `INSERT OR REPLACE INTO samples_hourly
 (series_id, hour, n, cpu_avg, cpu_max, mem_avg, mem_max, io_read_avg, io_write_avg, net_rx_avg, net_tx_avg, pids_max, storage_max, value_avg, value_max)
SELECT series_id, (ts / 3600) * 3600, COUNT(*), AVG(cpu_percent), MAX(cpu_percent), AVG(memory_bytes), MAX(memory_bytes),
 AVG(io_read_bps), AVG(io_write_bps), AVG(net_rx_bps), AVG(net_tx_bps), MAX(pids), MAX(storage_bytes), AVG(value), MAX(value)
FROM samples WHERE ts >= ? AND ts < ? GROUP BY series_id, ts / 3600`

func (s *Store) rolledThrough() (int64, bool, error) {
	var v []int64
	if err := s.db.Raw("SELECT v FROM meta WHERE k = ?", rollupThroughKey).Scan(&v).Error; err != nil || len(v) == 0 {
		return 0, false, err
	}
	return v[0], true, nil
}

// Rollup turns complete hours of one-minute rows into hourly rows and returns how many hours it
// processed. It starts one hour before where the last run stopped, so a late sample is picked up,
// and the work is idempotent. The current, incomplete hour is never rolled.
func (s *Store) Rollup(now time.Time) (int, error) {
	end := now.UTC().Truncate(time.Hour).Unix()
	start, ok, err := s.rolledThrough()
	if err != nil {
		return 0, err
	}
	if ok {
		start -= 3600
	} else {
		var first []int64
		if err := s.db.Raw("SELECT MIN(ts) FROM samples").Scan(&first).Error; err != nil || len(first) == 0 || first[0] == 0 {
			return 0, err
		}
		start = first[0] / 3600 * 3600
	}
	if limit := start + int64(rollupMaxPerRun/time.Second); end > limit {
		end = limit
	}
	hours := 0
	for from := start; from < end; from += int64(rollupChunk / time.Second) {
		to := min(from+int64(rollupChunk/time.Second), end)
		if err := s.db.Exec(rollupSQL, from, to).Error; err != nil {
			return hours, err
		}
		hours += int((to - from) / 3600)
	}
	if end > start {
		if err := s.db.Exec("INSERT INTO meta (k, v) VALUES (?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v", rollupThroughKey, end).Error; err != nil {
			return hours, err
		}
	}
	return hours, nil
}

// Prune deletes one-minute rows older than r.Raw and hourly rows older than r.Rollup, and then the
// series that have not reported for longer than r.Rollup. One-minute rows that have not been
// rolled up yet are kept, so retention can never delete history that has no summary.
func (s *Store) Prune(r Retention, now time.Time) (PruneResult, error) {
	var res PruneResult
	rawCutoff := now.UTC().Add(-r.Raw).Unix()
	if through, ok, err := s.rolledThrough(); err != nil {
		return res, err
	} else if !ok {
		rawCutoff = 0
	} else if rawCutoff > through {
		rawCutoff = through
	}
	hourlyCutoff := now.UTC().Add(-r.Rollup).Unix()

	var ids []int64
	if err := s.db.Raw("SELECT id FROM series").Scan(&ids).Error; err != nil {
		return res, err
	}
	for _, id := range ids {
		raw := s.db.Exec("DELETE FROM samples WHERE series_id = ? AND ts < ?", id, rawCutoff)
		if raw.Error != nil {
			return res, raw.Error
		}
		hourly := s.db.Exec("DELETE FROM samples_hourly WHERE series_id = ? AND hour < ?", id, hourlyCutoff)
		if hourly.Error != nil {
			return res, hourly.Error
		}
		res.RawRows += raw.RowsAffected
		res.HourlyRows += hourly.RowsAffected
	}
	var stale []int64
	if err := s.db.Raw("SELECT id FROM series WHERE last_seen < ?", hourlyCutoff).Scan(&stale).Error; err != nil {
		return res, err
	}
	for _, id := range stale {
		if err := s.db.Exec("DELETE FROM samples WHERE series_id = ?", id).Error; err != nil {
			return res, err
		}
		if err := s.db.Exec("DELETE FROM samples_hourly WHERE series_id = ?", id).Error; err != nil {
			return res, err
		}
		if err := s.db.Exec("DELETE FROM series WHERE id = ?", id).Error; err != nil {
			return res, err
		}
		res.Series++
	}
	s.db.Exec("PRAGMA incremental_vacuum(2000)")
	s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return res, nil
}

// RunMaintenance rolls up and prunes on every tick until ctx ends. retention is read each time, so a
// change in Settings applies without a restart.
func RunMaintenance(ctx context.Context, store MetricStore, retention func() Retention, logger *utils.Logger, every time.Duration) {
	run := func() {
		now := time.Now().UTC()
		if _, err := store.Rollup(now); err != nil {
			logger.Warn("Metric rollup failed", "error", err)
			return
		}
		if _, err := store.Prune(retention(), now); err != nil {
			logger.Warn("Metric prune failed", "error", err)
		}
	}
	run()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
