package service

import (
	"orion/core/internal/db"
	"orion/core/internal/logging"

	"gorm.io/gorm"
)

// ReportService handles report-related operations
type ReportService struct {
	db     *gorm.DB
	logger *logging.Logger
}

// NewReportService creates a new report service
func NewReportService(database *gorm.DB, logger *logging.Logger) *ReportService {
	return &ReportService{
		db:     database,
		logger: logger,
	}
}

// StoreReport stores a new report in the database
func (s *ReportService) StoreReport(agentID uint, payload string) error {
	report := db.Report{
		AgentID: agentID,
		Payload: payload,
	}

	if err := s.db.Create(&report).Error; err != nil {
		s.logger.Error("Failed to store report", "agent_id", agentID, "error", err)
		return err
	}

	s.logger.Info("Report stored successfully", "agent_id", agentID, "report_id", report.ID)
	return nil
}

// GetReportsByAgent retrieves all reports for a specific agent
func (s *ReportService) GetReportsByAgent(agentID uint, limit int, offset int) ([]db.Report, error) {
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

// GetReportCount returns the total number of reports for an agent
func (s *ReportService) GetReportCount(agentID uint) (int64, error) {
	var count int64
	
	if err := s.db.Model(&db.Report{}).Where("agent_id = ?", agentID).Count(&count).Error; err != nil {
		s.logger.Error("Failed to count reports", "agent_id", agentID, "error", err)
		return 0, err
	}

	return count, nil
}
