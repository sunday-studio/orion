package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"strings"
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

type IncidentService struct {
	db          *gorm.DB
	logger      *logging.Logger
	cfg         *config.Config
	diagnostics *RuntimeDiagnosticsService
}

func NewIncidentService(database *gorm.DB, logger *logging.Logger, cfg *config.Config) *IncidentService {
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

	if reportedHealth == "up" && !tlsExpiring {
		reconcileErr = s.resolveActiveIncident(monitor, monitorReportID, nextIncidentState)
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

func (s *IncidentService) AcknowledgeIncident(incidentID string) (db.Incident, error) {
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Incident{}, ErrIncidentNotFound
		}
		return db.Incident{}, err
	}
	if incident.Status == "resolved" {
		return db.Incident{}, ErrIncidentAlreadyResolved
	}
	if incident.Status == "acknowledged" {
		return incident, nil
	}

	now := time.Now().UTC()
	message := "Incident manually acknowledged"
	if err := s.db.Model(&incident).Updates(map[string]interface{}{
		"status":        "acknowledged",
		"last_event_at": now,
		"latest_event":  message,
	}).Error; err != nil {
		return db.Incident{}, err
	}
	if err := s.createIncidentEvent(incident.ID, "incident_acknowledged", message, ""); err != nil {
		return db.Incident{}, err
	}
	if err := s.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		return db.Incident{}, err
	}
	return incident, nil
}

func (s *IncidentService) ResolveIncident(incidentID string) (db.Incident, error) {
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Incident{}, ErrIncidentNotFound
		}
		return db.Incident{}, err
	}
	if incident.Status == "resolved" {
		return incident, nil
	}

	message := "Incident manually resolved"
	if err := s.resolveIncidentRecord(&incident, message, "", "up", true); err != nil {
		return db.Incident{}, err
	}
	if err := s.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		return db.Incident{}, err
	}
	return incident, nil
}

func (s *IncidentService) ResolveMonitorRemoved(monitorID string) error {
	incident, found, err := s.findActiveIncident(monitorID)
	if err != nil {
		return err
	}
	if !found {
		return s.updateMonitorIncidentState(monitorID, "", "unknown")
	}
	message := "Monitor removed; active incident resolved"
	return s.resolveIncidentRecord(incident, message, "", "unknown", true)
}

func (s *IncidentService) openOrUpdateIncident(agent db.Agent, monitor db.Monitor, monitorReportID string, health string, incidentState string) error {
	now := time.Now().UTC()
	message := incidentMessage(agent, monitor, health)
	severity := s.monitorIncidentSeverity(agent, monitor, health)

	if monitor.ActiveIncidentID != "" {
		if updated, err := s.updateActiveIncidentByID(monitor.ActiveIncidentID, monitor.ID, monitorReportID, severity, message, incidentState); err != nil {
			return err
		} else if updated {
			return nil
		}
	}

	incident, found, err := s.findActiveIncident(monitor.ID)
	if err != nil {
		return err
	}
	if found {
		updates := map[string]interface{}{
			"severity":      severity,
			"last_event_at": now,
			"latest_event":  message,
		}
		if err := s.db.Model(incident).Updates(updates).Error; err != nil {
			return err
		}
		if err := s.updateMonitorIncidentState(monitor.ID, incident.ID, incidentState); err != nil {
			return err
		}
		return s.createIncidentEvent(incident.ID, "monitor_failed", message, monitorReportID)
	}

	newIncident := db.Incident{
		ID:                 utils.GenerateID("incident"),
		Status:             "open",
		Severity:           severity,
		Title:              incidentTitle(agent, monitor, health),
		AgentID:            agent.ID,
		MonitorID:          monitor.ID,
		OpenedAt:           now,
		LastEventAt:        now,
		LatestEvent:        message,
		NotificationStatus: "pending",
	}
	if err := s.db.Create(&newIncident).Error; err != nil {
		return err
	}

	if err := s.updateMonitorIncidentState(monitor.ID, newIncident.ID, incidentState); err != nil {
		return err
	}

	if err := s.createIncidentEvent(newIncident.ID, "incident_opened", message, monitorReportID); err != nil {
		return err
	}
	return NewAlertService(s.db, s.logger, s.cfg).QueueIncidentNotifications(newIncident.ID, "incident_opened")
}

