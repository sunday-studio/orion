package service

import (
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	StatusPageAuditActionPublished                   = "status_page_published"
	StatusPageAuditActionUnpublished                 = "status_page_unpublished"
	StatusPageAuditActionComponentMappingCreated     = "status_page_component_mapping_created"
	StatusPageAuditActionComponentMappingUpdated     = "status_page_component_mapping_updated"
	StatusPageAuditActionPublicIncidentCreated       = "status_page_public_incident_created"
	StatusPageAuditActionPublicIncidentUpdated       = "status_page_public_incident_updated"
	StatusPageAuditActionPublicIncidentUpdateCreated = "status_page_public_incident_update_created"
	StatusPageAuditActionPublicIncidentResolved      = "status_page_public_incident_resolved"
)

type StatusPageAuditEventInput struct {
	Action             string
	StatusPageID       string
	AffectedObjectType string
	AffectedObjectID   string
	ActorType          string
	ActorID            string
}

type AuditService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewAuditService(database *gorm.DB, logger *logging.Logger) *AuditService {
	return &AuditService{
		db:     database,
		logger: logger,
	}
}

func (s *AuditService) RecordStatusPageEvent(input StatusPageAuditEventInput) (*db.AuditEvent, error) {
	normalized := normalizeStatusPageAuditEventInput(input)
	if err := validateStatusPageAuditEventInput(normalized); err != nil {
		return nil, err
	}

	event := db.AuditEvent{
		ID:                 utils.GenerateID("audit_event"),
		Action:             normalized.Action,
		StatusPageID:       normalized.StatusPageID,
		AffectedObjectType: normalized.AffectedObjectType,
		AffectedObjectID:   normalized.AffectedObjectID,
		ActorType:          normalized.ActorType,
		ActorID:            normalized.ActorID,
		CreatedAt:          time.Now().UTC(),
	}
	if err := s.db.Create(&event).Error; err != nil {
		s.logger.Error("Failed to record status page audit event", "action", event.Action, "status_page_id", event.StatusPageID, "error", err)
		return nil, err
	}
	return &event, nil
}

func normalizeStatusPageAuditEventInput(input StatusPageAuditEventInput) StatusPageAuditEventInput {
	return StatusPageAuditEventInput{
		Action:             strings.TrimSpace(input.Action),
		StatusPageID:       strings.TrimSpace(input.StatusPageID),
		AffectedObjectType: strings.TrimSpace(input.AffectedObjectType),
		AffectedObjectID:   strings.TrimSpace(input.AffectedObjectID),
		ActorType:          strings.TrimSpace(input.ActorType),
		ActorID:            strings.TrimSpace(input.ActorID),
	}
}

func validateStatusPageAuditEventInput(input StatusPageAuditEventInput) error {
	switch {
	case input.Action == "":
		return fmt.Errorf("audit action is required")
	case !validStatusPageAuditAction(input.Action):
		return fmt.Errorf("unsupported status page audit action")
	case input.StatusPageID == "":
		return fmt.Errorf("audit status page id is required")
	case input.AffectedObjectType == "":
		return fmt.Errorf("audit affected object type is required")
	case input.AffectedObjectID == "":
		return fmt.Errorf("audit affected object id is required")
	case input.ActorType == "":
		return fmt.Errorf("audit actor type is required")
	case input.ActorID == "":
		return fmt.Errorf("audit actor id is required")
	default:
		return nil
	}
}

func validStatusPageAuditAction(action string) bool {
	for _, supported := range []string{
		StatusPageAuditActionPublished,
		StatusPageAuditActionUnpublished,
		StatusPageAuditActionComponentMappingCreated,
		StatusPageAuditActionComponentMappingUpdated,
		StatusPageAuditActionPublicIncidentCreated,
		StatusPageAuditActionPublicIncidentUpdated,
		StatusPageAuditActionPublicIncidentUpdateCreated,
		StatusPageAuditActionPublicIncidentResolved,
	} {
		if action == supported {
			return true
		}
	}
	return false
}
