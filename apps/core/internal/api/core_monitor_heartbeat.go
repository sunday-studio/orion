package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	heartbeatPayloadMaxBytes = 4096
	heartbeatPayloadReadMax  = heartbeatPayloadMaxBytes + 1
)

type HeartbeatSignalResponse struct {
	MonitorID        string    `json:"monitor_id"`
	Health           string    `json:"health"`
	ReportID         string    `json:"report_id"`
	ReceivedAt       time.Time `json:"received_at"`
	PayloadTruncated bool      `json:"payload_truncated"`
}

// receiveHeartbeatSuccess records a public success signal for a heartbeat monitor.
// @Summary      Record heartbeat success
// @Description  Record a success signal for a Core heartbeat monitor using its generated token.
// @Tags         heartbeats
// @Accept       json
// @Produce      json
// @ID           receiveHeartbeatSuccess
// @Param        token  path      string true "Heartbeat token"
// @Success      200    {object}  utils.APIResponse{data=HeartbeatSignalResponse}
// @Failure      401    {object}  utils.APIResponse
// @Failure      500    {object}  utils.APIResponse
// @Router       /v1/heartbeats/{token}/success [post]
func (s *Server) receiveHeartbeatSuccess(c *gin.Context) {
	s.receiveHeartbeatSignal(c, "up")
}

// receiveHeartbeatFailure records a public failure signal for a heartbeat monitor.
// @Summary      Record heartbeat failure
// @Description  Record a failure signal for a Core heartbeat monitor using its generated token.
// @Tags         heartbeats
// @Accept       json
// @Produce      json
// @ID           receiveHeartbeatFailure
// @Param        token  path      string true "Heartbeat token"
// @Success      200    {object}  utils.APIResponse{data=HeartbeatSignalResponse}
// @Failure      401    {object}  utils.APIResponse
// @Failure      500    {object}  utils.APIResponse
// @Router       /v1/heartbeats/{token}/failure [post]
func (s *Server) receiveHeartbeatFailure(c *gin.Context) {
	s.receiveHeartbeatSignal(c, "down")
}

func (s *Server) receiveHeartbeatSignal(c *gin.Context, health string) {
	record, err := s.coreMonitorManagementService.GetHeartbeatMonitorByToken(c.Param("token"))
	if err != nil {
		if errors.Is(err, service.ErrCoreManagedMonitorNotFound) {
			utils.Unauthorized(c, "Invalid heartbeat token")
			return
		}
		utils.InternalError(c, "Failed to load heartbeat monitor", err)
		return
	}
	if record.Config.Paused {
		utils.Unauthorized(c, "Invalid heartbeat token")
		return
	}

	payload, truncated, err := readHeartbeatPayload(c)
	if err != nil {
		utils.BadRequest(c, "Invalid heartbeat payload")
		return
	}

	receivedAt := time.Now().UTC()
	reportPayload := service.MonitorReportPayload{
		Timestamp: receivedAt.Format(time.RFC3339),
		Health:    health,
		Metrics:   payload,
	}
	if health == "down" {
		reportPayload.Error = payload
	}
	reportID, err := s.reportService.StoreMonitorReport(record.Monitor.ID, reportPayload)
	if err != nil {
		utils.InternalError(c, "Failed to record heartbeat signal", err)
		return
	}
	if err := s.coreMonitorManagementService.RecordHeartbeatSignal(record.Monitor.ID, health, receivedAt); err != nil {
		utils.InternalError(c, "Failed to update heartbeat monitor", err)
		return
	}

	response := HeartbeatSignalResponse{
		MonitorID:        record.Monitor.ID,
		Health:           health,
		ReceivedAt:       receivedAt,
		PayloadTruncated: truncated,
	}
	if reportID != nil {
		response.ReportID = *reportID
	}
	utils.SuccessResponse(c, http.StatusOK, "Heartbeat signal recorded successfully", response)
}

func readHeartbeatPayload(c *gin.Context) (map[string]interface{}, bool, error) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, heartbeatPayloadReadMax))
	if err != nil {
		return nil, false, err
	}
	body = bytes.TrimSpace(body)
	truncated := len(body) > heartbeatPayloadMaxBytes
	if truncated {
		body = body[:heartbeatPayloadMaxBytes]
	}

	payload := map[string]interface{}{
		"runner":            "heartbeat",
		"payload_truncated": truncated,
	}
	if len(body) > 0 {
		payload["payload"] = redactHeartbeatText(string(body))
	}
	return payload, truncated, nil
}
