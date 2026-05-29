package api

import (
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type ReportResponse struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	ReportID  string    `json:"report_id"`
	Type      string    `json:"type"`
}

// receiveAgentReport receives a system report from an agent
// @Summary      Receive agent report
// @Description  Receive and store a system metrics report from an agent
// @Tags         reports
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           receiveAgentReport
// @Param        agent_id  path      string  true  "Agent ID"
// @Param        data      body      service.AgentReportPayload  true  "Agent report data"
// @Success      200       {object}  utils.APIResponse{data=ReportResponse}
// @Failure      400       {object}  utils.APIResponse
// @Failure      401       {object}  utils.APIResponse
// @Failure      500       {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/report [post]
func (s *Server) receiveAgentReport(c *gin.Context) {
	startedAt := time.Now()
	var ingestionErr error
	defer func() {
		s.runtimeDiagnosticsService.RecordIngestion("agent", time.Since(startedAt), ingestionErr)
	}()

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

	var payloadData service.AgentReportPayload
	if err := json.Unmarshal(rawData, &payloadData); err != nil {
		s.logger.Error("Failed to unmarshal report", "error", err)
		utils.BadRequest(c, "Failed to unmarshal report")
		return
	}

	agentReportID, err := s.reportService.StoreAgentReport(agentID.(string), payloadData)
	if err != nil {
		ingestionErr = err
		s.logger.Error("Failed to store agent report", "error", err)
		utils.InternalError(c, "Failed to store agent report", err)
		return
	}

	// Update agent last-seen timestamp
	if err := s.agentService.UpdateLastSeen(agentID.(string)); err != nil {
		s.logger.Error("Failed to update agent last-seen", "error", err, "agent_id", agentID)
		// Don't fail the request if last-seen update fails
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

// receiveMonitorReport receives a monitor report from an agent
// @Summary      Receive monitor report
// @Description  Receive and store a monitor report for a specific monitor
// @Tags         reports
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           receiveMonitorReport
// @Param        agent_id   path      string  true  "Agent ID"
// @Param        monitor_id path      string  true  "Monitor ID"
// @Param        data       body      service.MonitorReportPayload  true  "Monitor report data"
// @Success      200        {object}  utils.APIResponse
// @Failure      400        {object}  utils.APIResponse
// @Failure      401        {object}  utils.APIResponse
// @Failure      500        {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/{monitor_id}/report [post]
func (s *Server) receiveMonitorReport(c *gin.Context) {
	startedAt := time.Now()
	var ingestionErr error
	defer func() {
		s.runtimeDiagnosticsService.RecordIngestion("monitor", time.Since(startedAt), ingestionErr)
	}()

	agentID, agentIDExists := c.Get("agent_id")
	monitorID := c.Param("monitor_id")

	if monitorID == "" {
		utils.BadRequest(c, "Monitor ID is required")
		return
	}

	if !agentIDExists {
		s.logger.Error("Agent ID not found in context")
		utils.InternalError(c, "Internal server error", nil)
		return
	}
	agentIDString := agentID.(string)

	monitor, err := s.monitorService.GetMonitor(monitorID)
	if err != nil {
		utils.BadRequest(c, "Monitor not found")
		return
	}
	if monitor.AgentID != agentIDString {
		utils.Unauthorized(c, "Monitor does not belong to this agent")
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

	var payloadData service.MonitorReportPayload
	if err := json.Unmarshal(rawData, &payloadData); err != nil {
		trunc := string(rawData)
		if len(trunc) > 200 {
			trunc = trunc[:200] + "…"
		}
		s.logger.Error("Failed to unmarshal report", "error", err, "rawData", trunc)
		utils.BadRequest(c, "Failed to unmarshal report")
		return
	}

	if _, err = s.reportService.StoreMonitorReport(monitorID, payloadData); err != nil {
		ingestionErr = err
		s.logger.Error("Failed to store monitor report", "error", err)
		utils.InternalError(c, "Failed to store monitor report", nil)
		return
	}

	// Update agent last-seen timestamp
	if err := s.agentService.UpdateLastSeen(agentID.(string)); err != nil {
		s.logger.Error("Failed to update agent last-seen", "error", err, "agent_id", agentID)
		// Don't fail the request if last-seen update fails
	}

	utils.SuccessResponse(c, 200, "Monitor report received successfully", gin.H{
		"message": "Monitor report received successfully",
	})
}
