package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"strings"
	"time"
)

var ErrMonitorNotFound = errors.New("monitor not found")

const maxMaintenanceWindow = 30 * 24 * time.Hour

// MaintenanceWindow is a requested window; a nil StartsAt means now.
type MaintenanceWindow struct {
	StartsAt *time.Time
	EndsAt   time.Time
	Reason   string
}

// SetMonitorMaintenance holds incidents and alerts for the monitor between the window's times.
func (s *MonitorService) SetMonitorMaintenance(monitorID string, w MaintenanceWindow) error {
	now := time.Now().UTC()
	start := now
	if w.StartsAt != nil {
		start = w.StartsAt.UTC()
	}
	end := w.EndsAt.UTC()
	reason := strings.TrimSpace(w.Reason)
	switch {
	case !end.After(start):
		return fmt.Errorf("%w: the window must end after it starts", ErrIncidentInvalid)
	case end.Sub(start) > maxMaintenanceWindow:
		return fmt.Errorf("%w: a maintenance window can last at most 30 days", ErrIncidentInvalid)
	case !end.After(now):
		return fmt.Errorf("%w: the window must end in the future", ErrIncidentInvalid)
	case len(reason) > 200:
		return fmt.Errorf("%w: reason is too long", ErrIncidentInvalid)
	}
	res := s.db.Model(&db.Monitor{}).Where("id = ? AND lifecycle <> ?", monitorID, "deleted").
		Updates(map[string]any{"maintenance_starts_at": start, "maintenance_ends_at": end, "maintenance_reason": reason, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrMonitorNotFound
	}
	return nil
}

// ClearMonitorMaintenance ends the window now.
func (s *MonitorService) ClearMonitorMaintenance(monitorID string) error {
	res := s.db.Model(&db.Monitor{}).Where("id = ? AND lifecycle <> ?", monitorID, "deleted").
		Updates(map[string]any{"maintenance_starts_at": nil, "maintenance_ends_at": nil, "maintenance_reason": "", "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrMonitorNotFound
	}
	return nil
}
