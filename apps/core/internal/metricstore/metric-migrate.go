package metricstore

import "gorm.io/gorm"

// sumDeltasKey marks a database whose OTLP sums are stored as per-interval changes.
const sumDeltasKey = "otlp_sum_deltas"

// migrate brings data written by an older Core in line with the current storage rules. The schema is
// created with IF NOT EXISTS; the one column added to an existing table since (series_state.last_hash) is
// added here when the file lacks it.
//
// One rule changed meaning: an OTLP sum used to be stored as the cumulative value the exporter sent, and
// is now stored as the change per minute. Old rows cannot be told apart from new ones, and read as changes
// they would show up as a huge first value, so once, on the first start after the upgrade, the history of
// the existing OTLP sum series is removed (the series stay and fill again from the next readings).
func (s *Store) migrate() error {
	if err := s.addStateHash(); err != nil {
		return err
	}
	done, ok, err := s.readMeta(sumDeltasKey)
	if err != nil || (ok && done == 1) {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		const sums = "SELECT id FROM series WHERE source = 'otlp' AND kind = 'sum'"
		for _, stmt := range []string{
			"DELETE FROM samples WHERE series_id IN (" + sums + ")",
			"DELETE FROM samples_hourly WHERE series_id IN (" + sums + ")",
			"DELETE FROM dirty_hours WHERE series_id IN (" + sums + ")",
		} {
			if err := tx.Exec(stmt).Error; err != nil {
				return err
			}
		}
		return tx.Exec("INSERT INTO meta (k, v) VALUES (?, 1) ON CONFLICT(k) DO UPDATE SET v = 1", sumDeltasKey).Error
	})
}

func (s *Store) readMeta(key string) (int64, bool, error) {
	var v []int64
	if err := s.db.Raw("SELECT v FROM meta WHERE k = ?", key).Scan(&v).Error; err != nil || len(v) == 0 {
		return 0, false, err
	}
	return v[0], true, nil
}

// addStateHash adds series_state.last_hash to a metrics.db created before delta de-duplication. An empty
// hash matches nothing, so the first delta reading after the upgrade is simply accepted.
func (s *Store) addStateHash() error {
	var n int
	if err := s.db.Raw("SELECT COUNT(*) FROM pragma_table_info('series_state') WHERE name = 'last_hash'").Scan(&n).Error; err != nil || n > 0 {
		return err
	}
	return s.db.Exec("ALTER TABLE series_state ADD COLUMN last_hash TEXT NOT NULL DEFAULT ''").Error
}
