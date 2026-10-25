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

// receiveNodeReport receives a system report from a node
// @Summary      Receive node report
// @Description  Receive and store a system metrics report from a node
// @Tags         reports
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           receiveNodeReport
// @Param        node_id  path      string  true  "Node ID"
// @Param        data      body      service.NodeReportPayload  true  "Node report data"
// @Success      200       {object}  utils.APIResponse{data=ReportResponse}
// @Failure      400       {object}  utils.APIResponse
// @Failure      401       {object}  utils.APIResponse
// @Failure      500       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/report [post]
func (s *Server) receiveNodeReport(c *gin.Context) {
	startedAt := time.Now()
	var ingestionErr error
	defer func() {
		s.runtimeDiagnosticsService.RecordIngestion("node", time.Since(startedAt), ingestionErr)
	}()

	node, exists := c.Get("node")
	if !exists {
		s.logger.Error("Node not found in context")
		utils.InternalError(c, "Internal server error", nil)
		return
	}

	nodeID, exists := c.Get("node_id")
	if !exists {
		s.logger.Error("Node ID not found in context")
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

	s.logger.Info("Received report", "node_id", nodeID, "node_name", node.(*db.Node).Name, "payload_size", len(payload))

	var payloadData service.NodeReportPayload
	if err := json.Unmarshal(rawData, &payloadData); err != nil {
		s.logger.Error("Failed to unmarshal report", "error", err)
		utils.BadRequest(c, "Failed to unmarshal report")
		return
	}

	nodeReportID, err := s.reportService.StoreNodeReport(nodeID.(string), payloadData)
	if err != nil {
		ingestionErr = err
		s.logger.Error("Failed to store node report", "error", err)
		utils.InternalError(c, "Failed to store node report", err)
		return
	}

	// App measurements are a bonus on the report: failing to store them never fails it.
	if err := s.nodeService.StoreAppResources(nodeID.(string), payloadData.AppResources); err != nil {
		s.logger.Warn("Could not store app resources", "error", err, "node_id", nodeID)
	}

	// Update node last-seen timestamp
	if err := s.nodeService.UpdateLastSeen(nodeID.(string)); err != nil {
		s.logger.Error("Failed to update node last-seen", "error", err, "node_id", nodeID)
		// Don't fail the request if last-seen update fails
	}

	// A host that was reported offline is back: close its connectivity incident.
	if err := service.NewIncidentService(s.db, s.logger, s.cfg).ResolveNodeOffline(nodeID.(string)); err != nil {
		s.logger.Error("Failed to resolve node offline incident", "error", err, "node_id", nodeID)
	}

	// Prepare response
	response := ReportResponse{
		Message:   "Report received successfully",
		Timestamp: time.Now(),
		ReportID:  *nodeReportID,
		Type:      "node",
	}

	s.logger.Info("Report processed successfully", "node_id", nodeID)
	utils.SuccessResponse(c, 200, "Report received successfully", response)
}

// receiveMonitorReport receives a monitor report from a node
// @Summary      Receive monitor report
// @Description  Receive and store a monitor report for a specific monitor
// @Tags         reports
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           receiveMonitorReport
// @Param        node_id   path      string  true  "Node ID"
// @Param        monitor_id path      string  true  "Monitor ID"
// @Param        data       body      service.MonitorReportPayload  true  "Monitor report data"
// @Success      200        {object}  utils.APIResponse
// @Failure      400        {object}  utils.APIResponse
// @Failure      401        {object}  utils.APIResponse
// @Failure      500        {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/{monitor_id}/report [post]
func (s *Server) receiveMonitorReport(c *gin.Context) {
	startedAt := time.Now()
	var ingestionErr error
	defer func() {
		s.runtimeDiagnosticsService.RecordIngestion("monitor", time.Since(startedAt), ingestionErr)
	}()

	nodeID, nodeIDExists := c.Get("node_id")
	monitorID := c.Param("monitor_id")

	if monitorID == "" {
		utils.BadRequest(c, "Monitor ID is required")
		return
	}

	if !nodeIDExists {
		s.logger.Error("Node ID not found in context")
		utils.InternalError(c, "Internal server error", nil)
		return
	}
	nodeIDString := nodeID.(string)

	monitor, err := s.monitorService.GetMonitor(monitorID)
	if err != nil {
		utils.BadRequest(c, "Monitor not found")
		return
	}
	if monitor.NodeID != nodeIDString {
		utils.Unauthorized(c, "Monitor does not belong to this node")
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

	// Update node last-seen timestamp
	if err := s.nodeService.UpdateLastSeen(nodeID.(string)); err != nil {
		s.logger.Error("Failed to update node last-seen", "error", err, "node_id", nodeID)
		// Don't fail the request if last-seen update fails
	}

	utils.SuccessResponse(c, 200, "Monitor report received successfully", gin.H{
		"message": "Monitor report received successfully",
	})
}
