package api

import (
	"errors"
	"html"
	"html/template"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/publicpage"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type alertRecipientRequest struct {
	Email string `json:"email"`
}

// AlertRecipientResponse is one email recipient of an alert channel.
type AlertRecipientResponse struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	Status      string  `json:"status"` // pending | confirmed | unsubscribed
	ConfirmedAt *string `json:"confirmed_at,omitempty"`
}

func alertRecipientResponse(r db.AlertChannelRecipient) AlertRecipientResponse {
	out := AlertRecipientResponse{ID: r.ID, Email: r.Email, Status: r.Status}
	if r.ConfirmedAt != nil {
		v := r.ConfirmedAt.UTC().Format("2006-01-02T15:04:05Z")
		out.ConfirmedAt = &v
	}
	return out
}

// listAlertChannelRecipients lists recipients of an email alert channel.
// @Summary      List alert channel recipients
// @Tags         alerts
// @Produce      json
// @ID           getAlertChannelRecipients
// @Param        id   path      string  true  "Alert channel ID"
// @Success      200  {object}  utils.APIResponse{data=object{recipients=[]AlertRecipientResponse,count=int}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/channels/{id}/recipients [get]
func (s *Server) listAlertChannelRecipients(c *gin.Context) {
	recipients, err := service.NewAlertService(s.db, s.logger, s.cfg).ListRecipients(c.Param("id"))
	if err != nil {
		utils.InternalError(c, "Failed to list recipients", err)
		return
	}
	out := make([]AlertRecipientResponse, 0, len(recipients))
	for _, r := range recipients {
		resp := alertRecipientResponse(r)
		if isAPITokenCaller(c) {
			resp.Email = maskEmailAddress(resp.Email) // tokens (dashboards, scrapers) never see full addresses
		}
		out = append(out, resp)
	}
	utils.SuccessResponse(c, http.StatusOK, "Recipients retrieved successfully", gin.H{"recipients": out, "count": len(out)})
}

// addAlertChannelRecipient adds a recipient and sends the confirmation email.
// @Summary      Add alert channel recipient
// @Description  Adds an address to an email channel and mails it a confirmation link. No alerts are sent until it confirms (double opt-in).
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           addAlertChannelRecipient
// @Param        id       path      string                 true  "Alert channel ID"
// @Param        request  body      alertRecipientRequest  true  "Recipient"
// @Success      201      {object}  utils.APIResponse{data=object{recipient=AlertRecipientResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      502      {object}  utils.APIResponse
// @Router       /v1/alerts/channels/{id}/recipients [post]
func (s *Server) addAlertChannelRecipient(c *gin.Context) {
	var request alertRecipientRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid recipient payload")
		return
	}
	recipient, err := service.NewAlertService(s.db, s.logger, s.cfg).AddRecipient(c.Param("id"), request.Email)
	switch {
	case err == nil:
		utils.SuccessResponse(c, http.StatusCreated, "Recipient added; confirmation email sent", gin.H{"recipient": alertRecipientResponse(*recipient)})
	case errors.Is(err, gorm.ErrRecordNotFound):
		utils.NotFound(c, "Email alert channel not found")
	case errors.Is(err, service.ErrAlertRecipientInvalid), errors.Is(err, service.ErrAlertRecipientLimit):
		utils.BadRequest(c, err.Error())
	default:
		s.logger.Error("Failed to add alert recipient", "error", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Recipient saved but the confirmation email could not be sent", nil)
	}
}

