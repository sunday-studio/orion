package db

import "time"

// InMaintenance reports whether the node is in an unexpired maintenance window.
// A flag without an end time (legacy data) counts as active until it is migrated.
func (a Node) InMaintenance(now time.Time) bool {
	if !a.MaintenanceMode {
		return false
	}
	return a.MaintenanceUntil == nil || now.Before(*a.MaintenanceUntil)
}

// InMaintenance reports whether now falls inside the monitor's maintenance window.
func (m Monitor) InMaintenance(now time.Time) bool {
	if m.MaintenanceEndsAt == nil || !now.Before(*m.MaintenanceEndsAt) {
		return false
	}
	return m.MaintenanceStartsAt == nil || !now.Before(*m.MaintenanceStartsAt)
}
