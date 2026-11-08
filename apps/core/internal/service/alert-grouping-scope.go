package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"strings"
	"time"

	"gorm.io/gorm"
)

// serviceGroupJoinWindow bounds how long after its last event a service group still takes new
// incidents once none of its members is active any more. While a member is active the group always
// takes them (the incidents overlap).
const serviceGroupJoinWindow = 30 * time.Minute

// alertGroupScope says how an incident is grouped. Service is set when the incident belongs to a
// Service; such incidents group by service whatever the monitor type, so a failed check and a log
// burst of one service page once.
type alertGroupScope struct {
	Key         string
	ServiceID   string
	ServiceName string
}

func (g alertGroupScope) isService() bool { return g.ServiceID != "" }

func (g alertGroupScope) summary(event AlertRouteContext, count int) string {
	if g.isService() {
		return fmt.Sprintf("%d incident(s) on service %s", count, g.ServiceName)
	}
	return alertGroupSummary(event, count)
}

// alertGroupScopeFor picks the group an incident belongs to: its service, else the per-node default.
func (s *AlertService) alertGroupScopeFor(event AlertRouteContext, incident db.Incident) (alertGroupScope, error) {
	id, name, err := incidentServiceOf(s.db, incident.MonitorID)
	if err != nil {
		return alertGroupScope{}, err
	}
	if id == "" {
		return alertGroupScope{Key: alertGroupKey(event)}, nil
	}
	return alertGroupScope{Key: "service:" + id, ServiceID: id, ServiceName: name}, nil
}

// incidentServiceOf finds the service of an incident's monitor: the monitor's own link, or, for the
// monitor a log alert rule owns, the link of the rule's app. A link to a service that no longer
// exists counts as none.
func incidentServiceOf(tx *gorm.DB, monitorID string) (id, name string, err error) {
	if strings.TrimSpace(monitorID) == "" {
		return "", "", nil
	}
	var monitor db.Monitor
	if err := tx.Select("id", "service_id").Where("id = ?", monitorID).Limit(1).Find(&monitor).Error; err != nil {
		return "", "", err
	}
	serviceID := derefString(monitor.ServiceID)
	if serviceID == "" {
		var app db.App
		err := tx.Table("apps").Select("apps.service_id").
			Joins("JOIN log_alert_rules ON log_alert_rules.app_id = apps.id").
			Where("log_alert_rules.monitor_id = ?", monitorID).Limit(1).Find(&app).Error
		if err != nil {
			return "", "", err
		}
		serviceID = derefString(app.ServiceID)
	}
	if serviceID == "" {
		return "", "", nil
	}
	var svc db.Service
	if err := tx.Select("id", "name").Where("id = ?", serviceID).First(&svc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", nil
		}
		return "", "", err
	}
	return svc.ID, svc.Name, nil
}

// openAlertGroup returns the open group an incident may join, if any. A node group always takes it;
// a service group only while one of its incidents is active or it saw an event recently.
func (s *AlertService) openAlertGroup(scope alertGroupScope, now time.Time) (db.AlertGroup, bool, error) {
	var groups []db.AlertGroup
	if err := s.db.Where("group_key = ? AND status = ?", scope.Key, "open").Order("last_event_at DESC").Limit(5).Find(&groups).Error; err != nil {
		return db.AlertGroup{}, false, err
	}
	for _, g := range groups {
		if !scope.isService() || now.Sub(g.LastEventAt) <= serviceGroupJoinWindow {
			return g, true, nil
		}
		active, err := s.activeAlertGroupIncidentCount(g.ID)
		if err != nil {
			return db.AlertGroup{}, false, err
		}
		if active > 0 {
			return g, true, nil
		}
	}
	return db.AlertGroup{}, false, nil
}

// incidentSeverityRank orders severities from worst (0) to least bad; unknown values rank last.
func incidentSeverityRank(severity string) int {
	if rank, ok := severityRank[strings.ToLower(strings.TrimSpace(severity))]; ok {
		return rank
	}
	return len(severityRank)
}

// groupSummary words an existing group's summary from the group itself, not from the incident's
// current links: a group of service A keeps naming service A even if the incident was moved since.
func (s *AlertService) groupSummary(group db.AlertGroup, event AlertRouteContext, count int) (string, error) {
	serviceID, ok := strings.CutPrefix(group.GroupKey, "service:")
	if !ok {
		return alertGroupSummary(event, count), nil
	}
	var svc db.Service
	if err := s.db.Select("name").Where("id = ?", serviceID).Limit(1).Find(&svc).Error; err != nil {
		return "", err
	}
	name := svc.Name
	if name == "" {
		name = serviceID // the service was deleted since: the id still says which group this is
	}
	return alertGroupScope{ServiceID: serviceID, ServiceName: name}.summary(event, count), nil
}
