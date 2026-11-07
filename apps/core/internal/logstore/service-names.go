package logstore

import (
	"sort"
	"time"
)

// RecentServiceNames lists, per app, the distinct OpenTelemetry service.name resource values among
// its newest entries since the given time. Only the newest scanLimit entries across all apps are
// looked at, so the cost stays flat however large the log store is. Entries without a resource, or
// whose resource has no service.name, are skipped.
func (s *Store) RecentServiceNames(since time.Time, scanLimit int) (map[string][]string, error) {
	if scanLimit <= 0 || scanLimit > 20000 {
		scanLimit = 5000
	}
	var rows []struct {
		AppID string
		Name  string
	}
	// A dotted key is stored either literally ("service.name") or nested ({"service": {"name": ...}}).
	err := s.db.Raw(`SELECT DISTINCT app_id, name FROM (
  SELECT app_id, CAST(COALESCE(json_extract(resource, '$."service.name"'), json_extract(resource, '$.service.name')) AS TEXT) AS name
  FROM (SELECT app_id, resource FROM log_entries WHERE ts >= ? AND resource <> '' ORDER BY ts DESC LIMIT ?)
) WHERE name IS NOT NULL AND name <> ''`, since.UnixNano(), scanLimit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, r := range rows {
		out[r.AppID] = append(out[r.AppID], r.Name)
	}
	for _, names := range out {
		sort.Strings(names)
	}
	return out, nil
}
