package metricstore

import (
	"context"
	"orion/core/internal/utils"
	"time"
)

const (
	rollupThroughKey = "rollup_through"
	rollupMaxPerRun  = 14 * 24 * time.Hour // a first run on a large file catches up over several passes
	dirtyBatch       = 1000
	dirtyMaxBatches  = 50
)

// rollupSQL summarises the hours of one series inside [from, to). It reads through the primary key
// (series_id, ts), so its cost is that series' rows in the range, not the size of the table.
const rollupSQL = `INSERT OR REPLACE INTO samples_hourly
 (series_id, hour, n, cpu_avg, cpu_max, mem_avg, mem_max, io_read_avg, io_write_avg, net_rx_avg, net_tx_avg, pids_max, storage_max, value_avg, value_max)
SELECT series_id, (ts / 3600) * 3600, COUNT(*), AVG(cpu_percent), MAX(cpu_percent), AVG(memory_bytes), MAX(memory_bytes),
 AVG(io_read_bps), AVG(io_write_bps), AVG(net_rx_bps), AVG(net_tx_bps), MAX(pids), MAX(storage_bytes), AVG(value), MAX(value)
FROM samples WHERE series_id = ? AND ts >= ? AND ts < ? GROUP BY series_id, ts / 3600`

func (s *Store) readRolledThrough() (int64, bool, error) {
	var v []int64
	if err := s.db.Raw("SELECT v FROM meta WHERE k = ?", rollupThroughKey).Scan(&v).Error; err != nil || len(v) == 0 {
		return 0, false, err
	}
	return v[0], true, nil
}

// Rollup turns complete hours of one-minute rows into hourly rows and returns how many hours it
// processed. Work is done series by series, each in its own short statement, so the writer is never
// blocked behind a long scan. A pass starts one hour before where the last one stopped; samples that
// arrived later than that (marked dirty by the writer) are summarised again afterwards. The current,
// incomplete hour is never rolled.
func (s *Store) Rollup(now time.Time) (int, error) {
	end := now.UTC().Truncate(time.Hour).Unix()
	start, ok, err := s.readRolledThrough()
	if err != nil {
		return 0, err
	}
	var ids []int64
	if ok {
		start -= 3600
		if err := s.db.Raw("SELECT id FROM series WHERE last_seen >= ?", start-3600).Scan(&ids).Error; err != nil {
			return 0, err
		}
	} else {
		if err := s.db.Raw("SELECT id FROM series").Scan(&ids).Error; err != nil {
			return 0, err
		}
		start = end
		for _, id := range ids { // the oldest row of each series is one index seek
			var first []int64
			if err := s.db.Raw("SELECT COALESCE(MIN(ts), 0) FROM samples WHERE series_id = ?", id).Scan(&first).Error; err != nil {
				return 0, err
			}
			if len(first) > 0 && first[0] > 0 && first[0]/3600*3600 < start {
				start = first[0] / 3600 * 3600
			}
		}
	}
	if limit := start + int64(rollupMaxPerRun/time.Second); end > limit {
		end = limit
	}
	hours := 0
	if end > start {
		for _, id := range ids {
			if err := s.db.Exec(rollupSQL, id, start, end).Error; err != nil {
				return hours, err
			}
		}
		hours = int((end - start) / 3600)
		if err := s.db.Exec("INSERT INTO meta (k, v) VALUES (?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v", rollupThroughKey, end).Error; err != nil {
			return hours, err
		}
		s.rolledThrough.Store(end)
	}
	return hours, s.rollupDirtyHours()
}

// rollupDirtyHours summarises again the (series, hour) pairs that received samples after their hour
// was rolled up.
func (s *Store) rollupDirtyHours() error {
	for i := 0; i < dirtyMaxBatches; i++ {
		var dirty []struct{ SeriesID, Hour int64 }
		if err := s.db.Raw("SELECT series_id, hour FROM dirty_hours LIMIT ?", dirtyBatch).Scan(&dirty).Error; err != nil {
			return err
		}
		for _, d := range dirty {
			if err := s.db.Exec(rollupSQL, d.SeriesID, d.Hour, d.Hour+3600).Error; err != nil {
				return err
			}
			if err := s.db.Exec("DELETE FROM dirty_hours WHERE series_id = ? AND hour = ?", d.SeriesID, d.Hour).Error; err != nil {
				return err
			}
		}
		if len(dirty) < dirtyBatch {
			return nil
		}
	}
	return nil
}

// Prune deletes one-minute rows older than r.Raw and hourly rows older than r.Rollup, and then the
// series that have not reported for longer than r.Rollup. One-minute rows that are not summarised
// yet (after the rollup watermark, or in a dirty hour) are kept, so retention can never delete history
// that has no summary. The raw cutoff is aligned to the hour so a summarised hour is never half deleted.
func (s *Store) Prune(r Retention, now time.Time) (PruneResult, error) {
	var res PruneResult
	rawCutoff := now.UTC().Add(-r.Raw).Unix() / 3600 * 3600
	if through, ok, err := s.readRolledThrough(); err != nil {
		return res, err
	} else if !ok {
		rawCutoff = 0
	} else if rawCutoff > through {
		rawCutoff = through
	}
	var oldestDirty []int64
	if err := s.db.Raw("SELECT COALESCE(MIN(hour), 0) FROM dirty_hours").Scan(&oldestDirty).Error; err != nil {
		return res, err
	}
	if len(oldestDirty) > 0 && oldestDirty[0] > 0 && oldestDirty[0] < rawCutoff {
		rawCutoff = oldestDirty[0]
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
		for _, table := range []string{"samples", "samples_hourly", "dirty_hours"} {
			if err := s.db.Exec("DELETE FROM "+table+" WHERE series_id = ?", id).Error; err != nil {
				return res, err
			}
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
