package api

import (
	"fmt"
	"orion/core/internal/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var incidentSeverities = map[string]bool{"low": true, "medium": true, "high": true, "critical": true}

// incidentFiltersFromQuery reads the list/export filters. It is shared by GET /v1/incidents and
// the CSV export so both always agree.
func incidentFiltersFromQuery(c *gin.Context) (incidentListFilters, error) {
	filters := incidentListFilters{statuses: utils.QueryStatuses(c.DefaultQuery("status", "open,acknowledged,covered")), agentID: strings.TrimSpace(c.Query("agent_id")), monitorID: strings.TrimSpace(c.Query("monitor_id")), resolutionKind: strings.TrimSpace(c.Query("resolution_kind")), actor: strings.ToLower(strings.TrimSpace(c.Query("actor"))), covered: utils.QueryOptionalBool(c, "covered"), notificationStatus: strings.TrimSpace(c.Query("notification_status")), needsReview: utils.QueryBool(c, "needs_review", false)}
	for _, severity := range strings.Split(c.Query("severity"), ",") {
		severity = strings.ToLower(strings.TrimSpace(severity))
		if severity == "" {
			continue
		}
		if !incidentSeverities[severity] {
			return filters, fmt.Errorf("unsupported severity %q", severity)
		}
		filters.severities = append(filters.severities, severity)
	}
	filters.q = strings.TrimSpace(c.Query("q"))
	if len(filters.q) > 200 {
		return filters, fmt.Errorf("q is too long")
	}
	if raw := strings.TrimSpace(c.Query("opened_since")); raw != "" {
		since, err := parseSinceParam(raw, time.Now().UTC())
		if err != nil {
			return filters, fmt.Errorf("opened_since must be a duration like 24h or 7d, or an RFC3339 time")
		}
		filters.openedSince = &since
	}
	if raw := strings.TrimSpace(c.Query("opened_until")); raw != "" {
		until, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filters, fmt.Errorf("opened_until must be an RFC3339 time")
		}
		filters.openedUntil = &until
	}
	return filters, nil
}

func parseSinceParam(raw string, now time.Time) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if strings.HasSuffix(raw, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(raw, "d"))
		if err != nil || days <= 0 || days > 3650 {
			return time.Time{}, fmt.Errorf("bad days")
		}
		return now.AddDate(0, 0, -days), nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return time.Time{}, fmt.Errorf("bad duration")
	}
	return now.Add(-d), nil
}

var incidentLikeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// applyIncidentTextAndTimeFilters adds the severity, text and opened-time filters.
func (s *Server) applyIncidentTextAndTimeFilters(query *gorm.DB, filters incidentListFilters) *gorm.DB {
	if len(filters.severities) > 0 {
		query = query.Where("severity IN ?", filters.severities)
	}
	if filters.openedSince != nil {
		query = query.Where("opened_at >= ?", *filters.openedSince)
	}
	if filters.openedUntil != nil {
		query = query.Where("opened_at < ?", *filters.openedUntil)
	}
	if filters.q != "" {
		pattern := "%" + incidentLikeEscaper.Replace(strings.ToLower(filters.q)) + "%"
		number := -1 // matches nothing unless q looks like INC-0231
		digits := strings.TrimLeft(strings.ToUpper(filters.q), "INC-# ")
		if n, err := strconv.Atoi(digits); err == nil {
			number = n
		}
		query = query.Where(`LOWER(title) LIKE ? ESCAPE '\' OR number = ? OR monitor_id IN (SELECT id FROM monitors WHERE LOWER(name) LIKE ? ESCAPE '\') OR agent_id IN (SELECT id FROM agents WHERE LOWER(name) LIKE ? ESCAPE '\')`, pattern, number, pattern, pattern)
	}
	return query
}

// hiddenResolvedIncidentCount tells the list how many resolved incidents the status filter hides
// ("21 resolved incidents are hidden by this filter").
func (s *Server) hiddenResolvedIncidentCount(filters incidentListFilters) int64 {
	for _, status := range filters.statuses {
		if status == "resolved" {
			return 0
		}
	}
	other := filters
	other.statuses = []string{"resolved"}
	var count int64
	s.incidentListQuery(other).Count(&count)
	return count
}

// incidentDisplayID formats the human id, or "" for rows from before numbering.
func incidentDisplayID(number int) string {
	if number <= 0 {
		return ""
	}
	return fmt.Sprintf("INC-%04d", number)
}
