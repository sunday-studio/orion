package logstore

import (
	"fmt"
	"sort"
)

// Count is one facet value and how many entries carry it.
type Count struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// Facets summarises the entries matching a query for the filter rail.
type Facets struct {
	Total    int64            `json:"total"`
	Severity map[string]int64 `json:"severity"`
	Apps     []Count          `json:"apps"`
	Events   []Count          `json:"events"`
	// EventsDistinct is how many different events exist, so the rail can say "Show 41 more".
	EventsDistinct int `json:"events_distinct"`
}

// Facets counts severities, apps and events over the query's filters (paging is ignored).
func (s *Store) Facets(q Query, topEvents int) (Facets, error) {
	q.Cursor = ""
	where, args, err := q.where()
	if err != nil {
		return Facets{}, err
	}
	out := Facets{Severity: map[string]int64{}, Apps: []Count{}, Events: []Count{}}
	table := facetTable(q)
	// One pass gives the severity totals and the per-app counts.
	var pairs []struct {
		Severity string
		AppID    string
		N        int64
	}
	if err := s.dbq(q).Raw("SELECT severity, app_id, COUNT(*) AS n FROM "+table+where+" GROUP BY severity, app_id", args...).Scan(&pairs).Error; err != nil {
		return out, err
	}
	perApp := map[string]int64{}
	for _, r := range pairs {
		out.Severity[r.Severity] += r.N
		out.Total += r.N
		perApp[r.AppID] += r.N
	}
	for id, n := range perApp {
		out.Apps = append(out.Apps, Count{Value: id, Count: n})
	}
	sort.Slice(out.Apps, func(i, j int) bool {
		if out.Apps[i].Count != out.Apps[j].Count {
			return out.Apps[i].Count > out.Apps[j].Count
		}
		return out.Apps[i].Value < out.Apps[j].Value
	})
	if len(out.Apps) > 50 {
		out.Apps = out.Apps[:50]
	}
	eventSQL, eventArgs := eventCountsSQL(q, where, args)
	var events []struct {
		Event string
		N     int64
	}
	if err := s.dbq(q).Raw(eventSQL, eventArgs...).Scan(&events).Error; err != nil {
		return out, err
	}
	out.EventsDistinct = len(events)
	for i, r := range events {
		if i >= topEvents {
			break
		}
		out.Events = append(out.Events, Count{Value: r.Event, Count: r.N})
	}
	return out, nil
}

// eventCountsSQL counts the entries per event name. SQLite likes to answer this from the partial index
// on (app_id, event, ts), which holds every entry of the app and cannot narrow by time, so with an app
// filter the app and time index is pinned (it narrows by both).
func eventCountsSQL(q Query, where string, args []any) (string, []any) {
	table := facetTable(q)
	if !q.From.IsZero() && len(q.AppIDs) > 0 {
		table = "log_entries INDEXED BY idx_log_entries_app_ts"
	}
	clause := " WHERE event <> ''"
	if where != "" {
		clause = where + " AND event <> ''"
	}
	return "SELECT event, COUNT(*) AS n FROM " + table + clause + " GROUP BY event ORDER BY n DESC, event ASC", args
}

// facetTable is the table expression the facet counts read. With a time range and no app filter it
// pins the time index. Without that, grouping by app makes SQLite walk a whole app index in order to
// avoid sorting, which reads every entry ever stored however short the range is.
func facetTable(q Query) string {
	if q.From.IsZero() || len(q.AppIDs) > 0 {
		return "log_entries"
	}
	return "log_entries INDEXED BY idx_log_entries_ts"
}

// FieldValues returns the most common values of one attribute among the matching entries.
func (s *Store) FieldValues(q Query, field string, limit int) ([]Count, error) {
	if !validAttributeKey(field) {
		return nil, fmt.Errorf("%w: field must be a plain attribute key", ErrBadFilter)
	}
	q.Cursor = ""
	where, args, err := q.where()
	if err != nil {
		return nil, err
	}
	expr := attrExpr(field)
	clause := expr + " IS NOT NULL"
	if where == "" {
		where = " WHERE " + clause
	} else {
		where += " AND " + clause
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	var rows []struct {
		V string
		N int64
	}
	if err := s.dbq(q).Raw("SELECT "+expr+" AS v, COUNT(*) AS n FROM log_entries"+where+" GROUP BY v ORDER BY n DESC, v ASC LIMIT ?", append(args, limit)...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Count, 0, len(rows))
	for _, r := range rows {
		out = append(out, Count{Value: r.V, Count: r.N})
	}
	return out, nil
}
