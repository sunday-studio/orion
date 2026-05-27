package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/logging"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	stateDirMode           = 0o700
	stateFileMode          = 0o600
	reportSpoolBaseBackoff = 30 * time.Second
	reportSpoolMaxBackoff  = 15 * time.Minute
)

const (
	ReportSpoolKindSystem  = "system_report"
	ReportSpoolKindMonitor = "monitor_report"
)

type Store struct {
	path string
	db   *gorm.DB
}

type agentStateRecord struct {
	ID                int       `gorm:"primaryKey"`
	AgentID           string    `gorm:"column:agent_id"`
	Token             string    `gorm:"column:token"`
	Registered        bool      `gorm:"column:registered;not null;default:false"`
	CoreURL           string    `gorm:"column:core_url"`
	LastSync          time.Time `gorm:"column:last_sync"`
	MaintenanceMode   bool      `gorm:"column:maintenance_mode;not null;default:false"`
	MaintenanceReason *string   `gorm:"column:maintenance_reason"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

type monitorStateRecord struct {
	Name        string    `gorm:"primaryKey;column:name"`
	MonitorID   string    `gorm:"column:monitor_id;not null"`
	Status      string    `gorm:"column:status;not null"`
	LastChecked time.Time `gorm:"column:last_checked"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

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

func (agentStateRecord) TableName() string {
	return "agent_state"
}

func (monitorStateRecord) TableName() string {
	return "monitor_state"
}

func (reportSpoolRecord) TableName() string {
	return "report_spool"
}

func DefaultPath() string {
	switch runtime.GOOS {
	case "linux":
		return "/var/lib/orion/state.db"
	case "darwin":
		return "/usr/local/var/lib/orion/state.db"
	default:
		return "state.db"
	}
}

func Open(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := ensureStateDir(filepath.Dir(path)); err != nil {
		return nil, err
	}

	database, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open state database: %w", err)
	}
	if err := database.AutoMigrate(&agentStateRecord{}, &monitorStateRecord{}, &reportSpoolRecord{}); err != nil {
		return nil, fmt.Errorf("migrate state database: %w", err)
	}
	if err := os.Chmod(path, stateFileMode); err != nil {
		return nil, fmt.Errorf("secure state database permissions: %w", err)
	}

	return &Store{path: path, db: database}, nil
}

// InspectReadOnly loads the current agent state without creating, migrating, or
// chmodding the database. It is intended for read-only operator commands.
func InspectReadOnly(path string) (*config.InternalState, error) {
	if path == "" {
		path = DefaultPath()
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("inspect state database: %w", err)
	}

	database, err := gorm.Open(sqlite.Open(readOnlySQLiteDSN(path)), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open state database read-only: %w", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("open state database handle: %w", err)
	}
	defer sqlDB.Close()

	record := agentStateRecord{ID: 1}
	if err := database.First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			record = agentStateRecord{ID: 1}
		} else {
			return nil, fmt.Errorf("load agent state: %w", err)
		}
	}

	var monitorRecords []monitorStateRecord
	if err := database.Order("name ASC").Find(&monitorRecords).Error; err != nil {
		return nil, fmt.Errorf("load monitor state: %w", err)
	}

	monitors := make([]config.InternalStateMonitor, 0, len(monitorRecords))
	for _, monitor := range monitorRecords {
		monitors = append(monitors, config.InternalStateMonitor{
			Name:        monitor.Name,
			ID:          monitor.MonitorID,
			Status:      monitor.Status,
			LastChecked: monitor.LastChecked,
		})
	}

	return &config.InternalState{
		AgentID:           record.AgentID,
		Token:             record.Token,
		Registered:        record.Registered,
		CoreURL:           record.CoreURL,
		LastSync:          record.LastSync,
		MaintenanceMode:   record.MaintenanceMode,
		MaintenanceReason: record.MaintenanceReason,
		Monitors:          monitors,
	}, nil
}

func readOnlySQLiteDSN(path string) string {
	return "file:" + strings.ReplaceAll(path, "?", "%3f") + "?mode=ro"
}

func ensureStateDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	if _, err := os.Stat(dir); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect state directory: %w", err)
	}
	if err := os.MkdirAll(dir, stateDirMode); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	if err := os.Chmod(dir, stateDirMode); err != nil {
		return fmt.Errorf("secure state directory permissions: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Get() (*config.InternalState, error) {
	record, err := s.getOrCreateAgentState()
	if err != nil {
		return nil, err
	}

	var monitorRecords []monitorStateRecord
	if err := s.db.Order("name ASC").Find(&monitorRecords).Error; err != nil {
		return nil, fmt.Errorf("load monitor state: %w", err)
	}

	monitors := make([]config.InternalStateMonitor, 0, len(monitorRecords))
	for _, monitor := range monitorRecords {
		monitors = append(monitors, config.InternalStateMonitor{
			Name:        monitor.Name,
			ID:          monitor.MonitorID,
			Status:      monitor.Status,
			LastChecked: monitor.LastChecked,
		})
	}

	return &config.InternalState{
		AgentID:           record.AgentID,
		Token:             record.Token,
		Registered:        record.Registered,
		CoreURL:           record.CoreURL,
		LastSync:          record.LastSync,
		MaintenanceMode:   record.MaintenanceMode,
		MaintenanceReason: record.MaintenanceReason,
		Monitors:          monitors,
	}, nil
}

func (s *Store) GetMonitorByName(name string) (*config.InternalStateMonitor, error) {
	var record monitorStateRecord
	if err := s.db.Where("name = ?", name).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load monitor state: %w", err)
	}
	return &config.InternalStateMonitor{
		Name:        record.Name,
		ID:          record.MonitorID,
		Status:      record.Status,
		LastChecked: record.LastChecked,
	}, nil
}

func (s *Store) UpdateRegistration(agentID string, token string, coreURL string) error {
	record, err := s.getOrCreateAgentState()
	if err != nil {
		return err
	}
	return s.db.Model(record).Updates(map[string]interface{}{
		"agent_id":   agentID,
		"token":      token,
		"registered": true,
		"core_url":   coreURL,
		"last_sync":  time.Now(),
		"updated_at": time.Now(),
	}).Error
}

func (s *Store) ReplaceMonitors(monitors []config.InternalStateMonitor) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&monitorStateRecord{}).Error; err != nil {
			return err
		}
		for _, monitor := range monitors {
			record := monitorStateRecord{
				Name:        monitor.Name,
				MonitorID:   monitor.ID,
				Status:      monitor.Status,
				LastChecked: monitor.LastChecked,
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ResetRegistration() error {
	logging.Debugf("resetting registration state in %s", s.path)
	return s.db.Transaction(func(tx *gorm.DB) error {
		var record agentStateRecord
		result := tx.Where("id = ?", 1).Find(&record)
		if result.Error != nil {
			return fmt.Errorf("load agent state: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			record = agentStateRecord{ID: 1}
			if err := tx.Create(&record).Error; err != nil {
				return fmt.Errorf("create agent state: %w", err)
			}
		}

		if err := tx.Model(&record).Updates(map[string]interface{}{
			"agent_id":   "",
			"token":      "",
			"registered": false,
			"core_url":   "",
			"last_sync":  time.Time{},
			"updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}

		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&monitorStateRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&reportSpoolRecord{}).Error; err != nil {
			return err
		}
		logging.Debugf("registration state reset: monitor mappings and report spool cleared")

		return nil
	})
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

func (s *Store) SetMaintenanceMode(enabled bool, reason *string) error {
	record, err := s.getOrCreateAgentState()
	if err != nil {
		return err
	}
	return s.db.Model(record).Updates(map[string]interface{}{
		"maintenance_mode":   enabled,
		"maintenance_reason": reason,
		"updated_at":         time.Now(),
	}).Error
}

func (s *Store) getOrCreateAgentState() (*agentStateRecord, error) {
	var record agentStateRecord
	result := s.db.Where("id = ?", 1).Find(&record)
	if result.Error != nil {
		return nil, fmt.Errorf("load agent state: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		return &record, nil
	}

	record = agentStateRecord{
		ID: 1,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return nil, fmt.Errorf("create agent state: %w", err)
	}
	return &record, nil
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
