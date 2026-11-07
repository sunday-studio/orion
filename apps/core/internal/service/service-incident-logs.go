package service

import (
	"errors"
	"orion/core/internal/db"

	"gorm.io/gorm"
)

// Where an incident's app logs were found.
const (
	IncidentLogsViaService = "service" // incident.monitor_id -> monitor.service_id -> the service's apps
	IncidentLogsViaLogRule = "log_rule"
	IncidentLogsViaNone    = "none"
)

// IncidentLogSource says which apps' logs belong to an incident and how that was decided.
type IncidentLogSource struct {
	Via string
	// ServiceID and ServiceName are set whenever the incident's monitor is linked to a service,
	// even when that service has no app yet (the caller can then say so).
	ServiceID   string
	ServiceName string
	AppIDs      []string
}

// IncidentLogSource resolves the apps whose logs explain an incident on this monitor:
//
//  1. the apps of the monitor's service (monitor.service_id);
//  2. otherwise, or when the service has no app, the app of the log alert rule that owns the
//     monitor (the lookup that existed before services, kept so nothing regresses);
//  3. otherwise none.
func (c *ServiceCatalog) IncidentLogSource(monitorID string) (IncidentLogSource, error) {
	src := IncidentLogSource{Via: IncidentLogsViaNone}
	if monitorID == "" {
		return src, nil
	}

	var monitor db.Monitor
	err := c.db.Select("id", "service_id").Where("id = ?", monitorID).First(&monitor).Error
	switch {
	case err == nil && monitor.ServiceID != nil:
		var svc db.Service
		if err := c.db.Select("id", "name").Where("id = ?", *monitor.ServiceID).First(&svc).Error; err == nil {
			src.ServiceID, src.ServiceName = svc.ID, svc.Name
			if err := c.db.Model(&db.App{}).Where("service_id = ?", svc.ID).Order("name ASC").Pluck("id", &src.AppIDs).Error; err != nil {
				return src, err
			}
			if len(src.AppIDs) == 0 {
				src.AppIDs = nil
			} else {
				src.Via = IncidentLogsViaService
				return src, nil
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return src, err
		}
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		return src, err
	}

	var rule db.LogAlertRule
	if err := c.db.Select("app_id").Where("monitor_id = ?", monitorID).First(&rule).Error; err == nil && rule.AppID != "" {
		src.Via, src.AppIDs = IncidentLogsViaLogRule, []string{rule.AppID}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return src, err
	}
	return src, nil
}
