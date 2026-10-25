package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
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

// receiveHeartbeatSuccessHeader records a success signal; the token travels in the Authorization header.
// @Summary      Record heartbeat success (header token)
// @Description  Same as the path-token form, but the token is sent as `Authorization: Bearer <token>` so it never lands in URLs, proxy access logs or shell history.
// @Tags         heartbeats
// @Accept       json
// @Produce      json
// @ID           receiveHeartbeatSuccessHeader
// @Security     HeartbeatBearer
// @Success      200    {object}  utils.APIResponse{data=HeartbeatSignalResponse}
// @Failure      401    {object}  utils.APIResponse
// @Failure      500    {object}  utils.APIResponse
// @Router       /v1/heartbeat/success [post]
func (s *Server) receiveHeartbeatSuccessHeader(c *gin.Context) { s.receiveHeartbeatSignal(c, "up") }

// receiveHeartbeatFailureHeader records a failure signal; the token travels in the Authorization header.
// @Summary      Record heartbeat failure (header token)
// @Description  Same as the path-token form, but the token is sent as `Authorization: Bearer <token>`.
// @Tags         heartbeats
// @Accept       json
// @Produce      json
// @ID           receiveHeartbeatFailureHeader
// @Security     HeartbeatBearer
// @Success      200    {object}  utils.APIResponse{data=HeartbeatSignalResponse}
// @Failure      401    {object}  utils.APIResponse
// @Failure      500    {object}  utils.APIResponse
// @Router       /v1/heartbeat/failure [post]
func (s *Server) receiveHeartbeatFailureHeader(c *gin.Context) { s.receiveHeartbeatSignal(c, "down") }

// heartbeatToken reads the token from the path (legacy) or an Authorization bearer header.
func heartbeatToken(c *gin.Context) string {
	if token := c.Param("token"); token != "" {
		return token
	}
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(header) > 7 && strings.EqualFold(header[:7], "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

func (s *Server) receiveHeartbeatSignal(c *gin.Context, health string) {
	record, err := s.coreMonitorManagementService.GetHeartbeatMonitorByToken(heartbeatToken(c))
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

func readHeartbeatPayload(c *gin.Context) (map[string]any, bool, error) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, heartbeatPayloadReadMax))
	if err != nil {
		return nil, false, err
	}
	body = bytes.TrimSpace(body)
	truncated := len(body) > heartbeatPayloadMaxBytes
	if truncated {
		body = body[:heartbeatPayloadMaxBytes]
	}

	payload := map[string]any{
		"runner":            "heartbeat",
		"payload_truncated": truncated,
	}
	if len(body) > 0 {
		payload["payload"] = service.SafeMonitorReportPayload(string(body))
	}
	return payload, truncated, nil
}
