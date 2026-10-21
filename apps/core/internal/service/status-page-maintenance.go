package service

import (
	"time"

	"gorm.io/gorm"
)

// StatusPageMaintenanceHolds reports whether a status page maintenance window that holds alerts
// is active for a resource: a window's components map to the monitor or its node.
func StatusPageMaintenanceHolds(database *gorm.DB, monitorID string, nodeID string, now time.Time) bool {
	var n int64
	err := database.Raw(`SELECT COUNT(*) FROM status_page_maintenance w
		JOIN json_each(w.component_ids) j
		JOIN status_page_component_mappings m ON m.component_id = j.value
		WHERE w.hold_alerts = 1 AND w.starts_at <= ? AND w.ends_at > ?
		AND ((m.resource_type = 'monitor' AND m.resource_id = ?) OR (m.resource_type = 'node' AND m.resource_id = ?))`,
		now, now, monitorID, nodeID).Scan(&n).Error
	return err == nil && n > 0
}
