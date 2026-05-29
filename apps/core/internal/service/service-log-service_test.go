package service

import (
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestStoreAgentLogBatchDedupesAndSanitizesFields(t *testing.T) {
	database := openServiceLogTestDatabase(t)
	service := NewServiceLogService(database, logging.NewLogger())

	payload := ServiceLogBatchPayload{Entries: []ServiceLogEntryPayload{
		{
			Timestamp:   "2026-05-27T20:00:00Z",
			Level:       "error",
			Component:   "registration",
			Message:     "registration failed token=secret-token",
			Fingerprint: "fp-1",
			Fields: map[string]any{
				"token": "secret-token",
				"retry": float64(1),
			},
		},
		{
			Timestamp:   "2026-05-27T20:00:00Z",
			Level:       "error",
			Component:   "registration",
			Message:     "registration failed",
			Fingerprint: "fp-1",
		},
	}}

	result, err := service.StoreAgentLogBatch("agent-1", payload, time.Date(2026, 5, 27, 20, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("StoreAgentLogBatch() error = %v", err)
	}
	if result.Received != 2 || result.Stored != 1 {
		t.Fatalf("result = %+v, want received 2 stored 1", result)
	}

	listed, err := service.ListServiceLogs(ServiceLogListFilters{AgentID: "agent-1", Level: "ERROR"})
	if err != nil {
		t.Fatalf("ListServiceLogs() error = %v", err)
	}
	if listed.Count != 1 || len(listed.Entries) != 1 {
		t.Fatalf("listed = %+v, want one entry", listed)
	}
	entry := listed.Entries[0]
	if entry.Level != "ERROR" || entry.Component != "registration" || entry.Message != "registration failed token=[redacted]" {
		t.Fatalf("entry = %+v, want normalized error registration log", entry)
	}
	if entry.FieldsJSON != `{"retry":1,"token":"[redacted]"}` {
		t.Fatalf("fields_json = %q, want redacted token", entry.FieldsJSON)
	}
}

func TestListServiceLogsFiltersSearch(t *testing.T) {
	database := openServiceLogTestDatabase(t)
	service := NewServiceLogService(database, logging.NewLogger())
	_, err := service.StoreAgentLogBatch("agent-1", ServiceLogBatchPayload{Entries: []ServiceLogEntryPayload{
		{
			Timestamp:   "2026-05-27T20:00:00Z",
			Level:       "INFO",
			Component:   "runtime",
			Message:     "agent started",
			Fingerprint: "fp-started",
		},
		{
			Timestamp:   "2026-05-27T20:01:00Z",
			Level:       "WARN",
			Component:   "monitor",
			MonitorName: "homepage",
			Message:     "monitor check slow",
			Fingerprint: "fp-monitor",
		},
	}}, time.Now())
	if err != nil {
		t.Fatalf("StoreAgentLogBatch() error = %v", err)
	}

	listed, err := service.ListServiceLogs(ServiceLogListFilters{Search: "homepage", Level: "WARN"})
	if err != nil {
		t.Fatalf("ListServiceLogs() error = %v", err)
	}
	if listed.Count != 1 || listed.Entries[0].Fingerprint != "fp-monitor" {
		t.Fatalf("listed = %+v, want homepage warning entry", listed)
	}
}

func openServiceLogTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return database
}
