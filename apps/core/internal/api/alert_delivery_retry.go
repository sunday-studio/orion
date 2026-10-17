package api

import (
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// retryAlertDelivery re-queues a failed alert delivery.
// @Summary      Retry an alert delivery
// @Description  Re-queue a failed alert delivery so the Worker dispatcher sends it again. Only failed deliveries can be retried.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           retryAlertDelivery
// @Param        id   path      string  true  "Alert delivery ID"
// @Success      202  {object}  utils.APIResponse{data=object{delivery=AlertDeliveryResponse}}
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      409  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/deliveries/{id}/retry [post]
func (s *Server) retryAlertDelivery(c *gin.Context) {
	delivery, err := service.NewAlertService(s.db, s.logger, s.cfg).RetryDelivery(c.Param("id"))
	switch {
	case errors.Is(err, service.ErrAlertDeliveryNotFound):
		utils.NotFound(c, "Alert delivery not found")
		return
	case errors.Is(err, service.ErrAlertDeliveryNotRetryable):
		utils.ErrorResponse(c, http.StatusConflict, "Only failed deliveries can be retried", nil)
		return
	case err != nil:
		utils.InternalError(c, "Failed to retry alert delivery", err)
		return
	}
	s.logger.Info("Alert delivery re-queued", "delivery_id", delivery.ID, "actor", frontendActorID(c))
	responses := alertDeliveryResponses([]db.AlertDelivery{delivery})
	utils.SuccessResponse(c, http.StatusAccepted, "Alert delivery queued for retry", gin.H{"delivery": responses[0]})
}

func frontendActorID(c *gin.Context) string {
	if value, ok := c.Get("frontend_actor_id"); ok {
		if subject, ok := value.(string); ok && subject != "" {
			return subject
		}
	}
	return "admin"
}
