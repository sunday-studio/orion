package service

import (
	"encoding/json"
	"errors"
	"orion/core/internal/db"
	"strings"
	"time"

	"gorm.io/gorm"
)

type coreMonitorIncidentConfig struct {
	IncidentSeverity   string                         `json:"incident_severity"`
	MaintenanceWindows []coreMonitorMaintenanceWindow `json:"maintenance_windows"`
	Severity           string                         `json:"severity"`
}

type coreMonitorMaintenanceWindow struct {
	End     string `json:"end"`
	EndAt   string `json:"end_at"`
	Start   string `json:"start"`
	StartAt string `json:"start_at"`
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

func (s *IncidentService) coreMonitorMaintenanceActive(monitorID string, at time.Time) (bool, error) {
	var monitorConfig db.CoreMonitorConfig
	err := s.db.Where("monitor_id = ?", monitorID).First(&monitorConfig).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var config coreMonitorIncidentConfig
	if err := json.Unmarshal([]byte(monitorConfig.ConfigJSON), &config); err != nil {
		return false, nil
	}
	for _, window := range config.MaintenanceWindows {
		if window.contains(at) {
			return true, nil
		}
	}
	return false, nil
}

func (w coreMonitorMaintenanceWindow) contains(at time.Time) bool {
	start, ok := parseCoreMonitorMaintenanceTime(firstNonEmpty(w.StartAt, w.Start))
	if !ok {
		return false
	}
	end, ok := parseCoreMonitorMaintenanceTime(firstNonEmpty(w.EndAt, w.End))
	if !ok {
		return false
	}
	return !at.Before(start) && at.Before(end)
}

func parseCoreMonitorMaintenanceTime(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func monitorReportTime(payload MonitorReportPayload) time.Time {
	if reportedAt, err := time.Parse(time.RFC3339, payload.Timestamp); err == nil {
		return reportedAt
	}
	return time.Now().UTC()
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

func (s *IncidentService) isTLSExpiring(metrics any) bool {
	threshold := 14
	if s.cfg != nil {
		threshold = s.cfg.AlertTLSExpiryDays
	}
	if threshold <= 0 {
		return false
	}

	metricsMap, ok := metrics.(map[string]any)
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

func numericValue(value any) (float64, bool) {
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
