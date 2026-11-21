package logstore

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
