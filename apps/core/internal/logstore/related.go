package logstore

import "time"

// Related returns the entries that belong with e: the same request, else the same trace, else the
// lines just before and after it in the same app. by names which rule applied.
func (s *Store) Related(e Entry, limit int) (by string, total int64, entries []Entry, err error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	var column, value string
	switch {
	case e.RequestID != "":
		by, column, value = "request_id", "request_id", e.RequestID
	case e.TraceID != "":
		by, column, value = "trace_id", "trace_id", e.TraceID
	}
	if column != "" {
		if err = s.db.Raw("SELECT COUNT(*) FROM log_entries WHERE "+column+" = ?", value).Scan(&total).Error; err != nil {
			return
		}
		var rows []logRow
		if err = s.db.Raw("SELECT * FROM log_entries WHERE "+column+" = ? ORDER BY ts ASC, id ASC LIMIT ?", value, limit).Scan(&rows).Error; err != nil {
			return
		}
		for _, r := range rows {
			entries = append(entries, r.entry())
		}
		return
	}
	by = "nearby"
	half := max(limit/2, 1)
	var before, after []logRow
	ts := e.Timestamp.UnixNano()
	if err = s.db.Raw("SELECT * FROM log_entries WHERE app_id = ? AND (ts < ? OR (ts = ? AND id < ?)) ORDER BY ts DESC, id DESC LIMIT ?", e.AppID, ts, ts, e.ID, half).Scan(&before).Error; err != nil {
		return
	}
	if err = s.db.Raw("SELECT * FROM log_entries WHERE app_id = ? AND (ts > ? OR (ts = ? AND id >= ?)) ORDER BY ts ASC, id ASC LIMIT ?", e.AppID, ts, ts, e.ID, half+1).Scan(&after).Error; err != nil {
		return
	}
	for i := len(before) - 1; i >= 0; i-- {
		entries = append(entries, before[i].entry())
	}
	for _, r := range after {
		entries = append(entries, r.entry())
	}
	return by, int64(len(entries)), entries, nil
}

// Usage is the stored volume, cached briefly because it scans the table.
type Usage struct {
	Entries    int64
	Bytes      int64
	TodayLines int64
}

// Usage returns total entries, bytes and the lines stored since the start of the UTC day.
func (s *Store) Usage(now time.Time) (Usage, error) {
	s.usageMu.Lock()
	defer s.usageMu.Unlock()
	if !s.usageAt.IsZero() && now.Sub(s.usageAt) < 15*time.Second {
		return s.usageCache, nil
	}
	var u Usage
	if err := s.db.Raw("SELECT COUNT(*) AS entries, COALESCE(SUM(size),0) AS bytes FROM log_entries").Scan(&u).Error; err != nil {
		return u, err
	}
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if err := s.db.Raw("SELECT COUNT(*) FROM log_entries WHERE ts >= ?", startOfDay.UnixNano()).Scan(&u.TodayLines).Error; err != nil {
		return u, err
	}
	s.usageCache, s.usageAt = u, now
	return u, nil
}

// AppStats is what the Apps list shows for one app.
type AppStats struct {
	AppID      string
	Entries    int64
	Bytes      int64
	LastMinute int64
}

// StatsByApp returns stored volume and the last minute's rate for every app that has entries.
func (s *Store) StatsByApp(now time.Time) (map[string]AppStats, error) {
	var rows []AppStats
	if err := s.db.Raw("SELECT app_id, COUNT(*) AS entries, COALESCE(SUM(size),0) AS bytes FROM log_entries GROUP BY app_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]AppStats, len(rows))
	for _, r := range rows {
		out[r.AppID] = r
	}
	var recent []struct {
		AppID string
		N     int64
	}
	if err := s.db.Raw("SELECT app_id, COUNT(*) AS n FROM log_entries WHERE ts >= ? GROUP BY app_id", now.Add(-time.Minute).UnixNano()).Scan(&recent).Error; err != nil {
		return nil, err
	}
	for _, r := range recent {
		a := out[r.AppID]
		a.AppID, a.LastMinute = r.AppID, r.N
		out[r.AppID] = a
	}
	return out, nil
}
