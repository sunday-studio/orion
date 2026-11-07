package logstore

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// minuteCountsSchema is the per-app minute rollup: how many entries of each severity an app logged in
// each minute. The writer keeps it in the same transaction as the entries, so the error lane of a
// timeline never has to count raw rows.
const minuteCountsSchema = `
CREATE TABLE IF NOT EXISTS log_minute_counts (
  app_id TEXT NOT NULL,
  minute INTEGER NOT NULL,
  severity TEXT NOT NULL,
  n INTEGER NOT NULL,
  PRIMARY KEY (app_id, minute, severity)
) WITHOUT ROWID;
`

// MinuteCount is the number of entries of one severity in one minute of one app.
type MinuteCount struct {
	AppID    string
	Minute   time.Time
	Severity string
	Count    int64
}

type minuteKey struct {
	app      string
	minute   int64
	severity string
}

// addMinuteCounts adds the given entries to the rollup. Entries the insert ignored (a repeated
// client id) are left out so a retry never counts twice.
func addMinuteCounts(tx *gorm.DB, entries []Entry, stored int64) error {
	counted := entries
	if stored < int64(len(entries)) {
		kept, err := storedOnly(tx, entries)
		if err != nil {
			return err
		}
		counted = kept
	}
	counts := map[minuteKey]int64{}
	for _, e := range counted {
		counts[minuteKey{e.AppID, e.Timestamp.Unix() / 60 * 60, e.Severity}]++
	}
	for k, n := range counts {
		if err := tx.Exec(`INSERT INTO log_minute_counts (app_id, minute, severity, n) VALUES (?, ?, ?, ?)
ON CONFLICT(app_id, minute, severity) DO UPDATE SET n = n + excluded.n`, k.app, k.minute, k.severity, n).Error; err != nil {
			return err
		}
	}
	return nil
}

// storedOnly keeps the entries whose own id is in the table: a row ignored for a repeated client id
// was never stored under its id.
func storedOnly(tx *gorm.DB, entries []Entry) ([]Entry, error) {
	have := make(map[string]bool, len(entries))
	for start := 0; start < len(entries); start += insertChunk {
		end := min(start+insertChunk, len(entries))
		ids := make([]any, 0, end-start)
		for _, e := range entries[start:end] {
			ids = append(ids, e.ID)
		}
		var found []string
		q := "SELECT id FROM log_entries WHERE id IN (?" + strings.Repeat(",?", len(ids)-1) + ")"
		if err := tx.Raw(q, ids...).Scan(&found).Error; err != nil {
			return nil, err
		}
		for _, id := range found {
			have[id] = true
		}
	}
	kept := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if have[e.ID] {
			kept = append(kept, e)
		}
	}
	return kept, nil
}

// MinuteCounts returns the per-minute severity counts of the given apps in [from, to).
func (s *Store) MinuteCounts(appIDs []string, from, to time.Time) ([]MinuteCount, error) {
	if len(appIDs) == 0 {
		return []MinuteCount{}, nil
	}
	var rows []struct {
		AppID    string
		Minute   int64
		Severity string
		N        int64
	}
	if err := s.db.Raw("SELECT app_id, minute, severity, n FROM log_minute_counts WHERE app_id IN ? AND minute >= ? AND minute < ? ORDER BY minute, app_id, severity",
		appIDs, from.Unix()/60*60, to.Unix()).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]MinuteCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, MinuteCount{AppID: r.AppID, Minute: time.Unix(r.Minute, 0).UTC(), Severity: r.Severity, Count: r.N})
	}
	return out, nil
}

// pruneMinuteCounts drops rollup rows older than an app's retention.
func (s *Store) pruneMinuteCounts(appID string, cutoff time.Time) {
	s.db.Exec("DELETE FROM log_minute_counts WHERE app_id = ? AND minute < ?", appID, cutoff.Unix())
}
