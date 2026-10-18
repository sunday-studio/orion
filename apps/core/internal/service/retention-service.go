package service

import (
	"fmt"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

const (
	retentionBatchSize     = 1000
	incrementalVacuumPages = 2000
)

// RetentionResult counts rows pruned per table in one run.
type RetentionResult struct {
	ServiceLogEntries     int64 `json:"service_log_entries"`
	AlertDeliveryAttempts int64 `json:"alert_delivery_attempts"`
	AlertDeliveries       int64 `json:"alert_deliveries"`
	IncidentEvents        int64 `json:"incident_events"`
	AuditEvents           int64 `json:"audit_events"`
	DecommissionedAgents  int64 `json:"decommissioned_agents"`
	DeletedMonitors       int64 `json:"deleted_monitors"`
	WorkerStatuses        int64 `json:"worker_statuses"`
}

// RetentionService prunes the tables that otherwise grow forever. Every delete
// runs in short batches so it never holds the single SQLite writer for long.
type RetentionService struct {
	db      *gorm.DB
	logger  *utils.Logger
	dataDir string
}

func NewRetentionService(database *gorm.DB, logger *utils.Logger, dataDir string) *RetentionService {
	return &RetentionService{db: database, logger: logger, dataDir: dataDir}
}

func (s *RetentionService) Run(now time.Time) (*RetentionResult, error) {
	settings, err := NewSettingsService(s.db, s.logger, s.dataDir).GetDataLifecycleSettings()
	if err != nil {
		return nil, err
	}
	cutoff := func(days int) time.Time { return now.AddDate(0, 0, -days) }
	result := &RetentionResult{}

	steps := []struct {
		name string
		out  *int64
		run  func() (int64, error)
	}{
		{"service_log_entries", &result.ServiceLogEntries, func() (int64, error) {
			return s.pruneBatched("service_log_entries", "created_at < ?", cutoff(settings.ServiceLogRetentionDays))
		}},
		// Attempts first (they reference deliveries), then the deliveries. Pending
		// deliveries are never pruned: they are still waiting to be sent.
		{"alert_delivery_attempts", &result.AlertDeliveryAttempts, func() (int64, error) {
			// The per-recipient email ledger ages out with its deliveries.
			if _, err := s.pruneBatched("alert_delivery_recipients",
				"delivery_id IN (SELECT id FROM alert_deliveries WHERE created_at < ? AND status <> 'pending')",
				cutoff(settings.AlertDeliveryRetentionDays)); err != nil {
				return 0, err
			}
			return s.pruneBatched("alert_delivery_attempts",
				"alert_delivery_id IN (SELECT id FROM alert_deliveries WHERE created_at < ? AND status <> 'pending')",
				cutoff(settings.AlertDeliveryRetentionDays))
		}},
		{"alert_deliveries", &result.AlertDeliveries, func() (int64, error) {
			return s.pruneBatched("alert_deliveries", "created_at < ? AND status <> 'pending'", cutoff(settings.AlertDeliveryRetentionDays))
		}},
		// Only the history of resolved incidents ages out; active incidents keep their timeline.
		{"incident_events", &result.IncidentEvents, func() (int64, error) {
			return s.pruneBatched("incident_events",
				"created_at < ? AND incident_id IN (SELECT id FROM incidents WHERE status = 'resolved')",
				cutoff(settings.IncidentEventRetentionDays))
		}},
		{"decommissioned_agents", &result.DecommissionedAgents, func() (int64, error) {
			return PurgeDecommissionedAgents(s.db, s.logger, now)
		}},
		{"deleted_monitors", &result.DeletedMonitors, func() (int64, error) {
			return PurgeDeletedMonitors(s.db, s.logger, now)
		}},
		{"audit_events", &result.AuditEvents, func() (int64, error) {
			return s.pruneBatched("audit_events", "created_at < ?", cutoff(settings.AuditEventRetentionDays))
		}},
		{"core_worker_statuses", &result.WorkerStatuses, func() (int64, error) {
			return s.pruneBatched("core_worker_statuses", "last_heartbeat_at < ?", cutoff(settings.WorkerStatusRetentionDays))
		}},
	}
	var firstErr error
	for _, step := range steps {
		deleted, err := step.run()
		*step.out = deleted
		if err != nil {
			s.logger.Error("Retention step failed", "table", step.name, "error", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("retention %s: %w", step.name, err)
			}
		}
	}

	s.reclaimSpace()
	return result, firstErr
}

// pruneBatched deletes matching rows retentionBatchSize at a time until none remain.
func (s *RetentionService) pruneBatched(table string, where string, arg time.Time) (int64, error) {
	var total int64
	statement := fmt.Sprintf("DELETE FROM %s WHERE rowid IN (SELECT rowid FROM %s WHERE %s LIMIT %d)", table, table, where, retentionBatchSize)
	for {
		result := s.db.Exec(statement, arg)
		if result.Error != nil {
			return total, result.Error
		}
		total += result.RowsAffected
		if result.RowsAffected < retentionBatchSize {
			return total, nil
		}
	}
}

// reclaimSpace returns freed pages to the OS when the database uses
// auto_vacuum=INCREMENTAL (new databases do; see docs/sqlite-backup-restore.md
// for converting an existing one) and keeps the WAL file from growing.
func (s *RetentionService) reclaimSpace() {
	if err := s.db.Exec(fmt.Sprintf("PRAGMA incremental_vacuum(%d)", incrementalVacuumPages)).Error; err != nil {
		s.logger.Warn("Incremental vacuum failed", "error", err)
	}
	if err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
		s.logger.Warn("WAL checkpoint failed", "error", err)
	}
}