func (s *IncidentService) resolveActiveIncident(monitor db.Monitor, monitorReportID string, incidentState string) error {
	now := time.Now().UTC()
	message := fmt.Sprintf("Monitor %s recovered", monitor.Name)

	if monitor.ActiveIncidentID != "" {
		if resolved, err := s.resolveActiveIncidentByID(monitor.ActiveIncidentID, monitor.ID, monitorReportID, message, incidentState); err != nil {
			return err
		} else if resolved {
			return nil
		}
	}

	incident, found, err := s.findActiveIncident(monitor.ID)
	if err != nil {
		return err
	}
	if !found {
		return s.updateMonitorIncidentState(monitor.ID, "", incidentState)
	}

	updates := map[string]interface{}{
		"status":        "resolved",
		"resolved_at":   &now,
		"last_event_at": now,
		"latest_event":  message,
	}
	if err := s.db.Model(incident).Updates(updates).Error; err != nil {
		return err
	}

	if err := s.updateMonitorIncidentState(monitor.ID, "", incidentState); err != nil {
		return err
	}

	if err := s.createIncidentEvent(incident.ID, "incident_resolved", message, monitorReportID); err != nil {
		return err
	}
	if s.cfg != nil && !s.cfg.AlertRecoveryNotifications {
		return nil
	}
	return NewAlertService(s.db, s.logger, s.cfg).QueueIncidentNotifications(incident.ID, "incident_resolved")
}

func (s *IncidentService) resolveIncidentRecord(incident *db.Incident, message string, monitorReportID string, incidentState string, notify bool) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":        "resolved",
		"resolved_at":   &now,
		"last_event_at": now,
		"latest_event":  message,
	}
	if err := s.db.Model(incident).Where("status IN ?", []string{"open", "acknowledged"}).Updates(updates).Error; err != nil {
		return err
	}
	if err := s.updateMonitorIncidentState(incident.MonitorID, "", incidentState); err != nil {
		return err
	}
	if err := s.createIncidentEvent(incident.ID, "incident_resolved", message, monitorReportID); err != nil {
		return err
	}
	if !notify || (s.cfg != nil && !s.cfg.AlertRecoveryNotifications) {
		return nil
	}
	return NewAlertService(s.db, s.logger, s.cfg).QueueIncidentNotifications(incident.ID, "incident_resolved")
}

func (s *IncidentService) findActiveIncident(monitorID string) (*db.Incident, bool, error) {
	startedAt := time.Now()
	var incident db.Incident
	result := s.db.Where("monitor_id = ? AND status IN ?", monitorID, []string{"open", "acknowledged"}).
		Order("opened_at DESC").
		Limit(1).
		Find(&incident)
	duration := time.Since(startedAt)
	found := result.RowsAffected > 0
	s.diagnostics.RecordActiveIncidentLookup(duration, found, result.Error)
	if duration > activeIncidentLookupSlowThreshold {
		s.diagnostics.RecordSlowOperation("active_incident_lookup", monitorID, duration)
		s.logger.Warn("Slow active incident lookup", "monitor_id", monitorID, "duration_ms", duration.Milliseconds())
	}
	if result.Error != nil {
		return nil, false, result.Error
	}
	if !found {
		return nil, false, nil
	}
	return &incident, true, nil
}

func (s *IncidentService) updateActiveIncidentByID(incidentID string, monitorID string, monitorReportID string, severity string, message string, incidentState string) (bool, error) {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"severity":      severity,
		"last_event_at": now,
		"latest_event":  message,
	}
	result := s.db.Model(&db.Incident{}).
		Where("id = ? AND monitor_id = ? AND status IN ?", incidentID, monitorID, []string{"open", "acknowledged"}).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		if err := s.updateMonitorIncidentState(monitorID, "", incidentState); err != nil {
			return false, err
		}
		return false, nil
	}
	if err := s.updateMonitorIncidentState(monitorID, incidentID, incidentState); err != nil {
		return true, err
	}
	return true, s.createIncidentEvent(incidentID, "monitor_failed", message, monitorReportID)
}

