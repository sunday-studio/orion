package service

import (
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AgentReportPayload struct {
	UptimeSeconds uint64         `json:"uptime_seconds"`
	Timestamp     string         `json:"timestamp"`
	CPU           db.CPUStats    `json:"cpu"`
	Memory        db.MemoryStats `json:"memory"`
	Disk          db.DiskStats   `json:"disk"`
	Location      db.GeoLocation `json:"location,omitempty"`
}

type MonitorReportPayload struct {
	Timestamp string      `json:"timestamp" binding:"required"`
	Health    string      `json:"health" binding:"required"` // up | down
	Metrics   interface{} `json:"metrics" binding:"required"`
	Error     interface{} `json:"error,omitempty"`
}

type ReportService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewReportService(database *gorm.DB, logger *logging.Logger) *ReportService {
	return &ReportService{
		db:     database,
		logger: logger,
	}
}

func (s *ReportService) StoreMonitorReport(monitorID string, payload MonitorReportPayload) (*string, error) {
	monitorReportID := utils.GenerateID("monitor_report")

	// if health is down, store the error as payload
	var payloadData string

	// if payload.Health == "down" {
	// 	payloadJSON, err := json.Marshal(payload)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	payloadData = string(payloadJSON)
	// }

	if payload.Error != nil {
		payloadJSON, err := json.Marshal(payload.Error)
		if err != nil {
			return nil, err
		}
		payloadData = string(payloadJSON)
	}

	payloadJSON, err := json.Marshal(payload.Metrics)
	if err != nil {
		return nil, err
	}
	payloadData = string(payloadJSON)

	monitorReport := db.MonitorReport{
		ID:          monitorReportID,
		MonitorID:   monitorID,
		Health:      payload.Health,
		CollectedAt: payload.Timestamp,
		Payload:     payloadData,
	}

	if err := s.db.Create(&monitorReport).Error; err != nil {
		s.logger.Error("Failed to store monitor report", err)
		return nil, err
	}

	// Update monitor health and last successful report timestamp
	now := time.Now()
	updates := map[string]interface{}{
		"health": payload.Health,
	}

	// Only update last successful report if health is "up"
	if payload.Health == "up" {
		updates["last_successful_report_at"] = &now
	}

	if err := s.db.Model(&db.Monitor{}).Where("id = ?", monitorID).Updates(updates).Error; err != nil {
		s.logger.Error("Failed to update monitor health", "monitor_id", monitorID, "error", err)
		// Don't fail the request if monitor update fails
	}

	// Trigger health computation to update cache (async, don't block report storage)
	// This ensures cache is refreshed when new reports arrive
	go func() {
		healthService := NewHealthService(s.db, s.logger)
		config := DefaultHealthConfig()
		_, err := healthService.ComputeMonitorHealth(monitorID, config)
		if err != nil {
			s.logger.Error("Failed to compute health after report", "monitor_id", monitorID, "error", err)
		} else {
			s.logger.Debug("Health cache updated after report", "monitor_id", monitorID)
		}
	}()

	s.logger.Info("Monitor report stored successfully", "monitor_report_id ->", monitorReport.ID)
	return &monitorReportID, nil
}

func (s *ReportService) StoreAgentReport(agentID string, payload AgentReportPayload) (*string, error) {
	agentReportID := utils.GenerateID("agent_report")

	agentReport := db.AgentReport{
		ID:            agentReportID,
		AgentID:       agentID,
		UptimeSeconds: payload.UptimeSeconds,
		Timestamp:     payload.Timestamp,

		CPU:      datatypes.NewJSONType(payload.CPU),
		Memory:   datatypes.NewJSONType(payload.Memory),
		Disk:     datatypes.NewJSONType(payload.Disk),
		Location: datatypes.NewJSONType(payload.Location),
	}

	if err := s.db.Create(&agentReport).Omit("Agent").Error; err != nil {
		s.logger.Error("Failed to store agent report", err)
		return nil, err
	}

	s.logger.Info("Agent report stored successfully", "agent_report_id ->", agentReport.ID)
	return &agentReportID, nil
}

func (s *ReportService) GetAgentReportsById(agentID string, limit int, offset int) ([]db.AgentReport, error) {
	var reports []db.AgentReport

	query := s.db.Where("agent_id = ?", agentID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&reports).Error; err != nil {
		s.logger.Error("Failed to retrieve reports", "agent_id", agentID, "error", err)
		return nil, err
	}

	s.logger.Debug("Retrieved reports", "agent_id", agentID, "count", len(reports))
	return reports, nil
}

func (s *ReportService) GetAgentReportCountById(agentID string) (int64, error) {
	var count int64

	if err := s.db.Model(&db.AgentReport{}).Where("agent_id = ?", agentID).Count(&count).Error; err != nil {
		s.logger.Error("Failed to count reports", "agent_id", agentID, "error", err)
		return 0, err
	}

	return count, nil
}

func (s *ReportService) GetMonitorReports(monitorID string, limit int, offset int) ([]db.MonitorReport, error) {
	var reports []db.MonitorReport

	query := s.db.Where("monitor_id = ?", monitorID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&reports).Error; err != nil {
		s.logger.Error("Failed to retrieve monitor reports", "monitor_id", monitorID, "error", err)
		return nil, err
	}

	s.logger.Debug("Retrieved monitor reports", "monitor_id", monitorID, "count", len(reports))
	return reports, nil
}

func (s *ReportService) GetMonitorReportCount(monitorID string) (int64, error) {
	var count int64
	if err := s.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", monitorID).Count(&count).Error; err != nil {
		s.logger.Error("Failed to count monitor reports", "monitor_id", monitorID, "error", err)
		return 0, err
	}
	return count, nil
}
