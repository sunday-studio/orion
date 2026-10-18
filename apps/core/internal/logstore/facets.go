package logstore

import (
	"fmt"
	"sort"
	"strings"
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
	var sev []struct {
		Severity string
		N        int64
	}
	if err := s.dbq(q).Raw("SELECT severity, COUNT(*) AS n FROM log_entries"+where+" GROUP BY severity", args...).Scan(&sev).Error; err != nil {
		return out, err
	}
	for _, r := range sev {
		out.Severity[r.Severity] = r.N
		out.Total += r.N
	}
	var apps []struct {
		AppID string
		N     int64
	}
	if err := s.dbq(q).Raw("SELECT app_id, COUNT(*) AS n FROM log_entries"+where+" GROUP BY app_id ORDER BY n DESC LIMIT 50", args...).Scan(&apps).Error; err != nil {
		return out, err
	}
	for _, r := range apps {
		out.Apps = append(out.Apps, Count{Value: r.AppID, Count: r.N})
	}
	eventWhere := where
	if eventWhere == "" {
		eventWhere = " WHERE event <> ''"
	} else {
		eventWhere += " AND event <> ''"
	}
	var events []struct {
		Event string
		N     int64
	}
	if err := s.dbq(q).Raw("SELECT event, COUNT(*) AS n FROM log_entries"+eventWhere+" GROUP BY event ORDER BY n DESC, event ASC", args...).Scan(&events).Error; err != nil {
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

// SortCounts orders counts by size, then value, in place.
func SortCounts(c []Count) {
	sort.Slice(c, func(i, j int) bool {
		if c[i].Count != c[j].Count {
			return c[i].Count > c[j].Count
		}
		return strings.Compare(c[i].Value, c[j].Value) < 0
	})
}
