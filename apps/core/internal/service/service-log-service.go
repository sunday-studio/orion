package service

import (
	"encoding/json"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxServiceLogBatchEntries = 200

type ServiceLogBatchPayload struct {
	Entries []ServiceLogEntryPayload `json:"entries" binding:"required"`
}

type ServiceLogEntryPayload struct {
	Timestamp   string         `json:"timestamp" binding:"required"`
	Source      string         `json:"source,omitempty"`
	Stream      string         `json:"stream,omitempty"`
	Level       string         `json:"level,omitempty"`
	Component   string         `json:"component,omitempty"`
	MonitorName string         `json:"monitor_name,omitempty"`
	MonitorID   string         `json:"monitor_id,omitempty"`
	Message     string         `json:"message,omitempty"`
	Fields      map[string]any `json:"fields,omitempty"`
	Raw         string         `json:"raw,omitempty"`
	Fingerprint string         `json:"fingerprint" binding:"required"`
}

type ServiceLogListFilters struct {
	AgentID   string
	MonitorID string
	Source    string
	Level     string
	Component string
	Search    string
	Limit     int
	Offset    int
}

type StoreServiceLogResult struct {
	Received int `json:"received"`
	Stored   int `json:"stored"`
}

type ServiceLogListResult struct {
	Entries []db.ServiceLogEntry
	Count   int64
}

type ServiceLogService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewServiceLogService(database *gorm.DB, logger *logging.Logger) *ServiceLogService {
	return &ServiceLogService{db: database, logger: logger}
}

func (s *ServiceLogService) StoreAgentLogBatch(agentID string, payload ServiceLogBatchPayload, collectedAt time.Time) (StoreServiceLogResult, error) {
	if agentID == "" {
		return StoreServiceLogResult{}, fmt.Errorf("agent id is required")
	}
	if len(payload.Entries) > maxServiceLogBatchEntries {
		return StoreServiceLogResult{}, fmt.Errorf("service log batch has %d entries; max is %d", len(payload.Entries), maxServiceLogBatchEntries)
	}

	entries := make([]db.ServiceLogEntry, 0, len(payload.Entries))
	for _, entryPayload := range payload.Entries {
		entry, ok, err := serviceLogEntryFromPayload(agentID, entryPayload, collectedAt)
		if err != nil {
			return StoreServiceLogResult{}, err
		}
		if ok {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return StoreServiceLogResult{Received: len(payload.Entries)}, nil
	}

	result := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "agent_id"}, {Name: "fingerprint"}},
		DoNothing: true,
	}).Create(&entries)
	if result.Error != nil {
		return StoreServiceLogResult{}, result.Error
	}

	return StoreServiceLogResult{
		Received: len(payload.Entries),
		Stored:   int(result.RowsAffected),
	}, nil
}

func (s *ServiceLogService) ListServiceLogs(filters ServiceLogListFilters) (ServiceLogListResult, error) {
	query := s.db.Model(&db.ServiceLogEntry{})
	query = applyServiceLogFilters(query, filters)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return ServiceLogListResult{}, err
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}

	var entries []db.ServiceLogEntry
	if err := query.Order("occurred_at DESC").
		Limit(limit).
		Offset(filters.Offset).
		Find(&entries).Error; err != nil {
		return ServiceLogListResult{}, err
	}

	return ServiceLogListResult{Entries: entries, Count: count}, nil
}

func applyServiceLogFilters(query *gorm.DB, filters ServiceLogListFilters) *gorm.DB {
	if filters.AgentID != "" {
		query = query.Where("agent_id = ?", filters.AgentID)
	}
	if filters.MonitorID != "" {
		query = query.Where("monitor_id = ?", filters.MonitorID)
	}
	if filters.Source != "" {
		query = query.Where("LOWER(source) = ?", strings.ToLower(filters.Source))
	}
	if filters.Level != "" {
		query = query.Where("UPPER(level) = ?", strings.ToUpper(filters.Level))
	}
	if filters.Component != "" {
		query = query.Where("component = ?", filters.Component)
	}
	if filters.Search != "" {
		like := "%" + strings.ToLower(filters.Search) + "%"
		query = query.Where(
			"LOWER(message) LIKE ? OR LOWER(component) LIKE ? OR LOWER(monitor_name) LIKE ? OR LOWER(agent_id) LIKE ? OR LOWER(monitor_id) LIKE ?",
			like,
			like,
			like,
			like,
			like,
		)
	}
	return query
}

func serviceLogEntryFromPayload(agentID string, payload ServiceLogEntryPayload, collectedAt time.Time) (db.ServiceLogEntry, bool, error) {
	fingerprint := strings.TrimSpace(payload.Fingerprint)
	if fingerprint == "" {
		return db.ServiceLogEntry{}, false, nil
	}

	occurredAt, err := time.Parse(time.RFC3339Nano, payload.Timestamp)
	if err != nil {
		if parsed, fallbackErr := time.Parse(time.RFC3339, payload.Timestamp); fallbackErr == nil {
			occurredAt = parsed
		} else {
			return db.ServiceLogEntry{}, false, fmt.Errorf("invalid service log timestamp %q: %w", payload.Timestamp, err)
		}
	}

	fieldsJSON := "{}"
	if len(payload.Fields) > 0 {
		data, err := json.Marshal(sanitizeServiceLogFields(payload.Fields))
		if err != nil {
			return db.ServiceLogEntry{}, false, fmt.Errorf("marshal service log fields: %w", err)
		}
		fieldsJSON = string(data)
	}

	return db.ServiceLogEntry{
		ID:          utils.GenerateID("service_log"),
		AgentID:     agentID,
		MonitorID:   strings.TrimSpace(payload.MonitorID),
		Source:      defaultString(strings.TrimSpace(payload.Source), "agent"),
		Stream:      defaultString(strings.TrimSpace(payload.Stream), "jsonl"),
		Level:       defaultString(strings.ToUpper(strings.TrimSpace(payload.Level)), "INFO"),
		Component:   strings.TrimSpace(payload.Component),
		MonitorName: strings.TrimSpace(payload.MonitorName),
		Message:     redactServiceLogText(strings.TrimSpace(payload.Message)),
		FieldsJSON:  fieldsJSON,
		Raw:         sanitizeServiceLogRaw(payload.Raw),
		Fingerprint: fingerprint,
		OccurredAt:  occurredAt.UTC(),
		CollectedAt: collectedAt.UTC(),
	}, true, nil
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func sanitizeServiceLogFields(fields map[string]any) map[string]any {
	sanitized := make(map[string]any, len(fields))
	for key, value := range fields {
		if isSensitiveLogField(key) {
			sanitized[key] = "[redacted]"
			continue
		}
		sanitized[key] = value
	}
	return sanitized
}

func sanitizeServiceLogRaw(raw string) string {
	// Do not persist raw service output until per-source redaction policies exist.
	return ""
}

func isSensitiveLogField(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "_", ""))
	for _, marker := range []string{"token", "secret", "password", "apikey", "authorization"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

var serviceLogSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(token|secret|password|api[_-]?key|authorization)(\s*[:=]\s*)([^\s,;]+)`),
	regexp.MustCompile(`(?i)(bearer\s+)([A-Za-z0-9._~+/=-]+)`),
}

func redactServiceLogText(value string) string {
	redacted := value
	for _, pattern := range serviceLogSecretPatterns {
		redacted = pattern.ReplaceAllString(redacted, `${1}${2}[redacted]`)
	}
	return redacted
}
