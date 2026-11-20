package metricstore

import "gorm.io/gorm"

// sumDeltasKey marks a database whose OTLP sums are stored as per-interval changes.
const sumDeltasKey = "otlp_sum_deltas"

// migrate brings data written by an older Core in line with the current storage rules. The schema itself
// needs no migration: every table is created with IF NOT EXISTS, the new ones are separate tables and no
// existing table gained a column.
//
// One rule changed meaning: an OTLP sum used to be stored as the cumulative value the exporter sent, and
// is now stored as the change per minute. Old rows cannot be told apart from new ones, and read as changes
// they would show up as a huge first value, so once, on the first start after the upgrade, the history of
// the existing OTLP sum series is removed (the series stay and fill again from the next readings).
func (s *Store) migrate() error {
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
