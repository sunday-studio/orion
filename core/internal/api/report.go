package api

import (
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// ReportRequest represents the report payload
type ReportRequest struct {
	Data interface{} `json:"data" binding:"required"`
}

// ReportResponse represents the response for a successful report
type ReportResponse struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	ReportID  uint      `json:"report_id"`
}

// receiveReport handles incoming telemetry reports from agents
func (s *Server) receiveReport(c *gin.Context) {
	// Get agent from context (set by ValidateAgentToken middleware)
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

	// Get raw JSON payload
	rawData, err := c.GetRawData()
	if err != nil {
		s.logger.Error("Failed to read request body", "error", err)
		utils.BadRequest(c, "Failed to read request body")
		return
	}

	// Convert to string for storage
	payload := string(rawData)
	if payload == "" {
		utils.BadRequest(c, "Empty payload not allowed")
		return
	}

	s.logger.Info("Received report", "agent_id", agentID, "agent_name", agent.(*db.Agent).Name, "payload_size", len(payload))

	// Store report
	if err := s.reportService.StoreReport(agentID.(uint), payload); err != nil {
		s.logger.Error("Failed to store report", "error", err)
		utils.InternalError(c, "Failed to store report", err)
		return
	}

	// Update agent's last_seen timestamp
	if err := s.agentService.UpdateLastSeen(agentID.(uint)); err != nil {
		s.logger.Warn("Failed to update last_seen timestamp", "error", err)
		// Don't fail the request for this
	}

	// Prepare response
	response := ReportResponse{
		Message:   "Report received successfully",
		Timestamp: time.Now(),
		ReportID:  0, // We could return the report ID if needed
	}

	s.logger.Info("Report processed successfully", "agent_id", agentID)
	utils.SuccessResponse(c, 200, "Report received successfully", response)
}
