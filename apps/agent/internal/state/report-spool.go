package state

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	reportSpoolBaseBackoff = 30 * time.Second
	reportSpoolMaxBackoff  = 15 * time.Minute
)

const (
	ReportSpoolKindSystem  = "system_report"
	ReportSpoolKindMonitor = "monitor_report"
)

type reportSpoolRecord struct {
	ID            uint      `gorm:"primaryKey"`
	Kind          string    `gorm:"column:kind;not null;index:idx_report_spool_due"`
	AgentID       string    `gorm:"column:agent_id;not null"`
	MonitorID     string    `gorm:"column:monitor_id"`
	MonitorName   string    `gorm:"column:monitor_name"`
	PayloadJSON   string    `gorm:"column:payload_json;not null"`
	Attempts      int       `gorm:"column:attempts;not null;default:0"`
	LastError     string    `gorm:"column:last_error"`
	NextAttemptAt time.Time `gorm:"column:next_attempt_at;index:idx_report_spool_due"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

type SpooledReport struct {
	ID            uint
	Kind          string
	AgentID       string
	MonitorID     string
	MonitorName   string
	PayloadJSON   json.RawMessage
	Attempts      int
	LastError     string
	NextAttemptAt time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (reportSpoolRecord) TableName() string {
	return "report_spool"
}

func (s *Store) EnqueueReport(kind string, agentID string, monitorID string, monitorName string, payload any, lastErr error) (*SpooledReport, error) {
	if kind != ReportSpoolKindSystem && kind != ReportSpoolKindMonitor {
		return nil, fmt.Errorf("unsupported report spool kind: %s", kind)
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal report payload: %w", err)
	}

	record := reportSpoolRecord{
		Kind:          kind,
		AgentID:       agentID,
		MonitorID:     monitorID,
		MonitorName:   monitorName,
		PayloadJSON:   string(payloadBytes),
		LastError:     errorString(lastErr),
		NextAttemptAt: time.Now().UTC(),
	}
	if err := s.db.Create(&record).Error; err != nil {
		return nil, fmt.Errorf("enqueue report spool item: %w", err)
	}
	item := spooledReportFromRecord(record)
	return &item, nil
}

func (s *Store) ListDueReports(now time.Time, limit int) ([]SpooledReport, error) {
	if limit <= 0 {
		limit = 100
	}

	var records []reportSpoolRecord
	if err := s.db.
		Where("next_attempt_at <= ?", now.UTC()).
		Order("created_at ASC").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("load due report spool items: %w", err)
	}

	items := make([]SpooledReport, 0, len(records))
	for _, record := range records {
		items = append(items, spooledReportFromRecord(record))
	}
	return items, nil
}

func (s *Store) MarkReportSent(id uint) error {
	if err := s.db.Delete(&reportSpoolRecord{}, id).Error; err != nil {
		return fmt.Errorf("delete sent report spool item: %w", err)
	}
	return nil
}

func (s *Store) MarkReportFailed(id uint, lastErr error) error {
	var record reportSpoolRecord
	if err := s.db.Where("id = ?", id).First(&record).Error; err != nil {
		return fmt.Errorf("load failed report spool item: %w", err)
	}

	attempts := record.Attempts + 1
	if err := s.db.Model(&record).Updates(map[string]interface{}{
		"attempts":        attempts,
		"last_error":      errorString(lastErr),
		"next_attempt_at": time.Now().UTC().Add(reportSpoolBackoff(attempts)),
		"updated_at":      time.Now().UTC(),
	}).Error; err != nil {
		return fmt.Errorf("update failed report spool item: %w", err)
	}
	return nil
}

func (s *Store) CountSpooledReports() (int64, error) {
	var count int64
	if err := s.db.Model(&reportSpoolRecord{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count report spool items: %w", err)
	}
	return count, nil
}

func spooledReportFromRecord(record reportSpoolRecord) SpooledReport {
	return SpooledReport{
		ID:            record.ID,
		Kind:          record.Kind,
		AgentID:       record.AgentID,
		MonitorID:     record.MonitorID,
		MonitorName:   record.MonitorName,
		PayloadJSON:   json.RawMessage(record.PayloadJSON),
		Attempts:      record.Attempts,
		LastError:     record.LastError,
		NextAttemptAt: record.NextAttemptAt,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}
}

func reportSpoolBackoff(attempts int) time.Duration {
	if attempts <= 0 {
		return reportSpoolBaseBackoff
	}
	backoff := reportSpoolBaseBackoff
	for i := 1; i < attempts; i++ {
		backoff *= 2
		if backoff >= reportSpoolMaxBackoff {
			return reportSpoolMaxBackoff
		}
	}
	return backoff
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