// removeAlertChannelRecipient removes a recipient.
// @Summary      Remove alert channel recipient
// @Tags         alerts
// @Produce      json
// @ID           removeAlertChannelRecipient
// @Param        id            path      string  true  "Alert channel ID"
// @Param        recipient_id  path      string  true  "Recipient ID"
// @Success      200           {object}  utils.APIResponse
// @Failure      404           {object}  utils.APIResponse
// @Router       /v1/alerts/channels/{id}/recipients/{recipient_id} [delete]
func (s *Server) removeAlertChannelRecipient(c *gin.Context) {
	err := service.NewAlertService(s.db, s.logger, s.cfg).RemoveRecipient(c.Param("id"), c.Param("recipient_id"))
	if errors.Is(err, service.ErrAlertRecipientNotFound) {
		utils.NotFound(c, "Recipient not found")
		return
	}
	if err != nil {
		utils.InternalError(c, "Failed to remove recipient", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Recipient removed", gin.H{})
}

// resendAlertChannelRecipient resends the confirmation email (at most once a minute).
// @Summary      Resend recipient confirmation
// @Tags         alerts
// @Produce      json
// @ID           resendAlertChannelRecipient
// @Param        id            path      string  true  "Alert channel ID"
// @Param        recipient_id  path      string  true  "Recipient ID"
// @Success      200           {object}  utils.APIResponse
// @Failure      404           {object}  utils.APIResponse
// @Failure      502           {object}  utils.APIResponse
// @Router       /v1/alerts/channels/{id}/recipients/{recipient_id}/resend [post]
func (s *Server) resendAlertChannelRecipient(c *gin.Context) {
	err := service.NewAlertService(s.db, s.logger, s.cfg).ResendConfirmation(c.Param("id"), c.Param("recipient_id"))
	if errors.Is(err, service.ErrAlertRecipientNotFound) {
		utils.NotFound(c, "Recipient not found")
		return
	}
	if err != nil {
		s.logger.Error("Failed to resend confirmation", "error", err)
		utils.ErrorResponse(c, http.StatusBadGateway, "Confirmation email could not be sent", nil)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Confirmation email sent", gin.H{})
}

func writeAlertEmailPage(c *gin.Context, status int, title, message, button string) {
	data := publicpage.AlertEmailData{Title: title + " - Orion", Heading: title, Body: template.HTML(html.EscapeString(message)), Button: button}
	if button != "" {
		data.Action = c.Request.URL.Path
	}
	payload, err := publicpage.Render("alert-email.html", data)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to render page")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Data(status, "text/html; charset=utf-8", payload)
}

// alertEmailPage renders the confirm/unsubscribe page. GET never changes state, so
// mail link scanners that prefetch URLs cannot confirm or unsubscribe anyone.
func (s *Server) alertEmailPage(purpose string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := service.VerifyAlertEmailToken(s.cfg, purpose, c.Param("token"), time.Now().UTC()); err != nil {
			writeAlertEmailPage(c, http.StatusBadRequest, "Link expired", "Ask the Orion administrator to resend the confirmation email.", "")
			return
		}
		if purpose == service.AlertEmailPurposeConfirm {
			writeAlertEmailPage(c, http.StatusOK, "Confirm alert emails", "Press the button to start receiving Orion alert emails at this address.", "Confirm this address")
			return
		}
		writeAlertEmailPage(c, http.StatusOK, "Unsubscribe", "Press the button to stop receiving Orion alert emails at this address.", "Unsubscribe")
	}
}

func (s *Server) alertEmailAction(purpose string) gin.HandlerFunc {
	return func(c *gin.Context) {
		alerts := service.NewAlertService(s.db, s.logger, s.cfg)
		var err error
		if purpose == service.AlertEmailPurposeConfirm {
			err = alerts.ConfirmRecipient(c.Param("token"))
		} else {
			err = alerts.UnsubscribeRecipient(c.Param("token"))
		}
		if err != nil {
			writeAlertEmailPage(c, http.StatusBadRequest, "Link expired", "Ask the Orion administrator to resend the confirmation email.", "")
			return
		}
		if purpose == service.AlertEmailPurposeConfirm {
			writeAlertEmailPage(c, http.StatusOK, "Address confirmed", "You will now receive Orion alert emails. Every email has an unsubscribe link.", "")
			return
		}
		writeAlertEmailPage(c, http.StatusOK, "Unsubscribed", "You will no longer receive Orion alert emails.", "")
	}
}

// maskEmailAddress keeps the first letter and the domain: a***@example.com.
func maskEmailAddress(address string) string {
	return maskStatusPageSubscriberEmail(strings.ToLower(strings.TrimSpace(address)))
}
