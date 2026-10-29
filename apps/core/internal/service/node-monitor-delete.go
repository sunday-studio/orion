package service

import (
	"errors"
	"orion/core/internal/db"
	"time"

	"gorm.io/gorm"
)

// ErrNodeMonitorNotRemovable means the monitor runs through a Node but is not backed by a definition
// created in Orion (for example one declared in the Node's own config), so Core cannot remove it.
var ErrNodeMonitorNotRemovable = errors.New("monitor is defined on the node, not in Orion")

// DeleteMonitor deletes a Core-managed monitor, or a Node monitor that was added in Orion (a
// definition, including accepted discoveries). The Node stops running a definition once it is gone.
// keepHistory only applies to Core-managed monitors; a Node monitor is always kept soft-deleted.
func (s *CoreMonitorManagementService) DeleteMonitor(monitorID string, keepHistory bool) error {
	err := s.DeleteCoreMonitorWith(monitorID, keepHistory)
	if !errors.Is(err, ErrCoreManagedMonitorNotFound) {
		return err
	}
	now := time.Now().UTC()
	return s.db.Transaction(func(tx *gorm.DB) error {
		var monitor db.Monitor
		if err := tx.Where("id = ? AND lifecycle <> ?", monitorID, "deleted").First(&monitor).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMonitorNotFound
			}
			return err
		}
		res := tx.Where("node_id = ? AND name = ?", monitor.NodeID, monitor.Name).Delete(&db.NodeMonitorDefinition{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNodeMonitorNotRemovable
		}
		if err := resolveIncidentsForRemovedMonitor(tx, s.logger, monitorID); err != nil {
			return err
		}
		// keepHistory is ignored: the row stays (soft-deleted, purged by retention) because the Node
		// still holds the monitor until its next sync and then calls unregister-monitor, which fails
		// with not found if the row was removed, and would keep failing on every later reconcile.
		return tx.Model(&db.Monitor{}).Where("id = ?", monitorID).Updates(map[string]any{
			"lifecycle":  "deleted",
			"health":     "unknown",
			"updated_at": now,
			"deleted_at": now,
		}).Error
	})
}
