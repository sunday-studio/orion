package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	defaultCoreMonitorClaimLimit    = 10
	maxCoreMonitorClaimLimit        = 100
	defaultCoreMonitorLeaseDuration = 2 * time.Minute
	defaultCoreMonitorInterval      = 60 * time.Second
)

var ErrCoreMonitorLeaseNotHeld = errors.New("core monitor lease is not held by this owner")

type ClaimDueCoreMonitorConfigsRequest struct {
	LeaseOwner    string
	Limit         int
	LeaseDuration time.Duration
	Now           time.Time
}

type CompleteCoreMonitorCheckRequest struct {
	MonitorID  string
	LeaseOwner string
	FinishedAt time.Time
	Success    bool
	NextRunAt  *time.Time
}

type CoreMonitorSchedulerService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewCoreMonitorSchedulerService(database *gorm.DB, logger *logging.Logger) *CoreMonitorSchedulerService {
	return &CoreMonitorSchedulerService{
		db:     database,
		logger: logger,
	}
}

func (s *CoreMonitorSchedulerService) ClaimDueCoreMonitorConfigs(req ClaimDueCoreMonitorConfigsRequest) ([]db.CoreMonitorConfig, error) {
	leaseOwner := strings.TrimSpace(req.LeaseOwner)
	if leaseOwner == "" {
		return nil, fmt.Errorf("lease owner is required")
	}

	now := req.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultCoreMonitorClaimLimit
	}
	if limit > maxCoreMonitorClaimLimit {
		limit = maxCoreMonitorClaimLimit
	}

	leaseDuration := req.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = defaultCoreMonitorLeaseDuration
	}
	leaseExpiresAt := now.Add(leaseDuration)

	var claimed []db.CoreMonitorConfig
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var candidateIDs []string
		if err := tx.Model(&db.CoreMonitorConfig{}).
			Select("core_monitor_configs.monitor_id").
			Joins("JOIN monitors ON monitors.id = core_monitor_configs.monitor_id").
			Where("monitors.lifecycle = ?", "active").
			Where("core_monitor_configs.paused = ?", false).
			Where("core_monitor_configs.kind <> ?", "heartbeat").
			Where("core_monitor_configs.next_run_at <= ?", now).
			Where("(core_monitor_configs.lease_expires_at IS NULL OR core_monitor_configs.lease_expires_at <= ?)", now).
			Order("core_monitor_configs.next_run_at ASC").
			Limit(limit).
			Pluck("core_monitor_configs.monitor_id", &candidateIDs).Error; err != nil {
			return err
		}

		claimedIDs := make([]string, 0, len(candidateIDs))
		for _, monitorID := range candidateIDs {
			result := tx.Model(&db.CoreMonitorConfig{}).
				Where("monitor_id = ?", monitorID).
				Where("paused = ?", false).
				Where("kind <> ?", "heartbeat").
				Where("next_run_at <= ?", now).
				Where("(lease_expires_at IS NULL OR lease_expires_at <= ?)", now).
				Where("monitor_id IN (?)", tx.Model(&db.Monitor{}).Select("id").Where("lifecycle = ?", "active")).
				Updates(map[string]any{
					"lease_owner":      leaseOwner,
					"lease_expires_at": &leaseExpiresAt,
					"updated_at":       now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				claimedIDs = append(claimedIDs, monitorID)
			}
		}

		if len(claimedIDs) == 0 {
			claimed = []db.CoreMonitorConfig{}
			return nil
		}

		return tx.
			Where("monitor_id IN ?", claimedIDs).
			Order("next_run_at ASC").
			Find(&claimed).Error
	})
	if err != nil {
		s.logger.Error("Failed to claim due core monitor configs", "error", err)
		return nil, err
	}

	return claimed, nil
}

func (s *CoreMonitorSchedulerService) CompleteCoreMonitorCheck(req CompleteCoreMonitorCheckRequest) (*db.CoreMonitorConfig, error) {
	monitorID := strings.TrimSpace(req.MonitorID)
	if monitorID == "" {
		return nil, fmt.Errorf("monitor id is required")
	}
	leaseOwner := strings.TrimSpace(req.LeaseOwner)
	if leaseOwner == "" {
		return nil, fmt.Errorf("lease owner is required")
	}

	finishedAt := req.FinishedAt.UTC()
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}

	var updated db.CoreMonitorConfig
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var current db.CoreMonitorConfig
		if err := tx.Where("monitor_id = ?", monitorID).First(&current).Error; err != nil {
			return err
		}

		interval := time.Duration(current.IntervalSeconds) * time.Second
		if interval <= 0 {
			interval = defaultCoreMonitorInterval
		}

		nextRunAt := finishedAt.Add(interval)
		if req.NextRunAt != nil {
			nextRunAt = req.NextRunAt.UTC()
		}

		updates := map[string]any{
			"last_run_at":      &finishedAt,
			"next_run_at":      nextRunAt,
			"lease_owner":      "",
			"lease_expires_at": nil,
			"updated_at":       finishedAt,
		}
		if req.Success {
			updates["last_success_at"] = &finishedAt
		} else {
			updates["last_failure_at"] = &finishedAt
		}

		result := tx.Model(&db.CoreMonitorConfig{}).
			Where("monitor_id = ?", monitorID).
			Where("lease_owner = ?", leaseOwner).
			Where("lease_expires_at > ?", finishedAt).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCoreMonitorLeaseNotHeld
		}

		return tx.Where("monitor_id = ?", monitorID).First(&updated).Error
	})
	if err != nil {
		if !errors.Is(err, ErrCoreMonitorLeaseNotHeld) {
			s.logger.Error("Failed to complete core monitor check", "monitor_id", monitorID, "error", err)
		}
		return nil, err
	}

	return &updated, nil
}