func (s *IncidentService) resolveActiveIncidentByID(incidentID string, monitorID string, monitorReportID string, message string, incidentState string) (bool, error) {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":        "resolved",
		"resolved_at":   &now,
		"last_event_at": now,
		"latest_event":  message,
	}
	result := s.db.Model(&db.Incident{}).
		Where("id = ? AND monitor_id = ? AND status IN ?", incidentID, monitorID, []string{"open", "acknowledged"}).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		if err := s.updateMonitorIncidentState(monitorID, "", incidentState); err != nil {
			return false, err
		}
		return false, nil
	}
	if err := s.updateMonitorIncidentState(monitorID, "", incidentState); err != nil {
		return true, err
	}
	if err := s.createIncidentEvent(incidentID, "incident_resolved", message, monitorReportID); err != nil {
		return true, err
	}
	if s.cfg != nil && !s.cfg.AlertRecoveryNotifications {
		return true, nil
	}
	return true, NewAlertService(s.db, s.logger, s.cfg).QueueIncidentNotifications(incidentID, "incident_resolved")
}

func (s *IncidentService) updateMonitorIncidentState(monitorID string, activeIncidentID string, incidentState string) error {
	return s.db.Model(&db.Monitor{}).Where("id = ?", monitorID).Updates(map[string]interface{}{
		"active_incident_id": activeIncidentID,
		"incident_state":     incidentState,
	}).Error
}

func (s *IncidentService) createIncidentEvent(incidentID string, eventType string, message string, monitorReportID string) error {
	event := db.IncidentEvent{
		ID:              utils.GenerateID("incident_event"),
		IncidentID:      incidentID,
		Type:            eventType,
		Message:         message,
		MonitorReportID: monitorReportID,
	}
	return s.db.Create(&event).Error
}

func (s *IncidentService) reportOwner(monitor db.Monitor, payload MonitorReportPayload) (db.Agent, db.Monitor, error) {
	var agent db.Agent
	if monitor.AgentID != "" {
		err := s.db.Where("id = ?", monitor.AgentID).First(&agent).Error
		if err == nil {
			return agent, monitor, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Agent{}, monitor, err
		}
		if !isCoreProducedMonitorReport(payload) {
			return db.Agent{}, monitor, err
		}
	}

	if !isCoreProducedMonitorReport(payload) {
		return db.Agent{}, monitor, gorm.ErrRecordNotFound
	}

	agent, err := s.ensureCoreOwnerAgent(monitor.AgentID)
	if err != nil {
		return db.Agent{}, monitor, err
	}
	if monitor.AgentID != agent.ID {
		if err := s.db.Model(&db.Monitor{}).Where("id = ?", monitor.ID).Update("agent_id", agent.ID).Error; err != nil {
			return db.Agent{}, monitor, err
		}
		monitor.AgentID = agent.ID
	}
	return agent, monitor, nil
}

func (s *IncidentService) ensureCoreOwnerAgent(preferredID string) (db.Agent, error) {
	var agent db.Agent
	if preferredID != "" {
		err := s.db.Where("id = ?", preferredID).First(&agent).Error
		if err == nil {
			return agent, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Agent{}, err
		}
	}

	err := s.db.Where("machine_id = ?", coreOwnerMachineID).First(&agent).Error
	if err == nil {
		return agent, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Agent{}, err
	}

	ownerID := strings.TrimSpace(preferredID)
	if ownerID == "" {
		ownerID = coreOwnerAgentID
	}
	token, err := utils.GenerateToken()
	if err != nil {
		return db.Agent{}, err
	}

	now := time.Now().UTC()
	agent = db.Agent{
		ID:                       ownerID,
		MachineId:                coreOwnerMachineID,
		Name:                     coreOwnerName,
		OS:                       "core",
		Arch:                     "internal",
		Token:                    token,
		ReportingIntervalSeconds: 60,
		CreatedAt:                now,
		LastSeen:                 now,
		Meta:                     `{"owner":"core"}`,
	}
	if err := s.db.Create(&agent).Error; err != nil {
		var existing db.Agent
		if findErr := s.db.Where("machine_id = ? OR id = ?", coreOwnerMachineID, ownerID).First(&existing).Error; findErr == nil {
			return existing, nil
		}
		return db.Agent{}, err
	}
	return agent, nil
}

func isCoreProducedMonitorReport(payload MonitorReportPayload) bool {
	return mapFieldIsCore(payload.Metrics, "runner") ||
		mapFieldIsCore(payload.Metrics, "source") ||
		mapFieldIsCore(payload.Error, "runner") ||
		mapFieldIsCore(payload.Error, "source")
}

