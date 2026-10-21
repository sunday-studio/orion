package state

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Before the rename, the registration lived in a table called agent_state and the spool and state rows
// carried an agent_id column. A node upgraded in place keeps its identity: the table and columns are
// renamed once, in place, before the normal schema step.
func migrateLegacyNames(database *gorm.DB) error {
	m := database.Migrator()
	if m.HasTable("agent_state") && !m.HasTable("node_state") {
		if err := m.RenameTable("agent_state", "node_state"); err != nil {
			return fmt.Errorf("rename agent_state: %w", err)
		}
	}
	for _, table := range []string{"node_state", "report_spool"} {
		if m.HasTable(table) && m.HasColumn(table, "agent_id") && !m.HasColumn(table, "node_id") {
			if err := m.RenameColumn(table, "agent_id", "node_id"); err != nil {
				return fmt.Errorf("rename %s.agent_id: %w", table, err)
			}
		}
	}
	return nil
}

// legacyAgentState reads a registration kept under the old names, for the read-only commands that run
// before the daemon has had a chance to migrate the database.
type legacyAgentState struct {
	ID                int        `gorm:"primaryKey"`
	AgentID           string     `gorm:"column:agent_id"`
	Token             string     `gorm:"column:token"`
	Registered        bool       `gorm:"column:registered"`
	CoreURL           string     `gorm:"column:core_url"`
	LastSync          time.Time  `gorm:"column:last_sync"`
	MaintenanceMode   bool       `gorm:"column:maintenance_mode"`
	MaintenanceReason *string    `gorm:"column:maintenance_reason"`
	MaintenanceUntil  *time.Time `gorm:"column:maintenance_until"`
}

func readLegacyAgentState(database *gorm.DB) (nodeStateRecord, bool) {
	m := database.Migrator()
	if m.HasTable("node_state") || !m.HasTable("agent_state") {
		return nodeStateRecord{}, false
	}
	var old legacyAgentState
	if err := database.Table("agent_state").Where("id = ?", 1).First(&old).Error; err != nil {
		return nodeStateRecord{}, false
	}
	return nodeStateRecord{ID: old.ID, NodeID: old.AgentID, Token: old.Token, Registered: old.Registered, CoreURL: old.CoreURL,
		LastSync: old.LastSync, MaintenanceMode: old.MaintenanceMode, MaintenanceReason: old.MaintenanceReason, MaintenanceUntil: old.MaintenanceUntil}, true
}
