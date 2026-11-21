package logstore

import (
	"sync"
	"time"
)

// Usage is the stored volume, cached briefly.
type Usage struct {
	Entries    int64
	Bytes      int64
	TodayLines int64
}

// AppStats is what the Apps list shows for one app.
type AppStats struct {
	AppID      string
	Entries    int64
	Bytes      int64
	LastMinute int64
}

// volume is how much one app has stored.
type volume struct {
	Entries int64
	Bytes   int64
}

const (
	// volumeMinAge and volumeMaxAge bound the spacing of the background scans. An old snapshot is rescanned
	// once it is ten times as old as the last scan took, and a stale one (rows were deleted, or a batch
	// could not be counted) three times, so on a large table the scans use at most a quarter of one core
	// however busy the Apps pages are. Either spacing is at least volumeMinAge and at most volumeMaxAge.
	volumeMinAge = 15 * time.Second
	volumeMaxAge = 10 * time.Minute
)

// volumeCache keeps the number of entries and bytes each app has stored. Counting them takes a scan of
// the whole table, which is seconds on millions of lines, and the Apps pages ask for it on every load.
// So the scan runs once, in the background, and the writer adds what it stores to the snapshot as it
// goes. Deletes and writes that cannot be counted exactly mark it stale, and the next read starts a
// refresh while still answering from the snapshot.
type volumeCache struct {
	mu     sync.Mutex
	perApp map[string]volume
	ready  bool
	stale  bool
	// gen counts the events that make a running scan's answer wrong or out of date (a delete, a batch the
	// writer could not count). A scan that began before the latest one must not clear the stale mark.
	gen     uint64
	fast    bool // tests: refresh without the minimum age
	at      time.Time
	took    time.Duration
	loading chan struct{} // non-nil while a scan runs; closed when it ends
	err     error
}

// due says whether a refresh should start; see volumeMinAge.
func (c *volumeCache) due() bool {
	age := time.Since(c.at)
	staleGap := min(max(3*c.took, volumeMinAge), volumeMaxAge)
	if c.fast {
		staleGap = 0
	}
	return (c.stale && age >= staleGap) || age > min(max(10*c.took, volumeMinAge), volumeMaxAge)
}

// volumes returns a copy of the per-app snapshot. The first call after start waits for the first scan;
// later calls never wait for one.
func (s *Store) volumes() (map[string]volume, error) {
	c := &s.vol
	c.mu.Lock()
	if !c.ready {
		if c.loading == nil {
			s.startVolumeScanLocked()
		}
		done := c.loading
		c.mu.Unlock()
		<-done
		c.mu.Lock()
		if !c.ready {
			err := c.err
			c.mu.Unlock()
			return nil, err
		}
	} else if c.loading == nil && c.due() && !s.closed.Load() { // a reader-only store (OpenReader) is closed, so it never refreshes
		s.startVolumeScanLocked()
	}
	out := make(map[string]volume, len(c.perApp))
	for id, v := range c.perApp {
		out[id] = v
	}
	c.mu.Unlock()
	return out, nil
}

// startVolumeScanLocked starts the scan; the caller holds c.mu and has seen that none is running.
func (s *Store) startVolumeScanLocked() {
	c := &s.vol
	done := make(chan struct{})
	c.loading = done
	startGen := c.gen
	go func() {
		started := time.Now()
		var rows []struct {
			AppID   string
			Entries int64
			Bytes   int64
		}
		err := s.db.Raw("SELECT app_id, COUNT(*) AS entries, COALESCE(SUM(size),0) AS bytes FROM log_entries GROUP BY app_id").Scan(&rows).Error
		c.mu.Lock()
		defer close(done)
		defer c.mu.Unlock()
		c.loading = nil
		c.at = time.Now() // also after a failure, so a broken scan is retried at the normal pace
		if err != nil {
			c.err = err
			return
		}
		c.err = nil
		c.took = time.Since(started)
		c.stale = c.gen != startGen
		c.ready = true
		c.perApp = make(map[string]volume, len(rows))
		for _, r := range rows {
			c.perApp[r.AppID] = volume{Entries: r.Entries, Bytes: r.Bytes}
		}
	}()
}

