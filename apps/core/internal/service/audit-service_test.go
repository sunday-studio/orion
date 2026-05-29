package service

import (
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRecordStatusPageEventStoresMinimalAuditFields(t *testing.T) {
	database := openAuditServiceTestDatabase(t)
	service := NewAuditService(database, logging.NewLogger())

	event, err := service.RecordStatusPageEvent(StatusPageAuditEventInput{
		Action:             " " + StatusPageAuditActionPublished + " ",
		StatusPageID:       " status-page-1 ",
		AffectedObjectType: " status_page ",
		AffectedObjectID:   " status-page-1 ",
		ActorType:          " user ",
		ActorID:            " user-1 ",
	})
	if err != nil {
		t.Fatalf("RecordStatusPageEvent() error = %v", err)
	}

	if event.ID == "" {
		t.Fatal("event ID was empty")
	}
	if event.Action != StatusPageAuditActionPublished {
		t.Fatalf("Action = %q, want %q", event.Action, StatusPageAuditActionPublished)
	}
	if event.StatusPageID != "status-page-1" {
		t.Fatalf("StatusPageID = %q, want status-page-1", event.StatusPageID)
	}
	if event.AffectedObjectType != "status_page" {
		t.Fatalf("AffectedObjectType = %q, want status_page", event.AffectedObjectType)
	}
	if event.AffectedObjectID != "status-page-1" {
		t.Fatalf("AffectedObjectID = %q, want status-page-1", event.AffectedObjectID)
	}
	if event.ActorType != "user" {
		t.Fatalf("ActorType = %q, want user", event.ActorType)
	}
	if event.ActorID != "user-1" {
		t.Fatalf("ActorID = %q, want user-1", event.ActorID)
	}
	if event.CreatedAt.IsZero() {
		t.Fatal("CreatedAt was zero")
	}

	var stored db.AuditEvent
	if err := database.First(&stored, "id = ?", event.ID).Error; err != nil {
		t.Fatalf("load stored audit event: %v", err)
	}
	if stored.Action != event.Action || stored.StatusPageID != event.StatusPageID || stored.ActorID != event.ActorID {
		t.Fatalf("stored audit event = %+v, want %+v", stored, *event)
	}
}

func TestRecordStatusPageEventRejectsUnsupportedAction(t *testing.T) {
	database := openAuditServiceTestDatabase(t)
	service := NewAuditService(database, logging.NewLogger())

	_, err := service.RecordStatusPageEvent(StatusPageAuditEventInput{
		Action:             "incident_payload_recorded",
		StatusPageID:       "status-page-1",
		AffectedObjectType: "public_incident",
		AffectedObjectID:   "public-incident-1",
		ActorType:          "user",
		ActorID:            "user-1",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported status page audit action") {
		t.Fatalf("RecordStatusPageEvent() error = %v, want unsupported action error", err)
	}
}

func TestRecordStatusPageEventRequiresActor(t *testing.T) {
	database := openAuditServiceTestDatabase(t)
	service := NewAuditService(database, logging.NewLogger())

	_, err := service.RecordStatusPageEvent(StatusPageAuditEventInput{
		Action:             StatusPageAuditActionUnpublished,
		StatusPageID:       "status-page-1",
		AffectedObjectType: "status_page",
		AffectedObjectID:   "status-page-1",
		ActorType:          "user",
	})
	if err == nil || !strings.Contains(err.Error(), "audit actor id is required") {
		t.Fatalf("RecordStatusPageEvent() error = %v, want actor id error", err)
	}
}

func TestRecordDataLifecycleEventStoresMetadata(t *testing.T) {
	database := openAuditServiceTestDatabase(t)
	service := NewAuditService(database, logging.NewLogger())

	event, err := service.RecordDataLifecycleEvent(DataLifecycleAuditEventInput{
		Action:           " " + DataLifecycleAuditActionSettingsUpdated + " ",
		AffectedObjectID: " settings ",
		ActorType:        " user ",
		ActorID:          " admin ",
		Metadata: map[string]interface{}{
			"changed_fields": []string{"raw_report_hot_days"},
		},
	})
	if err != nil {
		t.Fatalf("RecordDataLifecycleEvent() error = %v", err)
	}

	if event.Action != DataLifecycleAuditActionSettingsUpdated {
		t.Fatalf("Action = %q, want %q", event.Action, DataLifecycleAuditActionSettingsUpdated)
	}
	if event.AffectedObjectType != "data_lifecycle" || event.AffectedObjectID != "settings" {
		t.Fatalf("affected object = %s/%s, want data_lifecycle/settings", event.AffectedObjectType, event.AffectedObjectID)
	}
	if event.ActorType != "user" || event.ActorID != "admin" {
		t.Fatalf("actor = %s/%s, want user/admin", event.ActorType, event.ActorID)
	}
	if !strings.Contains(event.MetadataJSON, "raw_report_hot_days") {
		t.Fatalf("MetadataJSON = %q, want changed field", event.MetadataJSON)
	}
}

func TestRecordDataLifecycleEventRejectsUnsupportedAction(t *testing.T) {
	database := openAuditServiceTestDatabase(t)
	service := NewAuditService(database, logging.NewLogger())

	_, err := service.RecordDataLifecycleEvent(DataLifecycleAuditEventInput{
		Action:    "data_lifecycle_unknown",
		ActorType: "user",
		ActorID:   "admin",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported data lifecycle audit action") {
		t.Fatalf("RecordDataLifecycleEvent() error = %v, want unsupported action error", err)
	}
}

func openAuditServiceTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&db.AuditEvent{}); err != nil {
		t.Fatalf("migrate audit event: %v", err)
	}
	return database
}
