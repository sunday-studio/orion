package service

import (
	"errors"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

const (
	activeIncidentLookupSlowThreshold = 50 * time.Millisecond
	incidentReconcileSlowThreshold    = 100 * time.Millisecond
	coreMonitorRunner                 = "core"
	coreOwnerAgentID                  = "agent_core"
	coreOwnerMachineID                = "core"
	coreOwnerName                     = "Orion Core"
)

var (
	ErrIncidentAlreadyResolved = errors.New("incident already resolved")
	ErrIncidentNotFound        = errors.New("incident not found")
)

type IncidentLifecycleActionMetadata struct {
	ActorType string
	ActorID   string
	Note      string
}

type IncidentService struct {
	db          *gorm.DB
	logger      *utils.Logger
	cfg         *config.Config
	diagnostics *RuntimeDiagnosticsService
}

func NewIncidentService(database *gorm.DB, logger *utils.Logger, cfg *config.Config) *IncidentService {
	return &IncidentService{
		db:     database,
		logger: logger,
		cfg:    cfg,
	}
}

func (s *IncidentService) SetDiagnostics(diagnostics *RuntimeDiagnosticsService) {
	s.diagnostics = diagnostics
}

func (s *IncidentService) ReconcileMonitorReport(monitorID string, monitorReportID string, payload MonitorReportPayload) error {
	startedAt := time.Now()
	var reconcileErr error
	defer func() {
		duration := time.Since(startedAt)
		s.diagnostics.RecordIncidentReconciliation(duration, reconcileErr)
		if duration > incidentReconcileSlowThreshold {
			s.diagnostics.RecordSlowOperation("incident_reconciliation", monitorID, duration)
			s.logger.Warn("Slow incident reconciliation", "monitor_id", monitorID, "monitor_report_id", monitorReportID, "duration_ms", duration.Milliseconds())
		}
	}()

	var monitor db.Monitor
	if err := s.db.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
		reconcileErr = err
		return err
	}

	reportedHealth := payload.Health
	tlsExpiring := s.isTLSExpiring(payload.Metrics)
	nextIncidentState := incidentStateForReport(reportedHealth, tlsExpiring)
	if nextIncidentState == monitor.IncidentState && monitor.ActiveIncidentID == "" && nextIncidentState == "up" {
		return nil
	}

	agent, monitor, err := s.reportOwner(monitor, payload)
	if err != nil {
		reconcileErr = err
		return err
	}

	maintenanceActive, err := s.coreMonitorMaintenanceActive(monitor.ID, monitorReportTime(payload))
	if err != nil {
		reconcileErr = err
		return err
	}
	if maintenanceActive {
		s.logger.Info("Core monitor incident suppressed during maintenance window", "monitor_id", monitorID)
		activeIncidentID := monitor.ActiveIncidentID
		if activeIncidentID == "" {
			activeIncident, found, err := s.findActiveIncident(monitor.ID)
			if err != nil {
				reconcileErr = err
				return err
			}
			if found {
				activeIncidentID = activeIncident.ID
			}
		}
		reconcileErr = s.updateMonitorIncidentState(monitor.ID, activeIncidentID, "maintenance")
		return reconcileErr
	}

	if reportedHealth == "up" && !tlsExpiring {
		reconcileErr = s.resolveActiveIncidentIfRecovered(monitor, monitorReportID, nextIncidentState)
		return reconcileErr
	}

	if agent.MaintenanceMode {
		s.logger.Info("Incident suppressed during maintenance", "monitor_id", monitorID, "agent_id", agent.ID)
		reconcileErr = s.updateMonitorIncidentState(monitor.ID, monitor.ActiveIncidentID, nextIncidentState)
		return reconcileErr
	}

	if monitor.ActiveIncidentID == "" {
		activeIncident, found, err := s.findActiveIncident(monitor.ID)
		if err != nil {
			reconcileErr = err
			return err
		}
		if found {
			monitor.ActiveIncidentID = activeIncident.ID
		} else {
			confirmed, err := s.coreMonitorFailureConfirmed(monitor.ID, nextIncidentState)
			if err != nil {
				reconcileErr = err
				return err
			}
			if !confirmed {
				s.logger.Info("Core monitor incident deferred during confirmation period", "monitor_id", monitorID, "state", nextIncidentState)
				reconcileErr = s.updateMonitorIncidentState(monitor.ID, "", nextIncidentState)
				return reconcileErr
			}
		}
	}

	if tlsExpiring {
		reconcileErr = s.openOrUpdateIncident(agent, monitor, monitorReportID, "degraded", nextIncidentState)
		return reconcileErr
	}

	if reportedHealth == "down" || reportedHealth == "degraded" || reportedHealth == "stale" {
		reconcileErr = s.openOrUpdateIncident(agent, monitor, monitorReportID, reportedHealth, nextIncidentState)
		return reconcileErr
	}

	return nil
}

func (s *IncidentService) resolveActiveIncidentIfRecovered(monitor db.Monitor, monitorReportID string, incidentState string) error {
	activeIncidentID := monitor.ActiveIncidentID
	if activeIncidentID == "" {
		activeIncident, found, err := s.findActiveIncident(monitor.ID)
		if err != nil {
			return err
		}
		if found {
			activeIncidentID = activeIncident.ID
			monitor.ActiveIncidentID = activeIncident.ID
		}
	}
	if activeIncidentID == "" {
		return s.resolveActiveIncident(monitor, monitorReportID, incidentState)
	}

	confirmed, err := s.coreMonitorRecoveryConfirmed(monitor.ID)
	if err != nil {
		return err
	}
	if !confirmed {
		s.logger.Info("Core monitor incident kept open during recovery period", "monitor_id", monitor.ID)
		return s.updateMonitorIncidentState(monitor.ID, activeIncidentID, "recovering")
	}
	return s.resolveActiveIncident(monitor, monitorReportID, incidentState)
}

