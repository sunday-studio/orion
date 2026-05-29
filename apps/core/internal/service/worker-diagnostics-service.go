package service

import (
	"context"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const CoreMonitorWorkerProcessKind = "core-monitor-worker"

type WorkerHeartbeat struct {
	WorkerID        string
	Hostname        string
	Status          string
	Version         string
	StartedAt       time.Time
	LastHeartbeatAt time.Time
	LastError       string
}

type CoreWorkerDiagnostics struct {
	Status            string                     `json:"status"`
	StaleAfterSeconds int64                      `json:"stale_after_seconds"`
	WorkerCount       int                        `json:"worker_count"`
	OnlineCount       int                        `json:"online_count"`
	StaleCount        int                        `json:"stale_count"`
	Workers           []CoreWorkerDiagnosticsRow `json:"workers"`
}

type CoreWorkerDiagnosticsRow struct {
	WorkerID            string    `json:"worker_id"`
	ProcessKind         string    `json:"process_kind"`
	Hostname            string    `json:"hostname"`
	Status              string    `json:"status"`
	Health              string    `json:"health"`
	Version             string    `json:"version"`
	StartedAt           time.Time `json:"started_at"`
	LastHeartbeatAt     time.Time `json:"last_heartbeat_at"`
	HeartbeatAgeSeconds int64     `json:"heartbeat_age_seconds"`
	LastError           string    `json:"last_error,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type WorkerDiagnosticsService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewWorkerDiagnosticsService(database *gorm.DB, logger *logging.Logger) *WorkerDiagnosticsService {
	return &WorkerDiagnosticsService{
		db:     database,
		logger: logger,
	}
}

func (s *WorkerDiagnosticsService) RecordHeartbeat(ctx context.Context, heartbeat WorkerHeartbeat) error {
	now := heartbeat.LastHeartbeatAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	startedAt := heartbeat.StartedAt
	if startedAt.IsZero() {
		startedAt = now
	}
	status := heartbeat.Status
	if status == "" {
		status = "running"
	}

	row := db.CoreWorkerStatus{
		WorkerID:        heartbeat.WorkerID,
		ProcessKind:     CoreMonitorWorkerProcessKind,
		Hostname:        heartbeat.Hostname,
		Status:          status,
		Version:         heartbeat.Version,
		StartedAt:       startedAt,
		LastHeartbeatAt: now,
		LastError:       heartbeat.LastError,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "worker_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"process_kind",
			"hostname",
			"status",
			"version",
			"started_at",
			"last_heartbeat_at",
			"last_error",
			"updated_at",
		}),
	}).Create(&row).Error
}

func (s *WorkerDiagnosticsService) GetDiagnostics(ctx context.Context, staleAfter time.Duration, now time.Time) (CoreWorkerDiagnostics, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if staleAfter <= 0 {
		staleAfter = time.Minute
	}

	var rows []db.CoreWorkerStatus
	if err := s.db.WithContext(ctx).
		Where("process_kind = ?", CoreMonitorWorkerProcessKind).
		Order("last_heartbeat_at DESC").
		Find(&rows).Error; err != nil {
		s.logger.Error("Failed to load Core worker diagnostics", "error", err)
		return CoreWorkerDiagnostics{}, err
	}

	diagnostics := CoreWorkerDiagnostics{
		Status:            "unknown",
		StaleAfterSeconds: int64(staleAfter.Seconds()),
		WorkerCount:       len(rows),
		Workers:           make([]CoreWorkerDiagnosticsRow, 0, len(rows)),
	}
	if len(rows) == 0 {
		return diagnostics, nil
	}

	for _, row := range rows {
		age := now.Sub(row.LastHeartbeatAt)
		ageSeconds := int64(age.Seconds())
		if ageSeconds < 0 {
			ageSeconds = 0
		}
		health := "online"
		if row.Status == "error" {
			health = "error"
		} else if age > staleAfter {
			health = "stale"
		}

		switch health {
		case "online":
			diagnostics.OnlineCount++
		case "stale":
			diagnostics.StaleCount++
		}

		diagnostics.Workers = append(diagnostics.Workers, CoreWorkerDiagnosticsRow{
			WorkerID:            row.WorkerID,
			ProcessKind:         row.ProcessKind,
			Hostname:            row.Hostname,
			Status:              row.Status,
			Health:              health,
			Version:             row.Version,
			StartedAt:           row.StartedAt,
			LastHeartbeatAt:     row.LastHeartbeatAt,
			HeartbeatAgeSeconds: ageSeconds,
			LastError:           row.LastError,
			CreatedAt:           row.CreatedAt,
			UpdatedAt:           row.UpdatedAt,
		})
	}

	if diagnostics.StaleCount > 0 {
		diagnostics.Status = "degraded"
	} else if diagnostics.OnlineCount > 0 {
		diagnostics.Status = "healthy"
	}

	return diagnostics, nil
}