// volumeAdded counts a committed batch. Rows committed while a scan runs are added to the map the scan
// then replaces, and a scan can also see a batch just before this runs, so the snapshot can be off by
// about the ingest rate times the scan time until the next scan. That is the tolerance of these figures. A batch with rows the insert ignored (a repeated client id)
// cannot be counted without a lookup, so it only marks the snapshot stale.
func (s *Store) volumeAdded(entries []Entry, stored int64) {
	c := &s.vol
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.ready {
		return
	}
	if stored != int64(len(entries)) {
		c.stale = true
		c.gen++
		return
	}
	for _, e := range entries {
		v := c.perApp[e.AppID]
		v.Entries++
		v.Bytes += int64(e.Size)
		c.perApp[e.AppID] = v
	}
}

// volumeShrunk notes that rows were deleted: the snapshot is refreshed on the next read. A whole app
// that was deleted is dropped at once, so the Apps list does not keep showing its size.
func (s *Store) volumeShrunk(appID string) {
	c := &s.vol
	c.mu.Lock()
	defer c.mu.Unlock()
	if appID != "" {
		delete(c.perApp, appID)
	}
	c.stale = true
	c.gen++
}

// WarmVolumes starts the first scan now, so the first Apps page after a start does not wait for it.
func (s *Store) WarmVolumes() {
	go func() { _, _ = s.volumes() }()
}

// Usage returns total entries, bytes and the lines stored since the start of the UTC day.
func (s *Store) Usage(now time.Time) (Usage, error) {
	s.usageMu.Lock()
	defer s.usageMu.Unlock()
	if !s.usageAt.IsZero() && now.Sub(s.usageAt) < 15*time.Second {
		return s.usageCache, nil
	}
	var u Usage
	vols, err := s.volumes()
	if err != nil {
		return u, err
	}
	for _, v := range vols {
		u.Entries += v.Entries
		u.Bytes += v.Bytes
	}
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if err := s.db.Raw("SELECT COUNT(*) FROM log_entries INDEXED BY idx_log_entries_ts WHERE ts >= ?", startOfDay.UnixNano()).Scan(&u.TodayLines).Error; err != nil {
		return u, err
	}
	s.usageCache, s.usageAt = u, now
	return u, nil
}

// StatsByApp returns stored volume and the last minute's rate for every app that has entries.
func (s *Store) StatsByApp(now time.Time) (map[string]AppStats, error) {
	vols, err := s.volumes()
	if err != nil {
		return nil, err
	}
	out := make(map[string]AppStats, len(vols))
	for id, v := range vols {
		if v.Entries > 0 {
			out[id] = AppStats{AppID: id, Entries: v.Entries, Bytes: v.Bytes}
		}
	}
	var recent []struct {
		AppID string
		N     int64
	}
	// INDEXED BY: without it SQLite walks a whole app index to avoid sorting the groups, ignoring the
	// one-minute range, which makes this as slow as a full scan.
	if err := s.db.Raw("SELECT app_id, COUNT(*) AS n FROM log_entries INDEXED BY idx_log_entries_ts WHERE ts >= ? GROUP BY app_id", now.Add(-time.Minute).UnixNano()).Scan(&recent).Error; err != nil {
		return nil, err
	}
	for _, r := range recent {
		a := out[r.AppID]
		a.AppID, a.LastMinute = r.AppID, r.N
		out[r.AppID] = a
	}
	return out, nil
}

// StatsForApp is StatsByApp for one app, without counting the last minute of every other app.
func (s *Store) StatsForApp(appID string, now time.Time) (AppStats, error) {
	vols, err := s.volumes()
	if err != nil {
		return AppStats{}, err
	}
	v := vols[appID]
	out := AppStats{AppID: appID, Entries: v.Entries, Bytes: v.Bytes}
	err = s.db.Raw("SELECT COUNT(*) FROM log_entries WHERE app_id = ? AND ts >= ?", appID, now.Add(-time.Minute).UnixNano()).Scan(&out.LastMinute).Error
	return out, err
}
