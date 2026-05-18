package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"orion/agent/internal/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	stateDirMode  = 0o700
	stateFileMode = 0o600
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

func (agentStateRecord) TableName() string {
	return "agent_state"
}

func (monitorStateRecord) TableName() string {
	return "monitor_state"
}

func DefaultPath() string {
	if _, err := os.Stat("/var/lib/orion"); err == nil {
		return "/var/lib/orion/state.db"
	}
	if _, err := os.Stat("/usr/local/var/lib/orion"); err == nil {
		return "/usr/local/var/lib/orion/state.db"
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "state.db")
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
	if err := database.AutoMigrate(&agentStateRecord{}, &monitorStateRecord{}); err != nil {
		return nil, fmt.Errorf("migrate state database: %w", err)
	}
	if err := os.Chmod(path, stateFileMode); err != nil {
		return nil, fmt.Errorf("secure state database permissions: %w", err)
	}

	return &Store{path: path, db: database}, nil
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

		return nil
	})
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