func (s *IncidentService) coreMonitorRecoveryConfirmed(monitorID string) (bool, error) {
	var config db.CoreMonitorConfig
	err := s.db.Where("monitor_id = ?", monitorID).First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if config.RecoveryPeriodSeconds <= 0 {
		return true, nil
	}

	streak, firstSuccessAt, latestSuccessAt, err := s.currentSuccessStreak(monitorID)
	if err != nil {
		return false, err
	}
	if streak == 0 || firstSuccessAt == nil || latestSuccessAt == nil {
		return false, nil
	}
	return latestSuccessAt.Sub(*firstSuccessAt) >= time.Duration(config.RecoveryPeriodSeconds)*time.Second, nil
}

func (s *IncidentService) coreMonitorFailureConfirmed(monitorID string, incidentState string) (bool, error) {
	if incidentState == "up" || incidentState == "unknown" {
		return true, nil
	}

	var config db.CoreMonitorConfig
	err := s.db.Where("monitor_id = ?", monitorID).First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if config.ConfirmationPeriodSeconds <= 0 && config.ConfirmationCheckCount <= 0 {
		return true, nil
	}

	streak, firstFailureAt, latestFailureAt, err := s.currentFailureStreak(monitorID)
	if err != nil {
		return false, err
	}
	if streak == 0 || firstFailureAt == nil || latestFailureAt == nil {
		return false, nil
	}

	periodConfirmed := false
	if config.ConfirmationPeriodSeconds > 0 {
		periodConfirmed = latestFailureAt.Sub(*firstFailureAt) >= time.Duration(config.ConfirmationPeriodSeconds)*time.Second
	}
	countConfirmed := false
	if config.ConfirmationCheckCount > 0 {
		countConfirmed = streak >= config.ConfirmationCheckCount
	}
	return periodConfirmed || countConfirmed, nil
}

func (s *IncidentService) currentFailureStreak(monitorID string) (int, *time.Time, *time.Time, error) {
	var reports []db.MonitorReport
	if err := s.db.Where("monitor_id = ?", monitorID).Order("created_at DESC").Limit(100).Find(&reports).Error; err != nil {
		return 0, nil, nil, err
	}

	streak := 0
	var firstFailureAt *time.Time
	var latestFailureAt *time.Time
	for _, report := range reports {
		if report.Health == "up" {
			break
		}
		if report.Health != "down" && report.Health != "degraded" && report.Health != "stale" {
			continue
		}
		streak++
		reportedAt := monitorReportConfirmationTime(report)
		if latestFailureAt == nil {
			latestFailureAt = &reportedAt
		}
		firstFailureAt = &reportedAt
	}
	return streak, firstFailureAt, latestFailureAt, nil
}

func monitorReportConfirmationTime(report db.MonitorReport) time.Time {
	if reportedAt, err := time.Parse(time.RFC3339, report.CollectedAt); err == nil {
		return reportedAt
	}
	return report.CreatedAt
}

func (s *IncidentService) currentSuccessStreak(monitorID string) (int, *time.Time, *time.Time, error) {
	var reports []db.MonitorReport
	if err := s.db.Where("monitor_id = ?", monitorID).Order("created_at DESC").Limit(100).Find(&reports).Error; err != nil {
		return 0, nil, nil, err
	}

	streak := 0
	var firstSuccessAt *time.Time
	var latestSuccessAt *time.Time
	for _, report := range reports {
		if report.Health == "down" || report.Health == "degraded" || report.Health == "stale" {
			break
		}
		if report.Health != "up" {
			continue
		}
		streak++
		reportedAt := monitorReportConfirmationTime(report)
		if latestSuccessAt == nil {
			latestSuccessAt = &reportedAt
		}
		firstSuccessAt = &reportedAt
	}
	return streak, firstSuccessAt, latestSuccessAt, nil
}

func (s *IncidentService) ReconcileStaleMonitors(agentID string) error {
	startedAt := time.Now()
	var reconcileErr error
	defer func() {
		duration := time.Since(startedAt)
		s.diagnostics.RecordIncidentReconciliation(duration, reconcileErr)
		if duration > incidentReconcileSlowThreshold {
			s.diagnostics.RecordSlowOperation("incident_reconciliation", "stale_monitors:"+agentID, duration)
			s.logger.Warn("Slow stale monitor reconciliation", "agent_id", agentID, "duration_ms", duration.Milliseconds())
		}
	}()

	var agent db.Agent
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		reconcileErr = err
		return err
	}
	if agent.MaintenanceMode {
		return nil
	}

	healthService := NewHealthService(s.db, s.logger)
	staleMonitors, err := healthService.DetectStaleMonitors(DefaultHealthConfig())
	if err != nil {
		reconcileErr = err
		return err
	}

	for _, monitor := range staleMonitors {
		if monitor.AgentID != agentID {
			continue
		}
		if err := s.openOrUpdateIncident(agent, monitor, "", "stale", "stale"); err != nil {
			reconcileErr = err
			return err
		}
	}

	return nil
}
