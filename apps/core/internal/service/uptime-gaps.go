package service

import (
	"orion/core/internal/db"
	"sort"
	"time"

	"gorm.io/gorm"
)

// timeSpan is a half-open interval [start, end).
type timeSpan struct{ start, end time.Time }

func (t timeSpan) seconds() float64 {
	if !t.end.After(t.start) {
		return 0
	}
	return t.end.Sub(t.start).Seconds()
}

// mergeSpans returns the union of spans as sorted, non-overlapping spans, so overlapping pauses and
// maintenance windows are never subtracted twice.
func mergeSpans(spans []timeSpan) []timeSpan {
	valid := make([]timeSpan, 0, len(spans))
	for _, s := range spans {
		if s.end.After(s.start) {
			valid = append(valid, s)
		}
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].start.Before(valid[j].start) })
	var out []timeSpan
	for _, s := range valid {
		if n := len(out); n > 0 && !s.start.After(out[n-1].end) {
			if s.end.After(out[n-1].end) {
				out[n-1].end = s.end
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

// uncoveredSeconds is the length of span not covered by the merged excluded spans.
func uncoveredSeconds(span timeSpan, excluded []timeSpan) float64 {
	total := span.seconds()
	for _, e := range excluded {
		start, end := e.start, e.end
		if start.Before(span.start) {
			start = span.start
		}
		if end.After(span.end) {
			end = span.end
		}
		total -= timeSpan{start, end}.seconds()
	}
	if total < 0 {
		return 0
	}
	return total
}

// silentMissedChecks counts the checks that should have run during one silence (the time between two
// reports, or between the last report and now). A silence only counts once it is longer than the stale
// window, the same threshold health uses to call a monitor stale, so one late check is not downtime.
// Time while the monitor was paused or in maintenance is not counted. The first interval is skipped
// because the next check is not due until a full interval after the last report.
func silentMissedChecks(silence timeSpan, interval time.Duration, excluded []timeSpan) int {
	if interval <= 0 {
		return 0
	}
	if silence.seconds() <= staleWindow(int(interval/time.Second), DefaultHealthConfig()).Seconds() {
		return 0
	}
	due := timeSpan{start: silence.start.Add(interval), end: silence.end}
	return int(uncoveredSeconds(due, excluded) / interval.Seconds())
}

// missedChecks totals silentMissedChecks over every silence inside coverage, given the sorted report
// times in it: before the first report, between reports, and after the last one.
func missedChecks(reports []time.Time, coverage timeSpan, interval time.Duration, excluded []timeSpan) int {
	if !coverage.end.After(coverage.start) {
		return 0
	}
	merged := mergeSpans(excluded)
	missed := 0
	prev := coverage.start
	for _, at := range reports {
		if at.Before(coverage.start) || !at.Before(coverage.end) {
			continue
		}
		missed += silentMissedChecks(timeSpan{prev, at}, interval, merged)
		prev = at
	}
	return missed + silentMissedChecks(timeSpan{prev, coverage.end}, interval, merged)
}

// gapAccountingMigration is the migration that started recording paused and maintenance periods.
// Days before it was applied have no such history, so counting their silences would charge paused
// time as downtime; those days keep counting only the reports that arrived.
const gapAccountingMigration = 56

// uptimeMonitor is what gap accounting needs to know about a monitor.
type uptimeMonitor struct {
	ID              string
	NodeID          string
	CreatedAt       time.Time
	Lifecycle       string
	IntervalSeconds int
}

func (m uptimeMonitor) interval() time.Duration {
	if m.IntervalSeconds <= 0 {
		return time.Minute
	}
	return time.Duration(m.IntervalSeconds) * time.Second
}

// coverage is the part of window during which the monitor existed and was meant to run.
func (m uptimeMonitor) coverage(window timeSpan) timeSpan {
	start := window.start
	if c := m.CreatedAt.UTC(); c.After(start) {
		start = c
	}
	return timeSpan{start, window.end}
}

// loadUptimeMonitors loads monitors by id (all active monitors when ids is nil). The interval is the
// core-managed one when set, otherwise the node-reported one.
func loadUptimeMonitors(database *gorm.DB, ids []string) (map[string]uptimeMonitor, error) {
	query := database.Table("monitors AS m").
		Select(`m.id AS id, m.node_id AS node_id, m.created_at AS created_at, m.lifecycle AS lifecycle,
			COALESCE(NULLIF(c.interval_seconds, 0), NULLIF(m.reporting_interval_seconds, 0), 60) AS interval_seconds`).
		Joins("LEFT JOIN core_monitor_configs c ON c.monitor_id = m.id")
	if ids != nil {
		query = query.Where("m.id IN ?", ids)
	} else {
		query = query.Where("m.lifecycle = ?", "active")
	}
	var rows []uptimeMonitor
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]uptimeMonitor, len(rows))
	for _, r := range rows {
		out[r.ID] = r
	}
	return out, nil
}

// loadExcludedSpans returns, per monitor, the merged paused/maintenance spans overlapping window,
// including maintenance on the monitor's node. Open spans run to the end of the window.
func loadExcludedSpans(database *gorm.DB, window timeSpan, monitors map[string]uptimeMonitor) (map[string][]timeSpan, error) {
	var rows []db.UptimeExclusion
	if err := database.Where("starts_at < ? AND (ends_at IS NULL OR ends_at > ?)", window.end, window.start).Find(&rows).Error; err != nil {
		return nil, err
	}
	bySubject := make(map[string][]timeSpan)
	for _, r := range rows {
		end := window.end
		if r.EndsAt != nil {
			end = r.EndsAt.UTC()
		}
		key := r.SubjectType + "|" + r.SubjectID
		bySubject[key] = append(bySubject[key], timeSpan{r.StartsAt.UTC(), end})
	}
	out := make(map[string][]timeSpan, len(monitors))
	for id, m := range monitors {
		var spans []timeSpan
		spans = append(spans, bySubject["monitor_pause|"+id]...)
		spans = append(spans, bySubject["monitor_maintenance|"+id]...)
		if m.NodeID != "" {
			spans = append(spans, bySubject["node_maintenance|"+m.NodeID]...)
		}
		if len(spans) > 0 {
			out[id] = mergeSpans(spans)
		}
	}
	return out, nil
}

// gapAccountingStart is the first day that has paused/maintenance history, or the zero time when the
// migration marker is unavailable (then gap accounting stays off rather than guessing).
func gapAccountingStart(database *gorm.DB) time.Time {
	var applied *string
	if err := database.Raw(`SELECT applied_at FROM schema_migrations WHERE version = ?`, gapAccountingMigration).Scan(&applied).Error; err != nil || applied == nil || len(*applied) < 10 {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", (*applied)[:10])
	if err != nil {
		return time.Time{}
	}
	return t
}