func mapFieldIsCore(value interface{}, key string) bool {
	fields, ok := value.(map[string]interface{})
	if !ok {
		return false
	}
	raw, ok := fields[key]
	if !ok {
		return false
	}
	text, ok := raw.(string)
	return ok && isCoreValue(text)
}

func isCoreValue(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), coreMonitorRunner)
}

func incidentTitle(agent db.Agent, monitor db.Monitor, health string) string {
	if isCoreOwnerAgent(agent) {
		return fmt.Sprintf("Core monitor %s: %s", health, monitor.Name)
	}
	return fmt.Sprintf("%s is %s", monitor.Name, health)
}

func incidentMessage(agent db.Agent, monitor db.Monitor, health string) string {
	if isCoreOwnerAgent(agent) {
		return fmt.Sprintf("Core monitor %s reported %s", monitor.Name, health)
	}
	return fmt.Sprintf("Monitor %s reported %s", monitor.Name, health)
}

func isCoreOwnerAgent(agent db.Agent) bool {
	return agent.MachineId == coreOwnerMachineID
}

type coreMonitorIncidentConfig struct {
	IncidentSeverity string `json:"incident_severity"`
	Severity         string `json:"severity"`
}

func (s *IncidentService) monitorIncidentSeverity(agent db.Agent, monitor db.Monitor, health string) string {
	if !isCoreOwnerAgent(agent) {
		return incidentSeverity(health)
	}

	var monitorConfig db.CoreMonitorConfig
	if err := s.db.Where("monitor_id = ?", monitor.ID).First(&monitorConfig).Error; err != nil {
		return incidentSeverity(health)
	}
	if override, ok := coreMonitorIncidentSeverityOverride(monitorConfig.ConfigJSON); ok {
		return override
	}
	return coreMonitorIncidentSeverityDefault(monitorConfig.Kind, health)
}

func coreMonitorIncidentSeverityOverride(configJSON string) (string, bool) {
	var config coreMonitorIncidentConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return "", false
	}
	severity := normalizeIncidentSeverity(config.IncidentSeverity)
	if severity == "" {
		severity = normalizeIncidentSeverity(config.Severity)
	}
	if !validIncidentSeverity(severity) {
		return "", false
	}
	return severity, true
}

func coreMonitorIncidentSeverityDefault(kind string, health string) string {
	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	switch normalizedKind {
	case "tls", "tls_certificate", "domain_expiration":
		if health == "degraded" {
			return "medium"
		}
	case "synthetic", "synthetic_multi_step", "playwright", "playwright_transaction":
		if health == "down" || health == "stale" {
			return "high"
		}
	case "http", "http_status", "http_keyword", "expected_status", "api_request", "tcp", "tcp_port", "dns", "udp", "ping", "mail", "smtp", "imap", "pop", "pop3":
		if health == "down" || health == "stale" {
			return "high"
		}
	}
	return incidentSeverity(health)
}

func normalizeIncidentSeverity(severity string) string {
	return strings.ToLower(strings.TrimSpace(severity))
}

func validIncidentSeverity(severity string) bool {
	switch severity {
	case "low", "medium", "high", "critical", "error":
		return true
	default:
		return false
	}
}

func incidentSeverity(health string) string {
	switch health {
	case "down", "stale":
		return "high"
	case "degraded":
		return "medium"
	default:
		return "low"
	}
}

func incidentStateForReport(reportedHealth string, tlsExpiring bool) string {
	if tlsExpiring {
		return "degraded"
	}
	switch reportedHealth {
	case "down", "degraded", "stale":
		return reportedHealth
	case "up":
		return "up"
	default:
		return "unknown"
	}
}

func (s *IncidentService) isTLSExpiring(metrics interface{}) bool {
	threshold := 14
	if s.cfg != nil {
		threshold = s.cfg.AlertTLSExpiryDays
	}
	if threshold <= 0 {
		return false
	}

	metricsMap, ok := metrics.(map[string]interface{})
	if !ok {
		return false
	}

	rawDays, exists := metricsMap["tls_days_remaining"]
	if !exists {
		return false
	}

	days, ok := numericValue(rawDays)
	return ok && days <= float64(threshold)
}

func numericValue(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
