package service

import (
	"orion/core/internal/db"
	"strings"
)

func (s *MonitorService) ListAllMonitors(opts ListAllMonitorsOpts) ([]db.Monitor, int64, error) {
	monitors, count, _, err := s.ListAllMonitorsPage(opts)
	return monitors, count, err
}

// ListAllMonitorsPage is ListAllMonitors that also returns total: how many monitors match every
// filter except Statuses, so a screen can say "20 healthy monitors are hidden".
func (s *MonitorService) ListAllMonitorsPage(opts ListAllMonitorsOpts) ([]db.Monitor, int64, int64, error) {
	var monitors []db.Monitor
	query := s.applyAllMonitorListFilters(s.db.Model(&db.Monitor{}), opts)

	sortColumn := "updated_at"
	switch strings.ToLower(strings.TrimSpace(opts.Sort)) {
	case "name", "type", "health", "lifecycle", "created_at", "updated_at", "last_successful_report_at":
		sortColumn = strings.ToLower(strings.TrimSpace(opts.Sort))
	}

	order := "desc"
	if strings.EqualFold(opts.Order, "asc") {
		order = "asc"
	}

	query = query.Order(sortColumn + " " + order)

	if err := query.Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to list all monitors", "error", err)
		return nil, 0, 0, err
	}

	monitors, err := s.monitorsWithComputedHealth(monitors)
	if err != nil {
		return nil, 0, 0, err
	}
	monitors = filterMonitorsByComputedHealth(monitors, opts.Health)
	if opts.StaleOnly {
		monitors = filterMonitorsByComputedHealth(monitors, "stale")
	}

	total := int64(len(monitors))
	if len(opts.Statuses) > 0 {
		kept := monitors[:0:0]
		for _, m := range monitors {
			health := m.ComputedHealth
			if health == "" {
				health = m.Health
			}
			if health == "" {
				health = "unknown"
			}
			if opts.Statuses[health] {
				kept = append(kept, m)
			}
		}
		monitors = kept
	}
	count := int64(len(monitors))
	return paginateMonitors(monitors, opts.Limit, opts.Offset), count, total, nil
}
