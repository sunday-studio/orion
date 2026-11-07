package metricstore

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type seriesKey struct{ source, nodeID, appID, key string }

type cachedSeries struct {
	id       int64
	name     string
	lastSeen int64
}

const seriesTouchEvery = int64(time.Hour / time.Second)

const upsertSample = `INSERT INTO samples (series_id, ts, cpu_percent, memory_bytes, io_read_bps, io_write_bps, net_rx_bps, net_tx_bps, pids, storage_bytes, value)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(series_id, ts) DO UPDATE SET cpu_percent = excluded.cpu_percent, memory_bytes = excluded.memory_bytes,
 io_read_bps = excluded.io_read_bps, io_write_bps = excluded.io_write_bps, net_rx_bps = excluded.net_rx_bps,
 net_tx_bps = excluded.net_tx_bps, pids = excluded.pids, storage_bytes = excluded.storage_bytes, value = excluded.value`

// writeBatch stores one batch in a single transaction. Samples over a series cap are dropped and
// counted; they never fail the batch.
func (s *Store) writeBatch(b Batch) error {
	at := b.At
	if at.IsZero() {
		at = time.Now()
	}
	ts := at.UTC().Truncate(time.Minute).Unix()
	now := time.Now().UTC().Unix()
	var written, dropped int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, smp := range b.Samples {
			id, ok, err := s.seriesID(tx, b, smp, now)
			if err != nil {
				return err
			}
			if !ok {
				dropped++
				continue
			}
			v := smp.Values
			if err := tx.Exec(upsertSample, id, ts, v.CPUPercent, v.MemoryBytes, v.IOReadBps, v.IOWriteBps, v.NetRxBps, v.NetTxBps, v.Pids, v.StorageBytes, v.Value).Error; err != nil {
				return err
			}
			written++
		}
		return nil
	})
	if err != nil {
		s.cache = map[seriesKey]cachedSeries{} // a rolled-back insert may have left ids in the cache
		return err
	}
	s.written.Add(written)
	s.droppedCap.Add(dropped)
	return nil
}

func (b Batch) cap() int {
	if b.SeriesCap > 0 {
		return b.SeriesCap
	}
	if b.Source == SourceOTLP {
		return DefaultSeriesCapPerApp
	}
	return DefaultSeriesCapPerNode
}

// seriesID finds or creates the series of a sample. ok is false when the owner is at its cap.
func (s *Store) seriesID(tx *gorm.DB, b Batch, smp Sample, now int64) (int64, bool, error) {
	key := seriesKey{b.Source, b.NodeID, b.AppID, smp.Key}
	name := strings.TrimSpace(smp.Name)
	if c, ok := s.cache[key]; ok {
		if c.name != name || now-c.lastSeen >= seriesTouchEvery {
			if err := tx.Exec("UPDATE series SET name = ?, kind = ?, unit = ?, last_seen = ? WHERE id = ?", name, smp.Kind, smp.Unit, now, c.id).Error; err != nil {
				return 0, false, err
			}
			s.cache[key] = cachedSeries{id: c.id, name: name, lastSeen: now}
		}
		return c.id, true, nil
	}
	var ids []int64
	if err := tx.Raw("SELECT id FROM series WHERE source = ? AND node_id = ? AND app_id = ? AND key = ?", b.Source, b.NodeID, b.AppID, smp.Key).Scan(&ids).Error; err != nil {
		return 0, false, err
	}
	if len(ids) > 0 {
		if err := tx.Exec("UPDATE series SET name = ?, kind = ?, unit = ?, last_seen = ? WHERE id = ?", name, smp.Kind, smp.Unit, now, ids[0]).Error; err != nil {
			return 0, false, err
		}
		s.cache[key] = cachedSeries{id: ids[0], name: name, lastSeen: now}
		return ids[0], true, nil
	}
	var active int
	cutoff := now - int64(ActiveSeriesWindow/time.Second)
	if err := tx.Raw("SELECT COUNT(*) FROM series WHERE source = ? AND node_id = ? AND app_id = ? AND last_seen >= ?", b.Source, b.NodeID, b.AppID, cutoff).Scan(&active).Error; err != nil {
		return 0, false, err
	}
	if active >= b.cap() {
		return 0, false, nil
	}
	if err := tx.Exec(`INSERT INTO series (source, node_id, app_id, key, kind, name, metric, unit, labels, first_seen, last_seen)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, b.Source, b.NodeID, b.AppID, smp.Key, smp.Kind, name, smp.Metric, smp.Unit, smp.Labels, now, now).Error; err != nil {
		return 0, false, err
	}
	var id int64
	if err := tx.Raw("SELECT id FROM series WHERE source = ? AND node_id = ? AND app_id = ? AND key = ?", b.Source, b.NodeID, b.AppID, smp.Key).Scan(&id).Error; err != nil {
		return 0, false, err
	}
	s.cache[key] = cachedSeries{id: id, name: name, lastSeen: now}
	return id, true, nil
}
