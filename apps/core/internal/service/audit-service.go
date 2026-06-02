package service

import (
	"encoding/json"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	StatusPageAuditActionPublished                   = "status_page_published"
	StatusPageAuditActionUnpublished                 = "status_page_unpublished"
	StatusPageAuditActionDeleted                     = "status_page_deleted"
	StatusPageAuditActionSectionDeleted              = "status_page_section_deleted"
	StatusPageAuditActionComponentDeleted            = "status_page_component_deleted"
	StatusPageAuditActionComponentMappingCreated     = "status_page_component_mapping_created"
	StatusPageAuditActionComponentMappingUpdated     = "status_page_component_mapping_updated"
	StatusPageAuditActionComponentMappingDeleted     = "status_page_component_mapping_deleted"
	StatusPageAuditActionPublicIncidentCreated       = "status_page_public_incident_created"
	StatusPageAuditActionPublicIncidentUpdated       = "status_page_public_incident_updated"
	StatusPageAuditActionPublicIncidentUpdateCreated = "status_page_public_incident_update_created"
	StatusPageAuditActionPublicIncidentResolved      = "status_page_public_incident_resolved"
	StatusPageAuditActionPublicIncidentDeleted       = "status_page_public_incident_deleted"
	StatusPageAuditActionSubscriberUnsubscribed      = "status_page_subscriber_unsubscribed"
	StatusPageAuditActionSubscriberAnonymized        = "status_page_subscriber_anonymized"
	StatusPageAuditActionSubscriberHardDeleted       = "status_page_subscriber_hard_deleted"
	StatusPageAuditActionSubscriberPendingPurged     = "status_page_subscriber_pending_purged"
	StatusPageAuditActionSubscriberDeliveriesPurged  = "status_page_subscriber_deliveries_purged"

	DataLifecycleAuditActionSettingsUpdated = "data_lifecycle_settings_updated"
	DataLifecycleAuditActionRollupRun       = "data_lifecycle_rollup_run"
	DataLifecycleAuditActionArchiveRun      = "data_lifecycle_archive_run"
)

type StatusPageAuditEventInput struct {
	Action             string
	StatusPageID       string
	AffectedObjectType string
	AffectedObjectID   string
	ActorType          string
	ActorID            string
	Metadata           map[string]interface{}
}

type AuditEventInput struct {
	Action             string
	StatusPageID       string
	AffectedObjectType string
	AffectedObjectID   string
	ActorType          string
	ActorID            string
	Metadata           map[string]interface{}
}

type AuditService struct {
	db     *gorm.DB
	logger *utils.Logger
}

func NewAuditService(database *gorm.DB, logger *utils.Logger) *AuditService {
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
	return s.RecordEvent(AuditEventInput{
		Action:             normalized.Action,
		StatusPageID:       normalized.StatusPageID,
		AffectedObjectType: normalized.AffectedObjectType,
		AffectedObjectID:   normalized.AffectedObjectID,
		ActorType:          normalized.ActorType,
		ActorID:            normalized.ActorID,
		Metadata:           normalized.Metadata,
	})
}

func (s *AuditService) RecordEvent(input AuditEventInput) (*db.AuditEvent, error) {
	normalized := normalizeAuditEventInput(input)
	if err := validateAuditEventInput(normalized); err != nil {
		return nil, err
	}
	metadata, err := statusPageAuditMetadataJSON(normalized.Metadata)
	if err != nil {
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
		MetadataJSON:       metadata,
		CreatedAt:          time.Now().UTC(),
	}
	if err := s.db.Create(&event).Error; err != nil {
		s.logger.Error("Failed to record audit event", "action", event.Action, "affected_object_type", event.AffectedObjectType, "affected_object_id", event.AffectedObjectID, "error", err)
		return nil, err
	}
	return &event, nil
}

func statusPageAuditMetadataJSON(metadata map[string]interface{}) (string, error) {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func normalizeAuditEventInput(input AuditEventInput) AuditEventInput {
	metadata := input.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	return AuditEventInput{
		Action:             strings.TrimSpace(input.Action),
		StatusPageID:       strings.TrimSpace(input.StatusPageID),
		AffectedObjectType: strings.TrimSpace(input.AffectedObjectType),
		AffectedObjectID:   strings.TrimSpace(input.AffectedObjectID),
		ActorType:          strings.TrimSpace(input.ActorType),
		ActorID:            strings.TrimSpace(input.ActorID),
		Metadata:           metadata,
	}
}

func validateAuditEventInput(input AuditEventInput) error {
	switch {
	case input.Action == "":
		return fmt.Errorf("audit action is required")
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

func normalizeStatusPageAuditEventInput(input StatusPageAuditEventInput) StatusPageAuditEventInput {
	return StatusPageAuditEventInput{
		Action:             strings.TrimSpace(input.Action),
		StatusPageID:       strings.TrimSpace(input.StatusPageID),
		AffectedObjectType: strings.TrimSpace(input.AffectedObjectType),
		AffectedObjectID:   strings.TrimSpace(input.AffectedObjectID),
		ActorType:          strings.TrimSpace(input.ActorType),
		ActorID:            strings.TrimSpace(input.ActorID),
		Metadata:           input.Metadata,
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
		StatusPageAuditActionDeleted,
		StatusPageAuditActionSectionDeleted,
		StatusPageAuditActionComponentDeleted,
		StatusPageAuditActionComponentMappingCreated,
		StatusPageAuditActionComponentMappingUpdated,
		StatusPageAuditActionComponentMappingDeleted,
		StatusPageAuditActionPublicIncidentCreated,
		StatusPageAuditActionPublicIncidentUpdated,
		StatusPageAuditActionPublicIncidentUpdateCreated,
		StatusPageAuditActionPublicIncidentResolved,
		StatusPageAuditActionPublicIncidentDeleted,
		StatusPageAuditActionSubscriberUnsubscribed,
		StatusPageAuditActionSubscriberAnonymized,
		StatusPageAuditActionSubscriberHardDeleted,
		StatusPageAuditActionSubscriberPendingPurged,
		StatusPageAuditActionSubscriberDeliveriesPurged,
	} {
		if action == supported {
			return true
		}
	}
	return false
}
