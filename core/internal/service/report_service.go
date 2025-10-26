package service

import (
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"

	"gorm.io/gorm"
)

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

func (s *ReportService) StoreReport(agentID string, payload string) (*string, error) {
	reportID := utils.GenerateID("report")
	report := db.Report{
		ID:      reportID,
		AgentID: agentID,
		Payload: payload,
	}

	if err := s.db.Create(&report).Omit("Agent").Error; err != nil {
		s.logger.Error("Failed to store report", err)
		return nil, err
	}

	s.logger.Info("Report stored successfully", "report_id ->", report.ID)
	return &reportID, nil
}

func (s *ReportService) GetReportsByAgent(agentID string, limit int, offset int) ([]db.Report, error) {
	var reports []db.Report

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

func (s *ReportService) GetReportCount(agentID string) (int64, error) {
	var count int64

	if err := s.db.Model(&db.Report{}).Where("agent_id = ?", agentID).Count(&count).Error; err != nil {
		s.logger.Error("Failed to count reports", "agent_id", agentID, "error", err)
		return 0, err
	}

	return count, nil
}
