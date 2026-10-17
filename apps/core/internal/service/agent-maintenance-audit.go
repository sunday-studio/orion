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
	AgentMaintenanceAuditEnabled  = "agent.maintenance_enabled"
	AgentMaintenanceAuditDisabled = "agent.maintenance_disabled"
	AgentMaintenanceAuditExpired  = "agent.maintenance_expired"
)

func recordMaintenanceAuditEvent(tx *gorm.DB, action string, agentID string, actor AgentTokenActionInput, metadata map[string]any) error {
	actorType := strings.TrimSpace(actor.ActorType)
	if actorType == "" {
		actorType = "agent"
	}
	actorID := strings.TrimSpace(actor.ActorID)
	if actorID == "" {
		actorID = agentID
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
		AffectedObjectType: "agent",
		AffectedObjectID:   agentID,
		ActorType:          actorType,
		ActorID:            actorID,
		MetadataJSON:       string(encoded),
		CreatedAt:          time.Now().UTC(),
	}).Error
}
