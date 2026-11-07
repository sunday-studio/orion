package metricstore

import "time"

const allowChunk = 500

// AllowSeries decides which series keys of one owner may be written: those that already exist, and
// new ones while the owner is under cap. Keys are considered in order, so the first new series win.
// It lets an ingest request report rejected data points synchronously; the writer enforces the same
// cap again, so a race can only drop a sample, never exceed the cap.
func (s *Store) AllowSeries(source, nodeID, appID string, keys []string, cap int) (map[string]bool, error) {
	if cap <= 0 {
		cap = Batch{Source: source}.cap()
	}
	existing := map[string]bool{}
	for start := 0; start < len(keys); start += allowChunk {
		end := min(start+allowChunk, len(keys))
		var found []string
		if err := s.db.Raw("SELECT key FROM series WHERE source = ? AND node_id = ? AND app_id = ? AND key IN ?", source, nodeID, appID, keys[start:end]).Scan(&found).Error; err != nil {
			return nil, err
		}
		for _, k := range found {
			existing[k] = true
		}
	}
	var active int
	cutoff := time.Now().UTC().Add(-ActiveSeriesWindow).Unix()
	if err := s.db.Raw("SELECT COUNT(*) FROM series WHERE source = ? AND node_id = ? AND app_id = ? AND last_seen >= ?", source, nodeID, appID, cutoff).Scan(&active).Error; err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(keys))
	for _, k := range keys {
		switch {
		case existing[k]:
			allowed[k] = true
		case active < cap:
			allowed[k] = true
			active++
		}
	}
	return allowed, nil
}
