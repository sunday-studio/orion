package service

import (
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	NodeMaintenanceAuditEnabled  = "node.maintenance_enabled"
	NodeMaintenanceAuditDisabled = "node.maintenance_disabled"
	NodeMaintenanceAuditExpired  = "node.maintenance_expired"
)

func recordMaintenanceAuditEvent(tx *gorm.DB, action string, nodeID string, actor NodeTokenActionInput, metadata map[string]any) error {
	actorType := strings.TrimSpace(actor.ActorType)
	if actorType == "" {
		actorType = "node"
	}
	actorID := strings.TrimSpace(actor.ActorID)
	if actorID == "" {
		actorID = nodeID
	}
	if actor.RequestID != "" {
		metadata["request_id"] = actor.RequestID
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return tx.Create(&db.AuditEvent{
		ID:                 utils.GenerateID("audit_event"),
		Action:             action,
		AffectedObjectType: "node",
		AffectedObjectID:   nodeID,
		ActorType:          actorType,
		ActorID:            actorID,
		MetadataJSON:       string(encoded),
		CreatedAt:          time.Now().UTC(),
	}).Error
}
