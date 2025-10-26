package api

import (
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type ReportRequest struct {
	Data interface{} `json:"data" binding:"required"`
}

type ReportResponse struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	ReportID  string    `json:"report_id"`
	Type      string    `json:"type"`
}

func (s *Server) receiveReport(c *gin.Context) {
	agent, exists := c.Get("agent")
	if !exists {
		s.logger.Error("Agent not found in context")
		utils.InternalError(c, "Internal server error", nil)
		return
	}

	agentID, exists := c.Get("agent_id")
	if !exists {
		s.logger.Error("Agent ID not found in context")
		utils.InternalError(c, "Internal server error", nil)
		return
	}

	rawData, err := c.GetRawData()
	if err != nil {
		s.logger.Error("Failed to read request body", "error", err)
		utils.BadRequest(c, "Failed to read request body")
		return
	}

	payload := string(rawData)
	if payload == "" {
		utils.BadRequest(c, "Empty payload not allowed")
		return
	}

	s.logger.Info("Received report", "agent_id", agentID, "agent_name", agent.(*db.Agent).Name, "payload_size", len(payload))

	agentReportID, err := s.reportService.StoreAgentReport(agentID.(string), payload)
	if err != nil {
		s.logger.Error("Failed to store agent report", "error", err)
		utils.InternalError(c, "Failed to store agent report", err)
		return
	}

	if err := s.agentService.UpdateLastSeen(agentID.(string)); err != nil {
		s.logger.Warn("Failed to update last_seen timestamp", "error", err)
	}

	// Prepare response
	response := ReportResponse{
		Message:   "Report received successfully",
		Timestamp: time.Now(),
		ReportID:  *agentReportID,
		Type:      "agent",
	}

	s.logger.Info("Report processed successfully", "agent_id", agentID)
	utils.SuccessResponse(c, 200, "Report received successfully", response)
}
