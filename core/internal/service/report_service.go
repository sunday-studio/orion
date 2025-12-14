package service

import (
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"

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
